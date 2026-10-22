package mods_test

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/downloads"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
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

type countingModArchiveDownloader struct {
	modArchiveDownloader
	count int
}

func (downloader *countingModArchiveDownloader) Download(ctx context.Context, request downloads.Request, progress chan<- downloads.Progress) error {
	downloader.count++
	return downloader.modArchiveDownloader.Download(ctx, request, progress)
}

func updateTestFixture(t *testing.T, installedVersion string, policy mods.UpdatePolicy) (testFixture, mods.InstanceRef, *countingModArchiveDownloader, string) {
	t.Helper()
	versions := []mods.ModVersion{
		{ID: "old", Version: "1.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable", FileName: "example-1.zip", DownloadURL: "https://example.test/old"},
		{ID: "equal", Version: "v2.0.0+catalog", GameVersions: []string{"1.20"}, ReleaseType: "stable", FileName: "example-2-equal.zip", DownloadURL: "https://example.test/equal"},
		{ID: "minor", Version: "2.1.0", GameVersions: []string{"1.20"}, ReleaseType: "stable", FileName: "example-2.1.zip", DownloadURL: "https://example.test/minor"},
		{ID: "major", Version: "3.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable", FileName: "example-3.zip", DownloadURL: "https://example.test/major"},
	}
	manifests := make(map[string]map[string]any, len(versions))
	for _, version := range versions {
		manifests[version.DownloadURL] = map[string]any{
			"modid": "example", "name": "Example", "version": version.Version, "dependencies": map[string]string{},
		}
	}
	downloader := &countingModArchiveDownloader{modArchiveDownloader: modArchiveDownloader{manifests: manifests}}
	fixture := newTestFixtureWithDeps(t, staticModCatalog{detailsByID: map[string]mods.ModDetails{
		"example": {ModSummary: mods.ModSummary{ID: "example", Name: "Example", LatestVersion: "3.0.0"}, Versions: versions},
	}}, downloader)
	instance := fixture.createTestInstance(t, "VersionChange")
	path := filepath.Join(instance.Directory, "Mods", "example.zip")
	writeModZip(t, path, map[string]any{"modid": "example", "name": "Example", "version": installedVersion})
	if err := fixture.repository.SaveMod(context.Background(), mods.InstalledMod{
		ID: "installed-example", InstanceID: instance.ID, Name: "Example", Version: installedVersion,
		FileName: "example.zip", FilePath: path, Enabled: true, Managed: true,
		Source: "moddb:example:installed", UpdatePolicy: policy, InstalledAt: mustTime(t), UpdatedAt: mustTime(t),
	}); err != nil {
		t.Fatal(err)
	}
	return fixture, instance, downloader, path
}

func TestUpdateInstanceModsRejectsNonIncreasingTargets(t *testing.T) {
	for _, versionID := range []string{"old", "equal"} {
		t.Run(versionID, func(t *testing.T) {
			fixture, instance, downloader, path := updateTestFixture(t, "2.0", mods.UpdatePolicyAutomatic)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			result, err := fixture.catalogService.UpdateInstanceMods(context.Background(), instance.ID, []mods.ModUpdateTarget{{
				ModID: " EXAMPLE ", VersionID: versionID,
			}}, true)
			if err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if result.Updated != 0 || fixture.snapshots.count() != 0 || downloader.count != 0 || string(after) != string(before) {
				t.Fatalf("result=%+v snapshots=%d downloads=%d changed=%v", result, fixture.snapshots.count(), downloader.count, string(after) != string(before))
			}
		})
	}
}

