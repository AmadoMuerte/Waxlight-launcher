package mods_test

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
	vsmodpack "github.com/AmadoMuerte/vintagestory-go/modpack"
)

func mustTime(t *testing.T) time.Time {
	t.Helper()
	return time.Now().UTC()
}

func writeModZip(t *testing.T, path string, manifest map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create("modinfo.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(entry).Encode(manifest); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCheckInstanceModUpdates(t *testing.T) {
	fixture := newTestFixtureWithDeps(t, staticModCatalog{detailsByID: map[string]mods.ModDetails{
		"stonequarry": {
			ModSummary: mods.ModSummary{
				ID:            "stonequarry",
				Name:          "Stone Quarry",
				LatestVersion: "1.3.0",
			},
			Versions: []mods.ModVersion{
				{ID: "v1", Version: "1.2.0", GameVersions: []string{"1.19", "1.20"}, ReleaseType: "stable"},
				{ID: "v2", Version: "1.3.0", GameVersions: []string{"1.19", "1.20"}, ReleaseType: "stable", Changelog: "Fixed a crash."},
			},
		},
	}}, recordingDownloader{})
	ctx := context.Background()
	instance := fixture.createTestInstance(t, "Updates")

	stoneQuarryPath := filepath.Join(instance.Directory, "Mods", "stonequarry.zip")
	writeModZip(t, stoneQuarryPath, map[string]any{
		"modid":        "stonequarry",
		"name":         "Stone Quarry",
		"version":      "1.2.0",
		"dependencies": map[string]string{"olddep": ">=1.0.0"},
	})
	if err := fixture.repository.SaveMod(ctx, mods.InstalledMod{
		ID:          "mod-stonequarry",
		InstanceID:  instance.ID,
		Name:        "Stone Quarry",
		Version:     "1.2.0",
		FileName:    "stonequarry.zip",
		FilePath:    stoneQuarryPath,
		Enabled:     true,
		Managed:     true,
		Source:      "moddb:stonequarry:123",
		InstalledAt: mustTime(t),
		UpdatedAt:   mustTime(t),
	}); err != nil {
		t.Fatalf("save mod: %v", err)
	}

	oldDepPath := filepath.Join(instance.Directory, "Mods", "olddep.zip")
	writeModZip(t, oldDepPath, map[string]any{
		"modid":        "olddep",
		"name":         "Old Dep",
		"version":      "1.0.0",
		"dependencies": map[string]string{},
	})
	if err := fixture.repository.SaveMod(ctx, mods.InstalledMod{
		ID:          "mod-olddep",
		InstanceID:  instance.ID,
		Name:        "Old Dep",
		Version:     "1.0.0",
		FileName:    "olddep.zip",
		FilePath:    oldDepPath,
		Enabled:     true,
		Managed:     true,
		Source:      "moddb:olddep:456",
		InstalledAt: mustTime(t),
		UpdatedAt:   mustTime(t),
	}); err != nil {
		t.Fatalf("save mod: %v", err)
	}

	localPath := filepath.Join(instance.Directory, "Mods", "mylocal.cs")
	if err := os.WriteFile(localPath, []byte("// local script mod"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := fixture.catalogService.CheckInstanceModUpdates(ctx, instance.ID)
	if err != nil {
		t.Fatalf("CheckInstanceModUpdates returned an error: %v", err)
	}

	if report.Summary.UpdatesAvailable != 1 {
		t.Fatalf("expected one update, got summary %+v", report.Summary)
	}
	updatable := findReportMod(report.Mods, "stonequarry")
	if updatable == nil {
		t.Fatalf("stonequarry mod missing from the report: %+v", report.Mods)
	}
	if updatable.Status != "update_available" {
		t.Fatalf("unexpected status %s", updatable.Status)
	}
	if updatable.TargetVersion != "1.3.0" || updatable.TargetVersionID != "v2" {
		t.Fatalf("unexpected target %s (%s)", updatable.TargetVersion, updatable.TargetVersionID)
	}
	if !updatable.Compatible {
		t.Fatalf("expected the update to be compatible with game version 1.20")
	}
	if updatable.Changelog != "Fixed a crash." {
		t.Fatalf("unexpected changelog %q", updatable.Changelog)
	}
	if len(updatable.RemovedDeps) != 1 || updatable.RemovedDeps[0].ModID != "olddep" {
		t.Fatalf("expected olddep to be reported as removed, got %+v", updatable.RemovedDeps)
	}

	if report.Summary.NotUpdatableAbsent != 1 {
		t.Fatalf("expected olddep to be absent from the catalog, got summary %+v", report.Summary)
	}
	missing := findReportMod(report.Mods, "olddep")
	if missing == nil || missing.Status != "not_updatable" || missing.Reason != "not_in_catalog" {
		t.Fatalf("expected olddep to be not_in_catalog, got %+v", missing)
	}

	local := findReportMod(report.Mods, "mylocal")
	if local == nil || local.Status != "not_updatable" || local.Reason != "local_mod" {
		t.Fatalf("expected the local script mod to be not_updatable/local_mod, got %+v", local)
	}
}

func TestCheckInstanceModUpdatesUnknownInstance(t *testing.T) {
	fixture := newTestFixture(t)
	_, err := fixture.catalogService.CheckInstanceModUpdates(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected an error for an unknown instance")
	}
}

func TestCheckInstanceModUpdatesHidesIgnoredUpdate(t *testing.T) {
	fixture := newTestFixtureWithDeps(t, staticModCatalog{detailsByID: map[string]mods.ModDetails{
		"ignored": {
			ModSummary: mods.ModSummary{ID: "ignored", Name: "Ignored", LatestVersion: "2.0.0"},
			Versions: []mods.ModVersion{
				{ID: "old", Version: "1.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable"},
				{ID: "new", Version: "2.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable"},
			},
		},
	}}, recordingDownloader{})
	instance := fixture.createTestInstance(t, "Ignored")
	path := filepath.Join(instance.Directory, "Mods", "ignored.zip")
	writeModZip(t, path, map[string]any{"modid": "ignored", "name": "Ignored", "version": "1.0.0"})
	if err := fixture.repository.SaveMod(context.Background(), mods.InstalledMod{
		ID: "ignored", InstanceID: instance.ID, Name: "Ignored", Version: "1.0.0", FileName: "ignored.zip",
		FilePath: path, Enabled: true, Managed: true, Source: "moddb:ignored:old",
		UpdatePolicy: mods.UpdatePolicyPinned, InstalledAt: mustTime(t), UpdatedAt: mustTime(t),
	}); err != nil {
		t.Fatal(err)
	}
	report, err := fixture.catalogService.CheckInstanceModUpdates(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.UpdatesAvailable != 0 || findReportMod(report.Mods, "ignored").Status != "up_to_date" {
		t.Fatalf("ignored update was reported: %+v", report)
	}
}

func TestCheckInstanceModUpdatesCompatibleOnlyChoosesCompatibleRelease(t *testing.T) {
	fixture := newTestFixtureWithDeps(t, staticModCatalog{detailsByID: map[string]mods.ModDetails{
		"compat": {
			ModSummary: mods.ModSummary{ID: "compat", Name: "Compatible", LatestVersion: "3.0.0"},
			Versions: []mods.ModVersion{
				{ID: "old", Version: "1.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable", DownloadURL: "https://example.test/old"},
				{ID: "compatible", Version: "2.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable", DownloadURL: "https://example.test/compatible"},
				{ID: "latest", Version: "3.0.0", GameVersions: []string{"1.21"}, ReleaseType: "stable", DownloadURL: "https://example.test/latest"},
			},
		},
	}}, recordingDownloader{})
	instance := fixture.createTestInstance(t, "Compatible")
	path := filepath.Join(instance.Directory, "Mods", "compat.zip")
	writeModZip(t, path, map[string]any{"modid": "compat", "name": "Compatible", "version": "1.0.0"})
	if err := fixture.repository.SaveMod(context.Background(), mods.InstalledMod{
		ID: "compat", InstanceID: instance.ID, Name: "Compatible", Version: "1.0.0", FileName: "compat.zip",
		FilePath: path, Enabled: true, Managed: true, Source: "moddb:compat:old",
		UpdatePolicy: mods.UpdatePolicyCompatibleOnly, InstalledAt: mustTime(t), UpdatedAt: mustTime(t),
	}); err != nil {
		t.Fatal(err)
	}
	report, err := fixture.catalogService.CheckInstanceModUpdates(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	update := findReportMod(report.Mods, "compat")
	if update == nil || update.TargetVersionID != "compatible" || !update.Compatible {
		t.Fatalf("compatible release not selected: %+v", update)
	}
}

func findReportMod(mods []vsmodpack.ModUpdate, name string) *vsmodpack.ModUpdate {
	for index := range mods {
		if mods[index].Name == name || mods[index].ModID == name {
			return &mods[index]
		}
	}
	return nil
}
