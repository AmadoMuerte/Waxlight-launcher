package mods_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
)

func TestGetCatalogModDoesNotWaitForFileSizes(t *testing.T) {
	details := mods.ModDetails{
		ModSummary: mods.ModSummary{ID: "51", Name: "Player Corpse"},
		Versions: []mods.ModVersion{{
			ID: "7", Version: "2.0.0", DownloadURL: "https://cdn.test/playercorpse.zip",
		}},
	}
	fixture := newTestFixtureWithDeps(t, staticModCatalog{details: details}, blockingContentLengthDownloader{})

	result, err := fixture.catalogService.GetCatalogMod(context.Background(), "51")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Versions) != 1 || result.Versions[0].FileSize != 0 {
		t.Fatalf("unexpected catalog details: %#v", result)
	}
}

type blockingContentLengthDownloader struct {
	recordingDownloader
}

func (blockingContentLengthDownloader) ContentLength(context.Context, string) (int64, error) {
	time.Sleep(time.Second)
	return 1, nil
}

func TestDownloadCatalogModInstallsIntoSeveralInstances(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	first := fixture.createTestInstance(t, "First")
	second := fixture.createTestInstance(t, "Second")
	details := mods.ModDetails{ModSummary: mods.ModSummary{ID: "51", Name: "Player Corpse", AuthorName: "Ada", LatestVersion: "2.0.0"}, Versions: []mods.ModVersion{{
		ID: "7", Version: "2.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable",
		FileName: "playercorpse.zip", DownloadURL: "https://cdn.test/playercorpse.zip",
	}}}
	fixture = newTestFixtureWithDeps(t, staticModCatalog{details: details}, recordingDownloader{})
	first = fixture.createTestInstance(t, "First")
	second = fixture.createTestInstance(t, "Second")
	result, err := fixture.catalogService.DownloadCatalogMod(ctx, mods.DownloadModRequest{
		ModID: "51", VersionID: "7", InstanceIDs: []string{first.ID, second.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Installations) != 2 || !result.Installations[0].Installed || !result.Installations[1].Installed {
		t.Fatalf("unexpected installation result: %#v", result.Installations)
	}
	for _, instance := range []mods.InstanceRef{first, second} {
		installed, listErr := fixture.repository.ListMods(ctx, instance.ID)
		if listErr != nil || len(installed) != 1 || installed[0].Source != "moddb:51:7" {
			t.Fatalf("unexpected installed mods: %#v, %v", installed, listErr)
		}
		if _, statErr := os.Stat(filepath.Join(instance.Directory, "Mods", "playercorpse.zip")); statErr != nil {
			t.Fatal(statErr)
		}
	}

	// Installing an already downloaded version reuses the cache and does not duplicate records.
	result, err = fixture.catalogService.InstallDownloadedMod(ctx, "51", "7", []string{first.ID}, false)
	if err != nil || !result.Installations[0].Installed {
		t.Fatalf("unexpected repeated install: %#v, %v", result, err)
	}
	installed, _ := fixture.repository.ListMods(ctx, first.ID)
	if len(installed) != 1 {
		t.Fatalf("repeated install created %d records", len(installed))
	}
}

func TestDownloadCatalogModsBatchContinuesAfterTargetFailure(t *testing.T) {
	first := mods.ModDetails{ModSummary: mods.ModSummary{ID: "first", Name: "First"}, Versions: []mods.ModVersion{{
		ID: "1", Version: "1.0.0", GameVersions: []string{"1.19"}, FileName: "first.zip", DownloadURL: "https://cdn.test/first.zip",
	}}}
	second := mods.ModDetails{ModSummary: mods.ModSummary{ID: "second", Name: "Second"}, Versions: []mods.ModVersion{{
		ID: "2", Version: "1.0.0", GameVersions: []string{"1.20"}, FileName: "second.zip", DownloadURL: "https://cdn.test/second.zip",
	}}}
	fixture := newTestFixtureWithDeps(t, staticModCatalog{detailsByID: map[string]mods.ModDetails{
		"first": first, "second": second,
	}}, recordingDownloader{})
	ctx := context.Background()
	instance := fixture.createTestInstance(t, "Batch")

	results := fixture.catalogService.DownloadCatalogModsBatch(ctx, mods.BatchDownloadModsRequest{
		InstanceID: instance.ID,
		Targets: []mods.DownloadModTarget{
			{ModID: "first", VersionID: "1"},
			{ModID: "missing", VersionID: "1"},
			{ModID: "second", VersionID: "2"},
		},
	})
	if len(results) != 3 || results[1].Error == "" {
		t.Fatalf("unexpected batch results: %#v", results)
	}
	if !results[0].Result.Installations[0].Installed || !results[2].Result.Installations[0].Installed {
		t.Fatalf("successful targets were not installed: %#v", results)
	}
	installed, err := fixture.repository.ListMods(ctx, instance.ID)
	if err != nil || len(installed) != 2 {
		t.Fatalf("unexpected installed mods: %#v, %v", installed, err)
	}
}

func TestRemoveUnusedDownloadedModsKeepsInstalledDependencies(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := fixture.createTestInstance(t, "Uses dependency")
	used := mods.DownloadedMod{SchemaVersion: 1, ModID: "library", VersionID: "2.0.10", Name: "Library", FileSize: 50}
	unused := mods.DownloadedMod{SchemaVersion: 1, ModID: "oldmod", VersionID: "1.0.0", Name: "Old Mod", FileSize: 25}
	if err := fixture.downloads.Save(ctx, used); err != nil {
		t.Fatal(err)
	}
	if err := fixture.downloads.Save(ctx, unused); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.SaveMod(ctx, mods.InstalledMod{
		ID: "installed-library", InstanceID: instance.ID, Name: "Library", Source: "moddb:library:2.0.10",
	}); err != nil {
		t.Fatal(err)
	}

	preview, err := fixture.catalogService.PreviewUnusedDownloadedMods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if preview.RemovedCount != 1 || preview.FreedBytes != 25 {
		t.Fatalf("unexpected cleanup preview: %#v", preview)
	}
	if _, err := fixture.catalogService.RemoveUnusedDownloadedMods(ctx); err != nil {
		t.Fatal(err)
	}
	items, err := fixture.downloads.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ModID != "library" || items[0].VersionID != "2.0.10" {
		t.Fatalf("installed dependency was not retained: %#v", items)
	}
}

func TestRemoveDownloadedModsIfUnusedLockedPreservesUsedCache(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := fixture.createTestInstance(t, "Uses dependency")
	used := mods.DownloadedMod{SchemaVersion: 1, ModID: "library", VersionID: "2.0.10", Name: "Library", FileSize: 50}
	introduced := mods.DownloadedMod{SchemaVersion: 1, ModID: "imported", VersionID: "1.0.0", Name: "Imported", FileSize: 25}
	if err := fixture.downloads.Save(ctx, used); err != nil {
		t.Fatal(err)
	}
	if err := fixture.downloads.Save(ctx, introduced); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.SaveMod(ctx, mods.InstalledMod{
		ID: "installed-library", InstanceID: instance.ID, Name: "Library", Source: "moddb:library:2.0.10",
	}); err != nil {
		t.Fatal(err)
	}

	if err := fixture.catalogService.RemoveDownloadedModsIfUnusedLocked(ctx, []mods.DownloadedMod{used, introduced}); err != nil {
		t.Fatal(err)
	}
	items, err := fixture.downloads.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ModID != used.ModID || items[0].VersionID != used.VersionID {
		t.Fatalf("unexpected cached downloads after cleanup: %#v", items)
	}
}

func TestDownloadCatalogModResolvesAndInstallsDependencies(t *testing.T) {
	rootMod := mods.ModDetails{
		ModSummary: mods.ModSummary{
			ID: "51", Slug: "rootmod", Name: "Root Mod", AuthorName: "Ada", LatestVersion: "2.0.0",
		},
		Versions: []mods.ModVersion{{
			ID: "7", Version: "2.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable",
			FileName: "rootmod.zip", DownloadURL: "https://cdn.test/rootmod.zip",
		}},
	}
	expandedMatter := mods.ModDetails{
		ModSummary: mods.ModSummary{
			ID: "60", Slug: "em", Name: "Expanded Matter", AuthorName: "Grace", LatestVersion: "3.4.0",
		},
		Versions: []mods.ModVersion{
			{
				ID: "12", Version: "3.2.0", GameVersions: []string{"1.20"}, ReleaseType: "stable",
				FileName: "em-3.2.0.zip", DownloadURL: "https://cdn.test/em-3.2.0.zip",
			},
			{
				ID: "13", Version: "3.3.3", GameVersions: []string{"1.20"}, ReleaseType: "stable",
				FileName: "em-3.3.3.zip", DownloadURL: "https://cdn.test/em-3.3.3.zip",
			},
			{
				ID: "14", Version: "3.4.0", GameVersions: []string{"1.20"}, ReleaseType: "stable",
				FileName: "em-3.4.0.zip", DownloadURL: "https://cdn.test/em-3.4.0.zip",
			},
		},
	}
	configLib := mods.ModDetails{
		ModSummary: mods.ModSummary{
			ID: "70", Slug: "configlib", Name: "Config Lib", AuthorName: "Linus", LatestVersion: "1.2.0",
		},
		Versions: []mods.ModVersion{{
			ID: "21", Version: "1.2.0", GameVersions: []string{"1.20"}, ReleaseType: "stable",
			FileName: "configlib.zip", DownloadURL: "https://cdn.test/configlib.zip",
		}},
	}

	fixture := newTestFixtureWithDeps(t, staticModCatalog{
		detailsByID: map[string]mods.ModDetails{
			"51": rootMod, "rootmod": rootMod,
			"60": expandedMatter, "em": expandedMatter,
			"70": configLib, "configlib": configLib,
		},
	}, modArchiveDownloader{
		manifests: map[string]map[string]any{
			"https://cdn.test/rootmod.zip": {
				"modid": "rootmod", "name": "Root Mod", "version": "2.0.0",
				"dependencies": map[string]string{"game": "1.20", "em": "3.3.3"},
			},
			"https://cdn.test/em-3.3.3.zip": {
				"modid": "em", "name": "Expanded Matter", "version": "3.3.3",
				"dependencies": map[string]string{"survival": "*", "configlib": "1.1.0"},
			},
			"https://cdn.test/configlib.zip": {
				"modid": "configlib", "name": "Config Lib", "version": "1.2.0",
				"dependencies": map[string]string{},
			},
		},
	})
	ctx := context.Background()
	instance := fixture.createTestInstance(t, "Dependency test")

	result, err := fixture.catalogService.DownloadCatalogMod(ctx, mods.DownloadModRequest{
		ModID: "51", VersionID: "7", InstanceIDs: []string{instance.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Installations) != 1 || !result.Installations[0].Installed {
		t.Fatalf("unexpected installation result: %#v", result.Installations)
	}
	if len(result.DownloadedNow) != 3 {
		t.Fatalf("expected root mod and dependencies to be tracked, got %#v", result.DownloadedNow)
	}

	installed, err := fixture.repository.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(installed) != 3 {
		t.Fatalf("expected root mod and two dependencies, got %#v", installed)
	}
	sources := make(map[string]bool, len(installed))
	for _, mod := range installed {
		sources[mod.Source] = true
	}
	for _, expectedSource := range []string{"moddb:51:7", "moddb:60:13", "moddb:70:21"} {
		if !sources[expectedSource] {
			t.Fatalf("missing installed source %q in %#v", expectedSource, sources)
		}
	}
	if sources["moddb:60:12"] || sources["moddb:60:14"] {
		t.Fatal("did not prefer the exact dependency version requested by modinfo.json")
	}

	for _, fileName := range []string{"rootmod.zip", "em-3.3.3.zip", "configlib.zip"} {
		if _, err := os.Stat(filepath.Join(instance.Directory, "Mods", fileName)); err != nil {
			t.Fatalf("expected %s to be installed: %v", fileName, err)
		}
	}

	payload, ok := fixture.events.last("mods:downloads-changed")
	if !ok {
		t.Fatal("expected mods:downloads-changed event")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var changed struct {
		ModID                  string `json:"modId"`
		DownloadedDependencies []struct {
			ModID   string `json:"modId"`
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"downloadedDependencies"`
	}
	if err := json.Unmarshal(data, &changed); err != nil {
		t.Fatal(err)
	}
	if changed.ModID != "51" || len(changed.DownloadedDependencies) != 2 {
		t.Fatalf("unexpected downloads-changed event: %s", data)
	}
	dependencyVersions := map[string]string{}
	for _, dependency := range changed.DownloadedDependencies {
		dependencyVersions[dependency.ModID] = dependency.Version
	}
	if dependencyVersions["60"] != "3.3.3" || dependencyVersions["70"] != "1.2.0" {
		t.Fatalf("unexpected downloaded dependencies: %#v", dependencyVersions)
	}

	fixture.events.clear()
	if _, err := fixture.catalogService.InstallDownloadedMod(ctx, "51", "7", []string{instance.ID}, false); err != nil {
		t.Fatal(err)
	}
	payload, ok = fixture.events.last("mods:downloads-changed")
	if !ok {
		t.Fatal("expected cache refresh event")
	}
	data, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &changed); err != nil {
		t.Fatal(err)
	}
	if len(changed.DownloadedDependencies) != 0 {
		t.Fatalf("cached dependencies must not be reported as newly downloaded: %s", data)
	}
}

func TestDownloadCatalogModUpgradesSharedDependencyVersion(t *testing.T) {
	root := mods.ModDetails{ModSummary: mods.ModSummary{ID: "root", Name: "Root"}, Versions: []mods.ModVersion{{
		ID: "root-1", Version: "1.0.0", GameVersions: []string{"1.20"}, FileName: "root.zip", DownloadURL: "https://cdn.test/root.zip",
	}}}
	first := mods.ModDetails{ModSummary: mods.ModSummary{ID: "first", Name: "First"}, Versions: []mods.ModVersion{{
		ID: "first-1", Version: "1.0.0", GameVersions: []string{"1.20"}, FileName: "first.zip", DownloadURL: "https://cdn.test/first.zip",
	}}}
	second := mods.ModDetails{ModSummary: mods.ModSummary{ID: "second", Name: "Second"}, Versions: []mods.ModVersion{{
		ID: "second-1", Version: "1.0.0", GameVersions: []string{"1.20"}, FileName: "second.zip", DownloadURL: "https://cdn.test/second.zip",
	}}}
	library := mods.ModDetails{ModSummary: mods.ModSummary{ID: "library", Name: "Library"}, Versions: []mods.ModVersion{
		{ID: "library-9", Version: "2.0.9", GameVersions: []string{"1.20"}, FileName: "library-9.zip", DownloadURL: "https://cdn.test/library-9.zip"},
		{ID: "library-10", Version: "2.0.10", GameVersions: []string{"1.20"}, FileName: "library-10.zip", DownloadURL: "https://cdn.test/library-10.zip"},
	}}
	fixture := newTestFixtureWithDeps(t, staticModCatalog{detailsByID: map[string]mods.ModDetails{
		"root": root, "first": first, "second": second, "library": library,
	}}, modArchiveDownloader{manifests: map[string]map[string]any{
		"https://cdn.test/root.zip":       {"modid": "root", "version": "1.0.0", "dependencies": map[string]string{"first": "1.0.0", "second": "1.0.0"}},
		"https://cdn.test/first.zip":      {"modid": "first", "version": "1.0.0", "dependencies": map[string]string{"library": "2.0.9"}},
		"https://cdn.test/second.zip":     {"modid": "second", "version": "1.0.0", "dependencies": map[string]string{"library": "2.0.10"}},
		"https://cdn.test/library-9.zip":  {"modid": "library", "version": "2.0.9", "dependencies": map[string]string{}},
		"https://cdn.test/library-10.zip": {"modid": "library", "version": "2.0.10", "dependencies": map[string]string{}},
	}})
	ctx := context.Background()
	instance := fixture.createTestInstance(t, "Shared dependency")

	if _, err := fixture.catalogService.DownloadCatalogMod(ctx, mods.DownloadModRequest{
		ModID: "root", VersionID: "root-1", InstanceIDs: []string{instance.ID},
	}); err != nil {
		t.Fatal(err)
	}
	installed, err := fixture.repository.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	sources := make(map[string]bool, len(installed))
	for _, mod := range installed {
		sources[mod.Source] = true
	}
	if !sources["moddb:library:library-10"] || sources["moddb:library:library-9"] {
		t.Fatalf("expected only upgraded library version to be installed: %#v", sources)
	}
}

func twoVersionCorpseCatalog() staticModCatalog {
	details := mods.ModDetails{
		ModSummary: mods.ModSummary{
			ID: "51", Name: "Player Corpse", AuthorName: "Ada", LatestVersion: "2.1.0",
		},
		Versions: []mods.ModVersion{
			{
				ID: "7", Version: "2.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable",
				FileName: "v7.zip", DownloadURL: "https://cdn.test/v7.zip",
			},
			{
				ID: "9", Version: "2.1.0", GameVersions: []string{"1.20"}, ReleaseType: "stable",
				FileName: "v9.zip", DownloadURL: "https://cdn.test/v9.zip",
			},
		},
	}
	return staticModCatalog{details: details}
}

func TestUpdateModRemovesSupersededCacheVersion(t *testing.T) {
	fixture := newTestFixtureWithDeps(t, twoVersionCorpseCatalog(), modArchiveDownloader{})
	ctx := context.Background()
	instance := fixture.createTestInstance(t, "Updater")

	if _, err := fixture.catalogService.DownloadCatalogMod(ctx, mods.DownloadModRequest{
		ModID: "51", VersionID: "7", InstanceIDs: []string{instance.ID},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.catalogService.DownloadCatalogMod(ctx, mods.DownloadModRequest{
		ModID: "51", VersionID: "9", InstanceIDs: []string{instance.ID},
	}); err != nil {
		t.Fatal(err)
	}

	downloaded, err := fixture.catalogService.ListDownloadedMods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(downloaded) != 1 {
		t.Fatalf("superseded cache version was not removed, got %#v", downloaded)
	}
	if downloaded[0].VersionID != "9" || downloaded[0].DownloadedVersion != "2.1.0" {
		t.Fatalf("unexpected surviving version: %#v", downloaded[0])
	}
}

func TestUpdateKeepsCacheVersionUsedByAnotherInstance(t *testing.T) {
	fixture := newTestFixtureWithDeps(t, twoVersionCorpseCatalog(), modArchiveDownloader{})
	ctx := context.Background()
	first := fixture.createTestInstance(t, "Old")
	second := fixture.createTestInstance(t, "New")

	// Both instances start on version 7.
	if _, err := fixture.catalogService.DownloadCatalogMod(ctx, mods.DownloadModRequest{
		ModID: "51", VersionID: "7", InstanceIDs: []string{first.ID, second.ID},
	}); err != nil {
		t.Fatal(err)
	}
	// Only the second instance is updated to version 9.
	if _, err := fixture.catalogService.DownloadCatalogMod(ctx, mods.DownloadModRequest{
		ModID: "51", VersionID: "9", InstanceIDs: []string{second.ID},
	}); err != nil {
		t.Fatal(err)
	}

	downloaded, err := fixture.catalogService.ListDownloadedMods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(downloaded) != 2 {
		t.Fatalf("cache version still used by another instance must be kept, got %#v", downloaded)
	}
	byVersion := map[string]mods.DownloadedMod{}
	for _, mod := range downloaded {
		byVersion[mod.VersionID] = mod
	}
	if len(byVersion["7"].InstalledInstances) != 1 || len(byVersion["9"].InstalledInstances) != 1 {
		t.Fatalf("each version must list its own instance: %#v", downloaded)
	}
}
