package modpack

import (
	"context"
	"errors"
	"testing"
)

type fakeCatalog struct {
	mods  map[string]ModInfo
	errs  map[string]error
	order []string
}

func (catalog *fakeCatalog) Get(_ context.Context, modID string) (ModInfo, error) {
	catalog.order = append(catalog.order, modID)
	if err := catalog.errs[modID]; err != nil {
		return ModInfo{}, err
	}
	return catalog.mods[modID], nil
}

func stableVersion(id, version string, gameVersions []string) ModVersion {
	return ModVersion{ID: id, Version: version, ReleaseType: "stable", GameVersions: gameVersions}
}

func buildWith(version string, mods ...ModInstall) Build {
	return Build{GameVersion: version, Mods: mods}
}

func managedMod(modID, version string) ModInstall {
	return ModInstall{ModID: modID, Name: modID, Version: version, Managed: true}
}

func TestAnalyzeUpToDate(t *testing.T) {
	catalog := &fakeCatalog{mods: map[string]ModInfo{
		"stonequarry": {
			ID:            "stonequarry",
			LatestVersion: "1.2.0",
			Versions:      []ModVersion{stableVersion("v1", "1.2.0", nil)},
		},
	}}
	report, err := Analyze(context.Background(), buildWith("1.19.8", managedMod("stonequarry", "1.2.0")), catalog)
	if err != nil {
		t.Fatalf("Analyze returned an error: %v", err)
	}
	mod := report.Mods[0]
	if mod.Status != StatusUpToDate {
		t.Fatalf("expected up_to_date, got %s", mod.Status)
	}
	if mod.TargetVersion != "" || mod.TargetVersionID != "" {
		t.Fatalf("up-to-date mod must not expose a target version")
	}
	if report.Summary.UpToDate != 1 || report.Summary.UpdatesAvailable != 0 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
}

func TestAnalyzeUpdateAvailable(t *testing.T) {
	catalog := &fakeCatalog{mods: map[string]ModInfo{
		"stonequarry": {
			ID:            "stonequarry",
			LatestVersion: "1.3.0",
			Versions: []ModVersion{
				stableVersion("v1", "1.2.0", []string{"1.19"}),
				stableVersion("v2", "1.3.0", []string{"1.19", "1.20"}),
			},
		},
	}}
	report, err := Analyze(context.Background(), buildWith("1.19.8", managedMod("stonequarry", "1.2.0")), catalog)
	if err != nil {
		t.Fatalf("Analyze returned an error: %v", err)
	}
	mod := report.Mods[0]
	if mod.Status != StatusUpdateAvailable {
		t.Fatalf("expected update_available, got %s", mod.Status)
	}
	if mod.TargetVersion != "1.3.0" || mod.TargetVersionID != "v2" {
		t.Fatalf("unexpected target %s (%s)", mod.TargetVersion, mod.TargetVersionID)
	}
	if !mod.Compatible || mod.Prerelease {
		t.Fatalf("expected compatible stable update, got compatible=%v prerelease=%v", mod.Compatible, mod.Prerelease)
	}
	if report.Summary.UpdatesAvailable != 1 || report.Summary.Incompatible != 0 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
}

func TestAnalyzePrereleaseOnly(t *testing.T) {
	catalog := &fakeCatalog{mods: map[string]ModInfo{
		"stonequarry": {
			ID:            "stonequarry",
			LatestVersion: "2.0.0-beta.1",
			Versions: []ModVersion{
				stableVersion("v1", "1.2.0", nil),
				{ID: "v2", Version: "2.0.0-beta.1", ReleaseType: "prerelease"},
			},
		},
	}}
	report, err := Analyze(context.Background(), buildWith("1.19.8", managedMod("stonequarry", "1.2.0")), catalog)
	if err != nil {
		t.Fatalf("Analyze returned an error: %v", err)
	}
	mod := report.Mods[0]
	if mod.Status != StatusUpdateAvailable {
		t.Fatalf("expected update_available, got %s", mod.Status)
	}
	if mod.TargetVersion != "2.0.0-beta.1" {
		t.Fatalf("unexpected target %s", mod.TargetVersion)
	}
	if !mod.Prerelease {
		t.Fatalf("expected the target to be marked as prerelease")
	}
}

