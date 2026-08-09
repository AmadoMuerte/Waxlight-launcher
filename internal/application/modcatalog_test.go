package application_test

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/modstorage"
)

type recordedEvent struct {
	name    string
	payload any
}

type recordingEventPublisher struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (publisher *recordingEventPublisher) Publish(name string, payload any) {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	publisher.events = append(publisher.events, recordedEvent{name: name, payload: payload})
}

func (publisher *recordingEventPublisher) last(name string) (any, bool) {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	for index := len(publisher.events) - 1; index >= 0; index-- {
		if publisher.events[index].name == name {
			return publisher.events[index].payload, true
		}
	}
	return nil, false
}

func (publisher *recordingEventPublisher) clear() {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	publisher.events = nil
}

type staticModCatalog struct {
	details     domain.ModDetails
	detailsByID map[string]domain.ModDetails
}

func (catalog staticModCatalog) List(context.Context) ([]domain.ModSummary, error) {
	if len(catalog.detailsByID) > 0 {
		items := make([]domain.ModSummary, 0, len(catalog.detailsByID))
		for _, details := range catalog.detailsByID {
			items = append(items, details.ModSummary)
		}
		return items, nil
	}
	return []domain.ModSummary{catalog.details.ModSummary}, nil
}

func (catalog staticModCatalog) Search(
	context.Context,
	domain.ModSearchQuery,
) (domain.ModSearchResult, error) {
	return domain.ModSearchResult{Items: []domain.ModSummary{catalog.details.ModSummary}, Page: 1, PageSize: 24, TotalItems: 1}, nil
}

func (catalog staticModCatalog) Get(_ context.Context, modID string) (domain.ModDetails, error) {
	if len(catalog.detailsByID) == 0 {
		return catalog.details, nil
	}
	if details, ok := catalog.detailsByID[modID]; ok {
		return details, nil
	}
	for _, details := range catalog.detailsByID {
		if details.ID == modID || details.Slug == modID {
			return details, nil
		}
	}
	return domain.ModDetails{}, domain.NewError(domain.ErrModNotFound, "Mod not found")
}

func (catalog staticModCatalog) ListTags(context.Context) ([]domain.ModTag, error) {
	return []domain.ModTag{}, nil
}

type modArchiveDownloader struct {
	manifests map[string]map[string]any
}