func TestUpdateInstanceModsSkipsUninstalledTarget(t *testing.T) {
	fixture, instance, downloader, _ := updateTestFixture(t, "2.0.0", mods.UpdatePolicyAutomatic)
	result, err := fixture.catalogService.UpdateInstanceMods(context.Background(), instance.ID, []mods.ModUpdateTarget{{ModID: "missing", VersionID: "unknown"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated != 0 || fixture.snapshots.count() != 0 || downloader.count != 0 {
		t.Fatalf("result=%+v snapshots=%d downloads=%d", result, fixture.snapshots.count(), downloader.count)
	}
}

func TestUpdateInstanceModsRejectsUnknownRelease(t *testing.T) {
	fixture, instance, downloader, _ := updateTestFixture(t, "2.0.0", mods.UpdatePolicyAutomatic)
	_, err := fixture.catalogService.UpdateInstanceMods(context.Background(), instance.ID, []mods.ModUpdateTarget{{ModID: "example", VersionID: "missing"}}, false)
	var appError *errs.AppError
	if !errors.As(err, &appError) || appError.Code != mods.ErrModVersionNotFound {
		t.Fatalf("error = %v", err)
	}
	if fixture.snapshots.count() != 0 || downloader.count != 0 {
		t.Fatalf("snapshots=%d downloads=%d", fixture.snapshots.count(), downloader.count)
	}
}

func TestUpdateInstanceModsAllowsMinorAndMajorUpgrades(t *testing.T) {
	for _, versionID := range []string{"minor", "major"} {
		t.Run(versionID, func(t *testing.T) {
			fixture, instance, downloader, _ := updateTestFixture(t, "2.0.0", mods.UpdatePolicyAutomatic)
			result, err := fixture.catalogService.UpdateInstanceMods(context.Background(), instance.ID, []mods.ModUpdateTarget{{ModID: "example", VersionID: versionID}}, false)
			if err != nil {
				t.Fatal(err)
			}
			if result.Updated != 1 || fixture.snapshots.count() != 1 || downloader.count != 1 {
				t.Fatalf("result=%+v snapshots=%d downloads=%d", result, fixture.snapshots.count(), downloader.count)
			}
		})
	}
}

func TestUpdateInstanceModsRevalidatesRepeatedTargets(t *testing.T) {
	fixture, instance, downloader, _ := updateTestFixture(t, "2.0.0", mods.UpdatePolicyAutomatic)
	result, err := fixture.catalogService.UpdateInstanceMods(context.Background(), instance.ID, []mods.ModUpdateTarget{
		{ModID: "example", VersionID: "old"},
		{ModID: "example", VersionID: "major"},
		{ModID: "example", VersionID: "minor"},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := fixture.modsService.ListMods(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	current := installedModByName(installed, "Example")
	if result.Updated != 1 || current.Version != "3.0.0" || fixture.snapshots.count() != 1 || downloader.count != 1 {
		t.Fatalf("result=%+v current=%+v snapshots=%d downloads=%d", result, current, fixture.snapshots.count(), downloader.count)
	}
}

func TestChangeInstanceModVersionAllowsExplicitReplacement(t *testing.T) {
	for _, test := range []struct {
		name, versionID, wantVersion string
	}{
		{name: "downgrade", versionID: "old", wantVersion: "1.0.0"},
		{name: "semantic equal", versionID: "equal", wantVersion: "v2.0.0+catalog"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture, instance, downloader, _ := updateTestFixture(t, "2.0.0", mods.UpdatePolicyAutomatic)
			result, err := fixture.catalogService.ChangeInstanceModVersion(context.Background(), instance.ID, mods.ModUpdateTarget{ModID: "example", VersionID: test.versionID}, false)
			if err != nil {
				t.Fatal(err)
			}
			installed, err := fixture.modsService.ListMods(context.Background(), instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			if result.Updated != 1 || installedModByName(installed, "Example").Version != test.wantVersion || fixture.snapshots.count() != 1 || downloader.count != 1 {
				t.Fatalf("result=%+v installed=%+v snapshots=%d downloads=%d", result, installed, fixture.snapshots.count(), downloader.count)
			}
		})
	}
}

func TestChangeInstanceModVersionSameSourceIsNoOp(t *testing.T) {
	fixture, instance, downloader, _ := updateTestFixture(t, "2.0.0", mods.UpdatePolicyAutomatic)
	result, err := fixture.catalogService.ChangeInstanceModVersion(context.Background(), instance.ID, mods.ModUpdateTarget{ModID: "example", VersionID: "installed"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated != 0 || fixture.snapshots.count() != 0 || downloader.count != 0 {
		t.Fatalf("result=%+v snapshots=%d downloads=%d", result, fixture.snapshots.count(), downloader.count)
	}
}

func TestPinnedModVersionChangesAreNoOps(t *testing.T) {
	fixture, instance, downloader, _ := updateTestFixture(t, "2.0.0", mods.UpdatePolicyPinned)
	automatic, err := fixture.catalogService.UpdateInstanceMods(context.Background(), instance.ID, []mods.ModUpdateTarget{{ModID: "example", VersionID: "major"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	manual, err := fixture.catalogService.ChangeInstanceModVersion(context.Background(), instance.ID, mods.ModUpdateTarget{ModID: "example", VersionID: "old"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if automatic.Updated != 0 || automatic.SkippedByPolicy != 1 || manual.Updated != 0 || manual.SkippedByPolicy != 1 || fixture.snapshots.count() != 0 || downloader.count != 0 {
		t.Fatalf("automatic=%+v manual=%+v snapshots=%d downloads=%d", automatic, manual, fixture.snapshots.count(), downloader.count)
	}
}

func TestUpdateInstanceModsRevalidatesAfterDependencyInstall(t *testing.T) {
	versionsA := []mods.ModVersion{{ID: "a2", Version: "2.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable", FileName: "a2.zip", DownloadURL: "https://example.test/a2"}}
	versionsB := []mods.ModVersion{
		{ID: "b2", Version: "2.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable", FileName: "b2.zip", DownloadURL: "https://example.test/b2"},
		{ID: "b3", Version: "3.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable", FileName: "b3.zip", DownloadURL: "https://example.test/b3"},
	}
	downloader := &countingModArchiveDownloader{modArchiveDownloader: modArchiveDownloader{manifests: map[string]map[string]any{
		"https://example.test/a2": {"modid": "a", "name": "A", "version": "2.0.0", "dependencies": map[string]string{"b": ">=3.0.0"}},
		"https://example.test/b2": {"modid": "b", "name": "B", "version": "2.0.0", "dependencies": map[string]string{}},
		"https://example.test/b3": {"modid": "b", "name": "B", "version": "3.0.0", "dependencies": map[string]string{}},
	}}}
	fixture := newTestFixtureWithDeps(t, staticModCatalog{detailsByID: map[string]mods.ModDetails{
		"a": {ModSummary: mods.ModSummary{ID: "a", Name: "A"}, Versions: versionsA},
		"b": {ModSummary: mods.ModSummary{ID: "b", Name: "B"}, Versions: versionsB},
	}}, downloader)
	instance := fixture.createTestInstance(t, "DependencyRevalidation")
	for _, modID := range []string{"a", "b"} {
		path := filepath.Join(instance.Directory, "Mods", modID+".zip")
		writeModZip(t, path, map[string]any{"modid": modID, "name": strings.ToUpper(modID), "version": "1.0.0"})
		if err := fixture.repository.SaveMod(context.Background(), mods.InstalledMod{
			ID: modID, InstanceID: instance.ID, Name: strings.ToUpper(modID), Version: "1.0.0", FileName: modID + ".zip", FilePath: path,
			Enabled: true, Managed: true, Source: "moddb:" + modID + ":old", InstalledAt: mustTime(t), UpdatedAt: mustTime(t),
		}); err != nil {
			t.Fatal(err)
		}
	}
	result, err := fixture.catalogService.UpdateInstanceMods(context.Background(), instance.ID, []mods.ModUpdateTarget{
		{ModID: "a", VersionID: "a2"}, {ModID: "b", VersionID: "b2"},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := fixture.modsService.ListMods(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated != 1 || installedModByName(installed, "B").Version != "3.0.0" || fixture.snapshots.count() != 1 || downloader.count != 2 {
		t.Fatalf("result=%+v installed=%+v snapshots=%d downloads=%d", result, installed, fixture.snapshots.count(), downloader.count)
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

func TestUpdateFlowRespectsReleaseChannel(t *testing.T) {
	versions := []mods.ModVersion{
		{ID: "stable19", Version: "1.9.8", GameVersions: []string{"1.20"}, ReleaseType: "stable", FileName: "example-1.9.8.zip", DownloadURL: "https://example.test/1.9.8"},
		{ID: "pre5", Version: "2.0.0-pre.5", GameVersions: []string{"1.20"}, ReleaseType: "beta", FileName: "example-pre5.zip", DownloadURL: "https://example.test/pre5"},
		{ID: "pre6", Version: "2.0.0-pre.6", GameVersions: []string{"1.20"}, ReleaseType: "beta", FileName: "example-pre6.zip", DownloadURL: "https://example.test/pre6"},
	}
	manifests := make(map[string]map[string]any, len(versions))
	for _, version := range versions {
		manifests[version.DownloadURL] = map[string]any{
			"modid": "example", "name": "Example", "version": version.Version, "dependencies": map[string]string{},
		}
	}
	newFixture := func(t *testing.T) (testFixture, mods.InstanceRef, *countingModArchiveDownloader, string) {
		t.Helper()
		downloader := &countingModArchiveDownloader{modArchiveDownloader: modArchiveDownloader{manifests: manifests}}
		fixture := newTestFixtureWithDeps(t, staticModCatalog{detailsByID: map[string]mods.ModDetails{
			"example": {ModSummary: mods.ModSummary{ID: "example", Name: "Example", LatestVersion: "1.9.8"}, Versions: versions},
		}}, downloader)
		instance := fixture.createTestInstance(t, "ReleaseChannel")
		path := filepath.Join(instance.Directory, "Mods", "example.zip")
		writeModZip(t, path, map[string]any{"modid": "example", "name": "Example", "version": "2.0.0-pre.5"})
		if err := fixture.repository.SaveMod(context.Background(), mods.InstalledMod{
			ID: "installed-example", InstanceID: instance.ID, Name: "Example", Version: "2.0.0-pre.5",
			FileName: "example.zip", FilePath: path, Enabled: true, Managed: true,
			Source: "moddb:example:pre5", UpdatePolicy: mods.UpdatePolicyAutomatic, InstalledAt: mustTime(t), UpdatedAt: mustTime(t),
		}); err != nil {
			t.Fatal(err)
		}
		return fixture, instance, downloader, path
	}

	t.Run("automatic update rejects older stable", func(t *testing.T) {
		fixture, instance, downloader, path := newFixture(t)
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		result, err := fixture.catalogService.UpdateInstanceMods(context.Background(), instance.ID, []mods.ModUpdateTarget{{ModID: "example", VersionID: "stable19"}}, false)
		if err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if result.Updated != 0 || fixture.snapshots.count() != 0 || downloader.count != 0 || string(after) != string(before) {
			t.Fatalf("result=%+v snapshots=%d downloads=%d", result, fixture.snapshots.count(), downloader.count)
		}
	})

	t.Run("automatic update accepts newer prerelease", func(t *testing.T) {
		fixture, instance, downloader, _ := newFixture(t)
		result, err := fixture.catalogService.UpdateInstanceMods(context.Background(), instance.ID, []mods.ModUpdateTarget{{ModID: "example", VersionID: "pre6"}}, false)
		if err != nil {
			t.Fatal(err)
		}
		if result.Updated != 1 || fixture.snapshots.count() != 1 || downloader.count != 1 {
			t.Fatalf("result=%+v snapshots=%d downloads=%d", result, fixture.snapshots.count(), downloader.count)
		}
	})

	t.Run("manual replacement allows older stable", func(t *testing.T) {
		fixture, instance, downloader, _ := newFixture(t)
		result, err := fixture.catalogService.ChangeInstanceModVersion(context.Background(), instance.ID, mods.ModUpdateTarget{ModID: "example", VersionID: "stable19"}, false)
		if err != nil {
			t.Fatal(err)
		}
		installed, err := fixture.modsService.ListMods(context.Background(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if result.Updated != 1 || installedModByName(installed, "Example").Version != "1.9.8" || fixture.snapshots.count() != 1 || downloader.count != 1 {
			t.Fatalf("result=%+v installed=%+v snapshots=%d downloads=%d", result, installed, fixture.snapshots.count(), downloader.count)
		}
	})
}