func TestAnalyzeIncompatibleUpdate(t *testing.T) {
	catalog := &fakeCatalog{mods: map[string]ModInfo{
		"stonequarry": {
			ID:            "stonequarry",
			LatestVersion: "1.3.0",
			Versions: []ModVersion{
				stableVersion("v1", "1.2.0", []string{"1.19"}),
				stableVersion("v2", "1.3.0", []string{"1.20"}),
			},
		},
	}}
	report, err := Analyze(context.Background(), buildWith("1.19.8", managedMod("stonequarry", "1.2.0")), catalog)
	if err != nil {
		t.Fatalf("Analyze returned an error: %v", err)
	}
	mod := report.Mods[0]
	if mod.Compatible {
		t.Fatalf("expected the update to be marked incompatible")
	}
	if report.Summary.Incompatible != 1 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
}

func TestAnalyzeLocalMod(t *testing.T) {
	report, err := Analyze(context.Background(), buildWith("1.19.8", ModInstall{
		Name: "my-local-mod", Version: "0.1.0", Managed: false,
	}), &fakeCatalog{})
	if err != nil {
		t.Fatalf("Analyze returned an error: %v", err)
	}
	mod := report.Mods[0]
	if mod.Status != StatusNotUpdatable || mod.Reason != ReasonLocalMod {
		t.Fatalf("expected not_updatable/local_mod, got %s/%s", mod.Status, mod.Reason)
	}
	if report.Summary.NotUpdatableLocal != 1 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
}

func TestAnalyzeManagedModWithoutIDIsLocal(t *testing.T) {
	report, err := Analyze(context.Background(), buildWith("1.19.8", ModInstall{
		Name: "managed-no-id", Version: "1.0.0", Managed: true,
	}), &fakeCatalog{})
	if err != nil {
		t.Fatalf("Analyze returned an error: %v", err)
	}
	if report.Mods[0].Status != StatusNotUpdatable || report.Mods[0].Reason != ReasonLocalMod {
		t.Fatalf("expected not_updatable/local_mod, got %s/%s", report.Mods[0].Status, report.Mods[0].Reason)
	}
}

func TestAnalyzeNotInCatalog(t *testing.T) {
	report, err := Analyze(context.Background(), buildWith("1.19.8", managedMod("ghost", "1.0.0")), &fakeCatalog{mods: map[string]ModInfo{}})
	if err != nil {
		t.Fatalf("Analyze returned an error: %v", err)
	}
	mod := report.Mods[0]
	if mod.Status != StatusNotUpdatable || mod.Reason != ReasonNotInCatalog {
		t.Fatalf("expected not_updatable/not_in_catalog, got %s/%s", mod.Status, mod.Reason)
	}
	if report.Summary.NotUpdatableAbsent != 1 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
}

func TestAnalyzeCatalogErrorKeepsAnalyzing(t *testing.T) {
	catalog := &fakeCatalog{
		errs: map[string]error{"broken": errors.New("boom")},
		mods: map[string]ModInfo{
			"working": {
				ID:            "working",
				LatestVersion: "1.0.0",
				Versions: []ModVersion{
					stableVersion("v0", "0.9.0", nil),
					stableVersion("v1", "1.0.0", nil),
				},
			},
		},
	}
	report, err := Analyze(context.Background(), buildWith("1.19.8",
		managedMod("broken", "1.0.0"),
		managedMod("working", "0.9.0"),
	), catalog)
	if err != nil {
		t.Fatalf("Analyze returned an error: %v", err)
	}
	if report.Mods[0].Status != StatusNotUpdatable || report.Mods[0].Reason != ReasonCatalogError {
		t.Fatalf("expected catalog_error for the broken mod, got %s/%s", report.Mods[0].Status, report.Mods[0].Reason)
	}
	if report.Mods[1].Status != StatusUpdateAvailable {
		t.Fatalf("expected the healthy mod to still be analyzed, got %s", report.Mods[1].Status)
	}
	if report.Summary.NotUpdatableCatalogError != 1 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
}