func (downloader modArchiveDownloader) Download(
	_ context.Context,
	request application.DownloadRequest,
	progress chan<- application.DownloadProgress,
) error {
	if err := os.MkdirAll(filepath.Dir(request.DestinationPath), 0o755); err != nil {
		return err
	}
	file, err := os.Create(request.DestinationPath)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(file)
	modInfo, err := archive.Create("modinfo.json")
	if err != nil {
		_ = file.Close()
		return err
	}
	manifest := downloader.manifests[request.URL]
	if manifest == nil {
		manifest = map[string]any{
			"modid":        "testmod",
			"name":         "Test mod",
			"version":      "1.0.0",
			"dependencies": map[string]string{},
		}
	}
	if err := json.NewEncoder(modInfo).Encode(manifest); err != nil {
		_ = archive.Close()
		_ = file.Close()
		return err
	}
	if err := archive.Close(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	info, err := os.Stat(request.DestinationPath)
	if err != nil {
		return err
	}
	progress <- application.DownloadProgress{
		DownloadedBytes: info.Size(),
		TotalBytes:      info.Size(),
		BytesPerSecond:  info.Size(),
	}
	return nil
}

func (downloader modArchiveDownloader) ContentLength(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func TestDownloadCatalogModInstallsIntoSeveralInstances(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	first, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{Name: "First", GameVersionID: "1.20"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{Name: "Second", GameVersionID: "1.20"})
	if err != nil {
		t.Fatal(err)
	}
	details := domain.ModDetails{ModSummary: domain.ModSummary{ID: "51", Name: "Player Corpse", AuthorName: "Ada", LatestVersion: "2.0.0"}, Versions: []domain.ModVersion{{
		ID: "7", Version: "2.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable",
		FileName: "playercorpse.zip", DownloadURL: "https://cdn.test/playercorpse.zip",
	}}}
	fixture.service.ConfigureVersionDownloads(nil, recordingDownloader{}, nil)
	fixture.service.ConfigureMods(staticModCatalog{details: details}, modstorage.New(fixture.root))
	result, err := fixture.service.DownloadCatalogMod(ctx, domain.DownloadModRequest{
		ModID: "51", VersionID: "7", InstanceIDs: []string{first.ID, second.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Installations) != 2 || !result.Installations[0].Installed || !result.Installations[1].Installed {
		t.Fatalf("unexpected installation result: %#v", result.Installations)
	}
	for _, instance := range []domain.Instance{first, second} {
		installed, listErr := fixture.store.ListMods(ctx, instance.ID)
		if listErr != nil || len(installed) != 1 || installed[0].Source != "moddb:51:7" {
			t.Fatalf("unexpected installed mods: %#v, %v", installed, listErr)
		}
		if _, statErr := os.Stat(filepath.Join(instance.Directory, "Mods", "playercorpse.zip")); statErr != nil {
			t.Fatal(statErr)
		}
	}

	// Installing an already downloaded version reuses the cache and does not duplicate records.
	result, err = fixture.service.InstallDownloadedMod(ctx, "51", "7", []string{first.ID}, false)
	if err != nil || !result.Installations[0].Installed {
		t.Fatalf("unexpected repeated install: %#v, %v", result, err)
	}
	installed, _ := fixture.store.ListMods(ctx, first.ID)
	if len(installed) != 1 {
		t.Fatalf("repeated install created %d records", len(installed))
	}
}

func TestDownloadCatalogModsBatchContinuesAfterTargetFailure(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name: "Batch", GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	first := domain.ModDetails{ModSummary: domain.ModSummary{ID: "first", Name: "First"}, Versions: []domain.ModVersion{{
		ID: "1", Version: "1.0.0", GameVersions: []string{"1.19"}, FileName: "first.zip", DownloadURL: "https://cdn.test/first.zip",
	}}}
	second := domain.ModDetails{ModSummary: domain.ModSummary{ID: "second", Name: "Second"}, Versions: []domain.ModVersion{{
		ID: "2", Version: "1.0.0", GameVersions: []string{"1.20"}, FileName: "second.zip", DownloadURL: "https://cdn.test/second.zip",
	}}}
	fixture.service.ConfigureVersionDownloads(nil, recordingDownloader{}, nil)
	fixture.service.ConfigureMods(staticModCatalog{detailsByID: map[string]domain.ModDetails{
		"first": first, "second": second,
	}}, modstorage.New(fixture.root))

	results := fixture.service.DownloadCatalogModsBatch(ctx, domain.BatchDownloadModsRequest{
		InstanceID: instance.ID,
		Targets: []domain.DownloadModTarget{
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
	installed, err := fixture.store.ListMods(ctx, instance.ID)
	if err != nil || len(installed) != 2 {
		t.Fatalf("unexpected installed mods: %#v, %v", installed, err)
	}
}

func TestDownloadCatalogModResolvesAndInstallsDependencies(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	events := &recordingEventPublisher{}
	fixture.service.SetEventPublisher(events)
	instance, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name:          "Dependency test",
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}

	rootMod := domain.ModDetails{
		ModSummary: domain.ModSummary{
			ID:            "51",
			Slug:          "rootmod",
			Name:          "Root Mod",
			AuthorName:    "Ada",
			LatestVersion: "2.0.0",
		},
		Versions: []domain.ModVersion{{
			ID:           "7",
			Version:      "2.0.0",
			GameVersions: []string{"1.20"},
			ReleaseType:  "stable",
			FileName:     "rootmod.zip",
			DownloadURL:  "https://cdn.test/rootmod.zip",
		}},
	}
	expandedMatter := domain.ModDetails{
		ModSummary: domain.ModSummary{
			ID:            "60",
			Slug:          "em",
			Name:          "Expanded Matter",
			AuthorName:    "Grace",
			LatestVersion: "3.4.0",
		},
		Versions: []domain.ModVersion{
			{
				ID:           "12",
				Version:      "3.2.0",
				GameVersions: []string{"1.20"},
				ReleaseType:  "stable",
				FileName:     "em-3.2.0.zip",
				DownloadURL:  "https://cdn.test/em-3.2.0.zip",
			},
			{
				ID:           "13",
				Version:      "3.3.3",
				GameVersions: []string{"1.20"},
				ReleaseType:  "stable",
				FileName:     "em-3.3.3.zip",
				DownloadURL:  "https://cdn.test/em-3.3.3.zip",
			},
			{
				ID:           "14",
				Version:      "3.4.0",
				GameVersions: []string{"1.20"},
				ReleaseType:  "stable",
				FileName:     "em-3.4.0.zip",
				DownloadURL:  "https://cdn.test/em-3.4.0.zip",
			},
		},
	}
	configLib := domain.ModDetails{
		ModSummary: domain.ModSummary{
			ID:            "70",
			Slug:          "configlib",
			Name:          "Config Lib",
			AuthorName:    "Linus",
			LatestVersion: "1.2.0",
		},
		Versions: []domain.ModVersion{{
			ID:           "21",
			Version:      "1.2.0",
			GameVersions: []string{"1.20"},
			ReleaseType:  "stable",
			FileName:     "configlib.zip",
			DownloadURL:  "https://cdn.test/configlib.zip",
		}},
	}

	fixture.service.ConfigureVersionDownloads(nil, modArchiveDownloader{
		manifests: map[string]map[string]any{
			"https://cdn.test/rootmod.zip": {
				"modid":   "rootmod",
				"name":    "Root Mod",
				"version": "2.0.0",
				"dependencies": map[string]string{
					"game": "1.20",
					"em":   "3.3.3",
				},
			},
			"https://cdn.test/em-3.3.3.zip": {
				"modid":   "em",
				"name":    "Expanded Matter",
				"version": "3.3.3",
				"dependencies": map[string]string{
					"survival":  "*",
					"configlib": "1.1.0",
				},
			},
			"https://cdn.test/configlib.zip": {
				"modid":        "configlib",
				"name":         "Config Lib",
				"version":      "1.2.0",
				"dependencies": map[string]string{},
			},
		},
	}, nil)
	fixture.service.ConfigureMods(staticModCatalog{
		detailsByID: map[string]domain.ModDetails{
			"51":        rootMod,
			"rootmod":   rootMod,
			"60":        expandedMatter,
			"em":        expandedMatter,
			"70":        configLib,
			"configlib": configLib,
		},
	}, modstorage.New(fixture.root))

	result, err := fixture.service.DownloadCatalogMod(ctx, domain.DownloadModRequest{
		ModID:       "51",
		VersionID:   "7",
		InstanceIDs: []string{instance.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Installations) != 1 || !result.Installations[0].Installed {
		t.Fatalf("unexpected installation result: %#v", result.Installations)
	}

	installed, err := fixture.store.ListMods(ctx, instance.ID)
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

	payload, ok := events.last("mods:downloads-changed")
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

	events.clear()
	if _, err := fixture.service.InstallDownloadedMod(ctx, "51", "7", []string{instance.ID}, false); err != nil {
		t.Fatal(err)
	}
	payload, ok = events.last("mods:downloads-changed")
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
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name: "Shared dependency", GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}

	root := domain.ModDetails{ModSummary: domain.ModSummary{ID: "root", Name: "Root"}, Versions: []domain.ModVersion{{
		ID: "root-1", Version: "1.0.0", GameVersions: []string{"1.20"}, FileName: "root.zip", DownloadURL: "https://cdn.test/root.zip",
	}}}
	first := domain.ModDetails{ModSummary: domain.ModSummary{ID: "first", Name: "First"}, Versions: []domain.ModVersion{{
		ID: "first-1", Version: "1.0.0", GameVersions: []string{"1.20"}, FileName: "first.zip", DownloadURL: "https://cdn.test/first.zip",
	}}}
	second := domain.ModDetails{ModSummary: domain.ModSummary{ID: "second", Name: "Second"}, Versions: []domain.ModVersion{{
		ID: "second-1", Version: "1.0.0", GameVersions: []string{"1.20"}, FileName: "second.zip", DownloadURL: "https://cdn.test/second.zip",
	}}}
	library := domain.ModDetails{ModSummary: domain.ModSummary{ID: "library", Name: "Library"}, Versions: []domain.ModVersion{
		{ID: "library-9", Version: "2.0.9", GameVersions: []string{"1.20"}, FileName: "library-9.zip", DownloadURL: "https://cdn.test/library-9.zip"},
		{ID: "library-10", Version: "2.0.10", GameVersions: []string{"1.20"}, FileName: "library-10.zip", DownloadURL: "https://cdn.test/library-10.zip"},
	}}
	fixture.service.ConfigureVersionDownloads(nil, modArchiveDownloader{manifests: map[string]map[string]any{
		"https://cdn.test/root.zip":       {"modid": "root", "version": "1.0.0", "dependencies": map[string]string{"first": "1.0.0", "second": "1.0.0"}},
		"https://cdn.test/first.zip":      {"modid": "first", "version": "1.0.0", "dependencies": map[string]string{"library": "2.0.9"}},
		"https://cdn.test/second.zip":     {"modid": "second", "version": "1.0.0", "dependencies": map[string]string{"library": "2.0.10"}},
		"https://cdn.test/library-9.zip":  {"modid": "library", "version": "2.0.9", "dependencies": map[string]string{}},
		"https://cdn.test/library-10.zip": {"modid": "library", "version": "2.0.10", "dependencies": map[string]string{}},
	}}, nil)
	fixture.service.ConfigureMods(staticModCatalog{detailsByID: map[string]domain.ModDetails{
		"root": root, "first": first, "second": second, "library": library,
	}}, modstorage.New(fixture.root))

	if _, err := fixture.service.DownloadCatalogMod(ctx, domain.DownloadModRequest{
		ModID: "root", VersionID: "root-1", InstanceIDs: []string{instance.ID},
	}); err != nil {
		t.Fatal(err)
	}
	installed, err := fixture.store.ListMods(ctx, instance.ID)
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
	details := domain.ModDetails{
		ModSummary: domain.ModSummary{
			ID: "51", Name: "Player Corpse", AuthorName: "Ada", LatestVersion: "2.1.0",
		},
		Versions: []domain.ModVersion{
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
	fixture := newTestFixture(t)
	ctx := context.Background()
	fixture.service.ConfigureVersionDownloads(nil, modArchiveDownloader{}, nil)
	fixture.service.ConfigureMods(twoVersionCorpseCatalog(), modstorage.New(fixture.root))
	instance, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name: "Updater", GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.service.DownloadCatalogMod(ctx, domain.DownloadModRequest{
		ModID: "51", VersionID: "7", InstanceIDs: []string{instance.ID},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.DownloadCatalogMod(ctx, domain.DownloadModRequest{
		ModID: "51", VersionID: "9", InstanceIDs: []string{instance.ID},
	}); err != nil {
		t.Fatal(err)
	}

	downloaded, err := fixture.service.ListDownloadedMods(ctx)
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
	fixture := newTestFixture(t)
	ctx := context.Background()
	fixture.service.ConfigureVersionDownloads(nil, modArchiveDownloader{}, nil)
	fixture.service.ConfigureMods(twoVersionCorpseCatalog(), modstorage.New(fixture.root))
	first, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name: "Old", GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name: "New", GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Both instances start on version 7.
	if _, err := fixture.service.DownloadCatalogMod(ctx, domain.DownloadModRequest{
		ModID: "51", VersionID: "7", InstanceIDs: []string{first.ID, second.ID},
	}); err != nil {
		t.Fatal(err)
	}
	// Only the second instance is updated to version 9.
	if _, err := fixture.service.DownloadCatalogMod(ctx, domain.DownloadModRequest{
		ModID: "51", VersionID: "9", InstanceIDs: []string{second.ID},
	}); err != nil {
		t.Fatal(err)
	}

	downloaded, err := fixture.service.ListDownloadedMods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(downloaded) != 2 {
		t.Fatalf("cache version still used by another instance must be kept, got %#v", downloaded)
	}
	byVersion := map[string]domain.DownloadedMod{}
	for _, mod := range downloaded {
		byVersion[mod.VersionID] = mod
	}
	if len(byVersion["7"].InstalledInstances) != 1 || len(byVersion["9"].InstalledInstances) != 1 {
		t.Fatalf("each version must list its own instance: %#v", downloaded)
	}
}