func TestAnalyzeAddedDependencies(t *testing.T) {
	catalog := &fakeCatalog{mods: map[string]ModInfo{
		"stonequarry": {
			ID:            "stonequarry",
			LatestVersion: "1.3.0",
			Versions: []ModVersion{
				stableVersion("v2", "1.3.0", nil),
			},
		},
	}}
	build := buildWith("1.19.8",
		managedMod("stonequarry", "1.2.0"),
		managedMod("alreadyinstalled", "1.0.0"),
	)
	// The target requires two deps: one already installed, one missing.
	catalog.mods["stonequarry"].Versions[0].Dependencies = []Dependency{
		{ModID: "alreadyinstalled", Name: "Already", Requirement: ">=1.0.0"},
		{ModID: "newdep", Name: "New", Requirement: ">=2.0.0"},
		{ModID: "game", Name: "Game"},
	}
	report, err := Analyze(context.Background(), build, catalog)
	if err != nil {
		t.Fatalf("Analyze returned an error: %v", err)
	}
	mod := report.Mods[0]
	if len(mod.AddedDeps) != 1 || mod.AddedDeps[0].ModID != "newdep" {
		t.Fatalf("expected only the missing dependency to be added, got %+v", mod.AddedDeps)
	}
}

func TestAnalyzeRemovedDependencies(t *testing.T) {
	catalog := &fakeCatalog{mods: map[string]ModInfo{
		"stonequarry": {
			ID:            "stonequarry",
			LatestVersion: "1.3.0",
			Versions:      []ModVersion{stableVersion("v2", "1.3.0", nil)},
		},
	}}
	build := buildWith("1.19.8",
		ModInstall{
			ModID: "stonequarry", Name: "stonequarry", Version: "1.2.0", Managed: true,
			Dependencies: []string{"olddep", "game"},
		},
		managedMod("olddep", "1.0.0"),
	)
	report, err := Analyze(context.Background(), build, catalog)
	if err != nil {
		t.Fatalf("Analyze returned an error: %v", err)
	}
	mod := report.Mods[0]
	if len(mod.RemovedDeps) != 1 || mod.RemovedDeps[0].ModID != "olddep" {
		t.Fatalf("expected the old dependency to be reported as removed, got %+v", mod.RemovedDeps)
	}
	if len(mod.AddedDeps) != 0 {
		t.Fatalf("unexpected added dependencies: %+v", mod.AddedDeps)
	}
}

func TestAnalyzeCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Analyze(ctx, buildWith("1.19.8", managedMod("stonequarry", "1.0.0")), &fakeCatalog{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestAnalyzeFallsBackToNewestStable(t *testing.T) {
	catalog := &fakeCatalog{mods: map[string]ModInfo{
		"stonequarry": {
			ID: "stonequarry",
			Versions: []ModVersion{
				stableVersion("v1", "1.2.0", nil),
				stableVersion("v2", "1.3.0", nil),
			},
		},
	}}
	report, err := Analyze(context.Background(), buildWith("1.19.8", managedMod("stonequarry", "1.2.0")), catalog)
	if err != nil {
		t.Fatalf("Analyze returned an error: %v", err)
	}
	if report.Mods[0].Status != StatusUpdateAvailable || report.Mods[0].TargetVersion != "1.3.0" {
		t.Fatalf("expected fallback to newest stable, got %+v", report.Mods[0])
	}
}
