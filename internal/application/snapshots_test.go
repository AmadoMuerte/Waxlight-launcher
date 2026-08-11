package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/downloads"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/filesystem"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/modstorage"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/snapshotstore"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/mods"
)

func createSnapshotTestInstance(t *testing.T, fixture testFixture, name string) instances.Instance {
	t.Helper()
	instance, err := fixture.service.CreateInstance(context.Background(), instances.CreateInput{
		Name:          name,
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	return instance
}

func writeTestInstanceFiles(t *testing.T, instance instances.Instance) {
	t.Helper()
	files := map[string]string{
		"SaveGame/world1/Main/level.bin": "world-bytes",
		"SaveGame/notes.txt":             "not-a-world",
		"Config/modconfig.json":          `{"enabled":true}`,
		"file.txt":                       "before",
		"clientsettings.json":            `{"stringsettings":{"sessionkey":"TOP_SECRET","playername":"gasada","fov":80},"intsettings":{"viewDistance":256}}`,
	}
	for relative, contents := range files {
		path := filepath.Join(instance.Directory, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func writeTestInstanceModFiles(t *testing.T, instance instances.Instance) {
	t.Helper()
	for relative, contents := range map[string]string{
		"Mods/modpack.zip":            "mod-bytes",
		"ModsDisabled/offline-mod.cs": "code-bytes",
	} {
		path := filepath.Join(instance.Directory, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// installManagedMod writes a mod file into an instance and registers it as a
// Waxlight-managed ModDB mod.
func installManagedMod(
	t *testing.T,
	fixture testFixture,
	instance instances.Instance,
	name string,
	fileName string,
	modID string,
	releaseID string,
	version string,
	enabled bool,
) {
	t.Helper()
	subdirectory := "Mods"
	if !enabled {
		subdirectory = "ModsDisabled"
	}
	path := filepath.Join(instance.Directory, subdirectory, fileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(name+"-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	record := mods.InstalledMod{
		ID:          newTestModID(),
		InstanceID:  instance.ID,
		Name:        name,
		Version:     version,
		FileName:    fileName,
		FilePath:    path,
		Enabled:     enabled,
		Managed:     true,
		Source:      "moddb:" + modID + ":" + releaseID,
		SizeBytes:   int64(len(name + "-bytes")),
		InstalledAt: time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := fixture.store.SaveMod(context.Background(), record); err != nil {
		t.Fatal(err)
	}
}

// cacheModRelease stores a downloaded release artifact in the shared mod
// download cache.
func cacheModRelease(
	t *testing.T,
	fixture testFixture,
	modID string,
	releaseID string,
	slug string,
	name string,
	version string,
	fileName string,
	checksum string,
) {
	t.Helper()
	path := filepath.Join(fixture.root, "cache", "mods", modID, releaseID, fileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(name+"-cached"), 0o644); err != nil {
		t.Fatal(err)
	}
	downloaded := mods.DownloadedMod{
		SchemaVersion:     1,
		ModID:             modID,
		Slug:              slug,
		Name:              name,
		VersionID:         releaseID,
		DownloadedVersion: version,
		FileName:          fileName,
		FilePath:          path,
		FileSize:          int64(len(name + "-cached")),
		Checksum:          checksum,
		DownloadURL:       "https://mods.example/" + slug + "_" + version + ".zip",
		DownloadedAt:      time.Now().UTC(),
	}
	if err := modstorage.New(fixture.root).Save(context.Background(), downloaded); err != nil {
		t.Fatal(err)
	}
}

var snapshotModCounter int

func newTestModID() string {
	snapshotModCounter++
	return fmt.Sprintf("test-mod-%d", snapshotModCounter)
}

func snapshotManifest(t *testing.T, snapshotDir string) domain.SnapshotManifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(snapshotDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest domain.SnapshotManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func appErrorCode(t *testing.T, err error) string {
	t.Helper()
	var appError *domain.AppError
	if !errors.As(err, &appError) {
		t.Fatalf("expected an AppError, got %v", err)
	}
	return appError.Code
}

func TestCreateInstanceSnapshotCapturesInstanceData(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Survival")
	writeTestInstanceFiles(t, instance)
	writeTestInstanceModFiles(t, instance)

	operation, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if operation.Status != "completed" {
		t.Fatalf("unexpected operation status %q", operation.Status)
	}

	snapshotDir, err := snapshotstore.New(fixture.root).SnapshotDir(instance.ID, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(snapshotDir); err != nil {
		t.Fatalf("snapshot directory was not created: %v", err)
	}
	manifest := snapshotManifest(t, snapshotDir)
	if manifest.FormatVersion != domain.SnapshotFormatVersion {
		t.Fatalf("unexpected format version %d", manifest.FormatVersion)
	}
	if manifest.ID != operation.ID || manifest.InstanceID != instance.ID {
		t.Fatalf("unexpected manifest identifiers %#v", manifest)
	}
	if manifest.InstanceName != "Survival" {
		t.Fatalf("unexpected instance name %q", manifest.InstanceName)
	}
	if manifest.Type != domain.SnapshotTypeManual {
		t.Fatalf("unexpected snapshot type %q", manifest.Type)
	}
	if manifest.GameVersion != "1.20" {
		t.Fatalf("unexpected game version %q", manifest.GameVersion)
	}
	if manifest.ModCount != 2 {
		t.Fatalf("expected 2 mods, got %d", manifest.ModCount)
	}
	if manifest.WorldCount != 1 {
		t.Fatalf("expected 1 world, got %d", manifest.WorldCount)
	}
	if len(manifest.Mods) != 2 {
		t.Fatalf("expected 2 mod entries, got %d", len(manifest.Mods))
	}
	for _, mod := range manifest.Mods {
		if mod.Source != domain.SnapshotModSourceUnknown {
			t.Fatalf("manually installed mod has an unexpected source %q", mod.Source)
		}
		if mod.Identifier == "" || mod.FileName == "" {
			t.Fatalf("manual mod entry misses its identity: %#v", mod)
		}
	}

	dataDir := snapshotstore.DataDir(snapshotDir)
	for _, relative := range []string{
		"SaveGame/world1/Main/level.bin",
		"SaveGame/notes.txt",
		"Config/modconfig.json",
		"file.txt",
		"clientsettings.json",
	} {
		if _, err := os.Stat(filepath.Join(dataDir, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("snapshot misses %s: %v", relative, err)
		}
	}
	for _, relative := range []string{"Mods/modpack.zip", "ModsDisabled/offline-mod.cs"} {
		if _, err := os.Stat(filepath.Join(dataDir, filepath.FromSlash(relative))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("snapshot must not contain the mod binary %s: %v", relative, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dataDir, ".waxlight-instance")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot must not contain the instance marker: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "Logs")); err != nil {
		t.Fatalf("snapshot misses the instance Logs directory: %v", err)
	}
	if size, err := snapshotstore.Size(snapshotDir); err != nil || size != manifest.SizeBytes {
		t.Fatalf("manifest size %d does not match the stored data %d (%v)", manifest.SizeBytes, size, err)
	}

	sanitized, err := os.ReadFile(filepath.Join(dataDir, "clientsettings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sanitized), "TOP_SECRET") || strings.Contains(string(sanitized), "gasada") {
		t.Fatalf("snapshot kept authentication data: %s", sanitized)
	}
	var settings map[string]map[string]any
	if err := json.Unmarshal(sanitized, &settings); err != nil {
		t.Fatal(err)
	}
	if settings["intsettings"]["viewDistance"] != float64(256) {
		t.Fatalf("sanitization removed non-auth settings: %s", sanitized)
	}

	contents, err := os.ReadFile(filepath.Join(instance.Directory, "clientsettings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "TOP_SECRET") {
		t.Fatal("source instance clientsettings were modified")
	}
	if contents, err = os.ReadFile(filepath.Join(instance.Directory, "file.txt")); err != nil || string(contents) != "before" {
		t.Fatalf("source instance file changed: %q, %v", contents, err)
	}
	if _, err := os.Stat(filepath.Join(instance.Directory, "Mods", "modpack.zip")); err != nil {
		t.Fatalf("source instance mod was removed: %v", err)
	}
}

func TestCreateInstanceSnapshotStoresManagedModReleases(t *testing.T) {
	fixture := newTestFixtureWithMods(t, nil)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Managed mods")
	writeTestInstanceFiles(t, instance)
	installManagedMod(t, fixture, instance, "Mod A", "mod-a.zip", "100", "1000", "1.0.0", true)
	installManagedMod(t, fixture, instance, "Mod B", "mod-b.zip", "200", "2000", "2.4.1", false)
	cacheModRelease(t, fixture, "100", "1000", "mod-a", "Mod A", "1.0.0", "mod-a_1.0.0.zip", "sha256:aaaa")
	cacheModRelease(t, fixture, "200", "2000", "mod-b", "Mod B", "2.4.1", "mod-b_2.4.1.zip", "sha256:bbbb")

	operation, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshotDir, err := snapshotstore.New(fixture.root).SnapshotDir(instance.ID, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	manifest := snapshotManifest(t, snapshotDir)
	if manifest.ModCount != 2 || len(manifest.Mods) != 2 {
		t.Fatalf("expected 2 managed mod entries, got %d/%d", manifest.ModCount, len(manifest.Mods))
	}

	byID := map[string]domain.SnapshotMod{}
	for _, mod := range manifest.Mods {
		if mod.Source != domain.SnapshotModSourceModDB {
			t.Fatalf("managed mod has unexpected source %q", mod.Source)
		}
		byID[mod.ModID] = mod
	}
	modA := byID["100"]
	if modA.ReleaseID != "1000" || modA.Identifier != "mod-a" || modA.Version != "1.0.0" ||
		modA.FileName != "mod-a_1.0.0.zip" || modA.SHA256 != "sha256:aaaa" || !modA.Enabled {
		t.Fatalf("managed mod A entry is incomplete: %#v", modA)
	}
	modB := byID["200"]
	if modB.ReleaseID != "2000" || modB.Identifier != "mod-b" || modB.Version != "2.4.1" ||
		modB.FileName != "mod-b_2.4.1.zip" || modB.SHA256 != "sha256:bbbb" || modB.Enabled {
		t.Fatalf("managed mod B entry is incomplete: %#v", modB)
	}

	dataDir := snapshotstore.DataDir(snapshotDir)
	for _, relative := range []string{"Mods/mod-a.zip", "ModsDisabled/mod-b.zip"} {
		if _, err := os.Stat(filepath.Join(dataDir, filepath.FromSlash(relative))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("snapshot must not contain the managed mod binary %s: %v", relative, err)
		}
	}
	expectedSize := len("world-bytes") + len("not-a-world") + len(`{"enabled":true}`) + len("before")
	sanitized, err := filesystem.SanitizeClientSettings([]byte(`{"stringsettings":{"sessionkey":"TOP_SECRET","playername":"gasada","fov":80},"intsettings":{"viewDistance":256}}`))
	if err != nil {
		t.Fatal(err)
	}
	expectedSize += len(sanitized)
	if manifest.SizeBytes != int64(expectedSize) {
		t.Fatalf("snapshot size %d does not exclude mod binaries (want %d)", manifest.SizeBytes, expectedSize)
	}

	if _, err := os.Stat(filepath.Join(instance.Directory, "Mods", "mod-a.zip")); err != nil {
		t.Fatalf("source instance mod was removed: %v", err)
	}
}

func TestCreateInstanceSnapshotRejectsRunningInstance(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Running")
	writeTestInstanceFiles(t, instance)

	if _, err := fixture.launching.Launch(ctx, instance.ID, nil); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fixture.launching.Stop(ctx, instance.ID, true) }()

	_, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if code := appErrorCode(t, err); code != instances.ErrInstanceRunning {
		t.Fatalf("expected INSTANCE_ALREADY_RUNNING, got %s", code)
	}
	snapshots, err := fixture.service.ListInstanceSnapshots(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("a snapshot was created while the game was running: %d", len(snapshots))
	}
}

func TestCreateInstanceSnapshotRejectsInsufficientSpace(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Full disk")
	writeTestInstanceFiles(t, instance)
	fixture.setDiskSpace(fixedDiskSpace(0))

	_, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if code := appErrorCode(t, err); code != domain.ErrInsufficientSpace {
		t.Fatalf("expected INSUFFICIENT_DISK_SPACE, got %s", code)
	}
	snapshots, err := fixture.service.ListInstanceSnapshots(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("a snapshot was created despite the disk space check: %d", len(snapshots))
	}
}

func TestCreateInstanceSnapshotCleansUpOnFailure(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Broken settings")
	writeTestInstanceFiles(t, instance)
	// An unreadable clientsettings.json makes the sanitizing copy fail.
	if err := os.WriteFile(
		filepath.Join(instance.Directory, "clientsettings.json"),
		[]byte("{not json"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	_, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err == nil {
		t.Fatal("expected a failure")
	}
	instanceDir, err := snapshotstore.New(fixture.root).InstancePath(instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(instanceDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			t.Fatalf("staging directory was left behind: %s", entry.Name())
		}
	}
	snapshots, err := fixture.service.ListInstanceSnapshots(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("an incomplete snapshot is visible: %d", len(snapshots))
	}
}

func TestListInstanceSnapshotsNewestFirst(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Ordered")
	writeTestInstanceFiles(t, instance)
	writeTestInstanceModFiles(t, instance)

	first, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	second, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}

	snapshots, err := fixture.service.ListInstanceSnapshots(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snapshots))
	}
	if snapshots[0].ID != second.ID || snapshots[1].ID != first.ID {
		t.Fatalf("snapshots are not sorted newest first: %s, %s", snapshots[0].ID, snapshots[1].ID)
	}
	if snapshots[0].ModCount != 2 || snapshots[0].WorldCount != 1 {
		t.Fatalf("snapshot summary misses counts: %#v", snapshots[0])
	}
}

func TestListInstanceSnapshotsSkipsMalformedDirectories(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Robust")
	writeTestInstanceFiles(t, instance)

	valid, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}

	instanceDir, err := snapshotstore.New(fixture.root).InstanceDir(instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, damaged := range []struct {
		name string
		prep func(dir string)
	}{
		{name: "garbage-manifest", prep: func(dir string) {
			os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{not json"), 0o600)
		}},
		{name: "no-manifest", prep: func(dir string) {
			os.WriteFile(filepath.Join(dir, "stray.bin"), []byte("data"), 0o600)
		}},
		{name: "unsupported-format", prep: func(dir string) {
			os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"formatVersion":99,"id":"x","instanceId":"y","type":"manual","createdAt":"2026-01-01T00:00:00Z"}`), 0o600)
		}},
	} {
		dir := filepath.Join(instanceDir, damaged.name)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		damaged.prep(dir)
	}

	snapshots, err := fixture.service.ListInstanceSnapshots(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 || snapshots[0].ID != valid.ID {
		t.Fatalf("malformed snapshots broke the listing: %#v", snapshots)
	}
}

func TestRestoreInstanceSnapshotReproducesExactState(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Restorable")
	writeTestInstanceFiles(t, instance)

	operation, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(instance.Directory, "file.txt"), []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instance.Directory, "new-file.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := fixture.service.RestoreInstanceSnapshot(ctx, instance.ID, operation.ID); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(filepath.Join(instance.Directory, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "before" {
		t.Fatalf("restored file.txt = %q, want %q", contents, "before")
	}
	if _, err := os.Stat(filepath.Join(instance.Directory, "new-file.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file added after the snapshot still exists after restore: %v", err)
	}
	marker, err := os.ReadFile(filepath.Join(instance.Directory, ".waxlight-instance"))
	if err != nil {
		t.Fatal(err)
	}
	if string(marker) != instance.ID {
		t.Fatalf("restored instance marker = %q, want %q", marker, instance.ID)
	}
	if _, err := os.Stat(filepath.Join(instance.Directory, "SaveGame", "world1", "Main", "level.bin")); err != nil {
		t.Fatalf("restored instance misses snapshot files: %v", err)
	}
}

func TestRestoreInstanceSnapshotDownloadsExactReleases(t *testing.T) {
	catalog := staticModCatalog{detailsByID: map[string]mods.ModDetails{
		"100": {
			ModSummary: mods.ModSummary{ID: "100", Slug: "mod-a", Name: "Mod A"},
			Versions: []mods.ModVersion{
				{ID: "1000", Version: "1.0.0", GameVersions: []string{"1.20"}, FileName: "mod-a_1.0.0.zip", DownloadURL: "https://mods.example/mod-a_1.0.0.zip"},
				{ID: "1100", Version: "2.0.0", GameVersions: []string{"1.20"}, FileName: "mod-a_2.0.0.zip", DownloadURL: "https://mods.example/mod-a_2.0.0.zip"},
			},
		},
		"200": {
			ModSummary: mods.ModSummary{ID: "200", Slug: "mod-b", Name: "Mod B"},
			Versions: []mods.ModVersion{
				{ID: "2000", Version: "2.4.1", GameVersions: []string{"1.20"}, FileName: "mod-b_2.4.1.zip", DownloadURL: "https://mods.example/mod-b_2.4.1.zip"},
				{ID: "2100", Version: "3.0.0", GameVersions: []string{"1.20"}, FileName: "mod-b_3.0.0.zip", DownloadURL: "https://mods.example/mod-b_3.0.0.zip"},
			},
		},
	}}
	fixture := newTestFixtureWithMods(t, catalog)
	fixture.downloader.Set(recordingDownloader{})
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Exact releases")
	writeTestInstanceFiles(t, instance)
	installManagedMod(t, fixture, instance, "Mod A", "mod-a.zip", "100", "1000", "1.0.0", true)
	installManagedMod(t, fixture, instance, "Mod B", "mod-b.zip", "200", "2000", "2.4.1", false)

	operation, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshotDir, err := snapshotstore.New(fixture.root).SnapshotDir(instance.ID, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	manifest := snapshotManifest(t, snapshotDir)
	if len(manifest.Mods) != 2 {
		t.Fatalf("expected 2 managed mods in the snapshot, got %d", len(manifest.Mods))
	}

	// Change the current instance: A and B replaced by C, data modified.
	installManagedMod(t, fixture, instance, "Mod C", "mod-c.zip", "300", "3000", "5.1.0", true)
	if err := os.Remove(filepath.Join(instance.Directory, "Mods", "mod-a.zip")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(instance.Directory, "ModsDisabled", "mod-b.zip")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instance.Directory, "file.txt"), []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := fixture.service.RestoreInstanceSnapshot(ctx, instance.ID, operation.ID); err != nil {
		t.Fatal(err)
	}

	if contents, err := os.ReadFile(filepath.Join(instance.Directory, "file.txt")); err != nil || string(contents) != "before" {
		t.Fatalf("instance data was not restored: %q, %v", contents, err)
	}
	// The exact releases, not the latest ones, must be in the cache.
	cache := modstorage.New(fixture.root)
	if _, err := cache.Get(ctx, "100", "1000"); err != nil {
		t.Fatalf("exact release 100/1000 was not downloaded: %v", err)
	}
	if _, err := cache.Get(ctx, "200", "2000"); err != nil {
		t.Fatalf("exact release 200/2000 was not downloaded: %v", err)
	}
	for _, newer := range [][2]string{{"100", "1100"}, {"200", "2100"}} {
		if _, err := cache.Get(ctx, newer[0], newer[1]); err == nil {
			t.Fatalf("a newer release %s/%s was fetched", newer[0], newer[1])
		}
	}

	modsDir := filepath.Join(instance.Directory, "Mods")
	if _, err := os.Stat(filepath.Join(modsDir, "mod-a_1.0.0.zip")); err != nil {
		t.Fatalf("restored Mods misses mod-a_1.0.0.zip: %v", err)
	}
	if _, err := os.Stat(filepath.Join(instance.Directory, "ModsDisabled", "mod-b_2.4.1.zip")); err != nil {
		t.Fatalf("restored ModsDisabled misses mod-b_2.4.1.zip: %v", err)
	}
	if _, err := os.Stat(filepath.Join(modsDir, "mod-b_2.4.1.zip")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("disabled mod was not placed into ModsDisabled: %v", err)
	}
	if _, err := os.Stat(filepath.Join(modsDir, "mod-c.zip")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("mod added after the snapshot still exists: %v", err)
	}

	records, err := fixture.store.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 restored mod records, got %d", len(records))
	}
	bySource := map[string]mods.InstalledMod{}
	for _, record := range records {
		bySource[record.Source] = record
	}
	for _, source := range []string{"moddb:100:1000", "moddb:200:2000"} {
		record, ok := bySource[source]
		if !ok {
			t.Fatalf("mod record %s is missing after restore", source)
		}
		if !record.Managed {
			t.Fatalf("restored record %s lost its managed marker: %#v", source, record)
		}
	}
	if bySource["moddb:100:1000"].Enabled != true {
		t.Fatalf("enabled mod was restored disabled: %#v", bySource["moddb:100:1000"])
	}
	if bySource["moddb:200:2000"].Enabled != false {
		t.Fatalf("disabled mod was restored enabled: %#v", bySource["moddb:200:2000"])
	}
}

func TestRestoreInstanceSnapshotReusesCachedRelease(t *testing.T) {
	fixture := newTestFixtureWithMods(t, staticModCatalog{details: mods.ModDetails{}})
	fixture.downloader.Set(refusingDownloader{})
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Offline restore")
	writeTestInstanceFiles(t, instance)
	installManagedMod(t, fixture, instance, "Mod A", "mod-a.zip", "100", "1000", "1.0.0", true)
	cacheModRelease(t, fixture, "100", "1000", "mod-a", "Mod A", "1.0.0", "mod-a_1.0.0.zip", "sha256:aaaa")

	operation, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	// The downloader must never be touched: the exact release is already in
	// the shared mod cache.

	if err := fixture.service.RestoreInstanceSnapshot(ctx, instance.ID, operation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(instance.Directory, "Mods", "mod-a_1.0.0.zip")); err != nil {
		t.Fatalf("cached release was not installed: %v", err)
	}
}

func TestRestoreInstanceSnapshotFailsWhenReleaseUnavailable(t *testing.T) {
	fixture := newTestFixtureWithMods(t, staticModCatalog{details: mods.ModDetails{
		ModSummary: mods.ModSummary{ID: "100", Slug: "mod-a", Name: "Mod A"},
		Versions: []mods.ModVersion{
			{ID: "1100", Version: "2.0.0", GameVersions: []string{"1.20"}, FileName: "mod-a_2.0.0.zip", DownloadURL: "https://mods.example/mod-a_2.0.0.zip"},
		},
	}})
	fixture.downloader.Set(recordingDownloader{})
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Retracted release")
	writeTestInstanceFiles(t, instance)
	installManagedMod(t, fixture, instance, "Mod A", "mod-a.zip", "100", "1000", "1.0.0", true)

	operation, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instance.Directory, "file.txt"), []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The catalog still knows the mod but the exact release is gone.

	err = fixture.service.RestoreInstanceSnapshot(ctx, instance.ID, operation.ID)
	if code := appErrorCode(t, err); code != domain.ErrSnapshotInvalid {
		t.Fatalf("expected SNAPSHOT_INVALID, got %s", code)
	}
	if !strings.Contains(err.Error(), "no longer available") {
		t.Fatalf("unexpected error message: %v", err)
	}
	// The instance must be completely untouched.
	if contents, readErr := os.ReadFile(filepath.Join(instance.Directory, "file.txt")); readErr != nil || string(contents) != "after" {
		t.Fatalf("instance was modified by a failed restore: %q, %v", contents, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(instance.Directory, "Mods", "mod-a.zip")); statErr != nil {
		t.Fatalf("instance mod was removed by a failed restore: %v", statErr)
	}
	if leftovers, readErr := os.ReadDir(filepath.Dir(instance.Directory)); readErr == nil {
		for _, entry := range leftovers {
			if strings.HasPrefix(entry.Name(), ".waxlight-restore-") {
				t.Fatalf("staging directory was left behind: %s", entry.Name())
			}
		}
	}
}

func TestRestoreInstanceSnapshotFailsWhenDownloadFails(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Network down")
	writeTestInstanceFiles(t, instance)
	installManagedMod(t, fixture, instance, "Mod A", "mod-a.zip", "100", "1000", "1.0.0", true)

	operation, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instance.Directory, "file.txt"), []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixture = newTestFixtureWithMods(t, staticModCatalog{details: mods.ModDetails{}})
	fixture.downloader.Set(failingDownloader{})

	err = fixture.service.RestoreInstanceSnapshot(ctx, instance.ID, operation.ID)
	if err == nil {
		t.Fatal("expected a download failure")
	}
	if contents, readErr := os.ReadFile(filepath.Join(instance.Directory, "file.txt")); readErr != nil || string(contents) != "after" {
		t.Fatalf("instance was modified by a failed restore: %q, %v", contents, readErr)
	}
}

func TestRestoreInstanceSnapshotRejectsUnknownMod(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Manual mod")
	writeTestInstanceFiles(t, instance)
	writeTestInstanceModFiles(t, instance)

	operation, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instance.Directory, "file.txt"), []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}

	err = fixture.service.RestoreInstanceSnapshot(ctx, instance.ID, operation.ID)
	if code := appErrorCode(t, err); code != domain.ErrSnapshotInvalid {
		t.Fatalf("expected SNAPSHOT_INVALID, got %s", code)
	}
	if !strings.Contains(err.Error(), "cannot download automatically") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if contents, readErr := os.ReadFile(filepath.Join(instance.Directory, "file.txt")); readErr != nil || string(contents) != "after" {
		t.Fatalf("instance was modified by a failed restore: %q, %v", contents, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(instance.Directory, "Mods", "modpack.zip")); statErr != nil {
		t.Fatalf("instance mod was removed by a failed restore: %v", statErr)
	}
}

func TestRestoreInstanceSnapshotV1BackwardCompatible(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Legacy")
	writeTestInstanceFiles(t, instance)

	// Craft a legacy v1 snapshot that physically contains the Mods directory.
	snapshotDir, err := snapshotstore.New(fixture.root).SnapshotDir(instance.ID, "legacy-snap")
	if err != nil {
		t.Fatal(err)
	}
	dataDir := snapshotstore.DataDir(snapshotDir)
	modsDir := filepath.Join(dataDir, "Mods")
	if err := os.MkdirAll(modsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modsDir, "legacy-mod.zip"), []byte("legacy-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "file.txt"), []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := snapshotstore.New(fixture.root).WriteManifest(snapshotDir, domain.SnapshotManifest{
		FormatVersion: domain.SnapshotFormatVersion1,
		ID:            "legacy-snap",
		InstanceID:    instance.ID,
		InstanceName:  instance.Name,
		CreatedAt:     time.Now().UTC(),
		Type:          domain.SnapshotTypeManual,
		SizeBytes:     13,
		ModCount:      1,
	}); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(instance.Directory, "file.txt"), []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.RestoreInstanceSnapshot(ctx, instance.ID, "legacy-snap"); err != nil {
		t.Fatal(err)
	}
	if contents, err := os.ReadFile(filepath.Join(instance.Directory, "file.txt")); err != nil || string(contents) != "before" {
		t.Fatalf("legacy restore did not restore the data: %q, %v", contents, err)
	}
	if _, err := os.Stat(filepath.Join(instance.Directory, "Mods", "legacy-mod.zip")); err != nil {
		t.Fatalf("legacy snapshot mod binary was not restored: %v", err)
	}
	if marker, err := os.ReadFile(filepath.Join(instance.Directory, ".waxlight-instance")); err != nil || string(marker) != instance.ID {
		t.Fatalf("restored instance marker = %q, %v", marker, err)
	}
}

func TestRestoreInstanceSnapshotRejectsRunningInstance(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Busy restore")
	writeTestInstanceFiles(t, instance)
	operation, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.launching.Launch(ctx, instance.ID, nil); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fixture.launching.Stop(ctx, instance.ID, true) }()

	err = fixture.service.RestoreInstanceSnapshot(ctx, instance.ID, operation.ID)
	if code := appErrorCode(t, err); code != instances.ErrInstanceRunning {
		t.Fatalf("expected INSTANCE_ALREADY_RUNNING, got %s", code)
	}
}

func TestRestoreInstanceSnapshotRejectsForeignAndUnknownSnapshots(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	source := createSnapshotTestInstance(t, fixture, "Source")
	writeTestInstanceFiles(t, source)
	operation, err := fixture.service.CreateInstanceSnapshot(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	other := createSnapshotTestInstance(t, fixture, "Other")

	// A snapshot that belongs to another instance is not visible under this
	// instance, even when the identifier matches.
	err = fixture.service.RestoreInstanceSnapshot(ctx, other.ID, operation.ID)
	if code := appErrorCode(t, err); code != domain.ErrSnapshotNotFound {
		t.Fatalf("expected SNAPSHOT_NOT_FOUND, got %s", code)
	}

	// A crafted directory whose manifest names a different instance is rejected.
	snapshotDir, err := snapshotstore.New(fixture.root).SnapshotDir(other.ID, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(snapshotstore.DataDir(snapshotDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := snapshotstore.New(fixture.root).WriteManifest(snapshotDir, domain.SnapshotManifest{
		FormatVersion: domain.SnapshotFormatVersion,
		ID:            operation.ID,
		InstanceID:    source.ID,
		InstanceName:  "Source",
		CreatedAt:     time.Now().UTC(),
		Type:          domain.SnapshotTypeManual,
	}); err != nil {
		t.Fatal(err)
	}
	err = fixture.service.RestoreInstanceSnapshot(ctx, other.ID, operation.ID)
	if code := appErrorCode(t, err); code != domain.ErrSnapshotInvalid {
		t.Fatalf("expected SNAPSHOT_INVALID for a foreign snapshot, got %s", code)
	}

	// The crafted snapshot must not have damaged the target instance.
	if _, err := os.Stat(filepath.Join(other.Directory, ".waxlight-instance")); err != nil {
		t.Fatalf("target instance was damaged: %v", err)
	}
}

func TestRestoreInstanceSnapshotRejectsCorruptManifest(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Corrupt")
	writeTestInstanceFiles(t, instance)
	operation, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshotDir, err := snapshotstore.New(fixture.root).SnapshotDir(instance.ID, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	err = fixture.service.RestoreInstanceSnapshot(ctx, instance.ID, operation.ID)
	if code := appErrorCode(t, err); code != domain.ErrSnapshotInvalid {
		t.Fatalf("expected SNAPSHOT_INVALID, got %s", code)
	}
	if contents, err := os.ReadFile(filepath.Join(instance.Directory, "file.txt")); err != nil || string(contents) != "before" {
		t.Fatalf("instance was modified by a failed restore: %q, %v", contents, err)
	}
}

func TestRestoreInstanceSnapshotKeepsInstanceOnSwapFailure(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Swap safe")
	writeTestInstanceFiles(t, instance)
	operation, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instance.Directory, "file.txt"), []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A read-only parent directory makes the directory swap fail; the live
	// instance directory must stay untouched.
	if err := os.Chmod(filepath.Dir(instance.Directory), 0o500); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(filepath.Dir(instance.Directory), 0o755) }()

	err = fixture.service.RestoreInstanceSnapshot(ctx, instance.ID, operation.ID)
	if err == nil {
		t.Fatal("expected a swap failure")
	}
	if contents, err := os.ReadFile(filepath.Join(instance.Directory, "file.txt")); err != nil || string(contents) != "after" {
		t.Fatalf("instance was destroyed by a failed restore: %q, %v", contents, err)
	}
}

func TestDeleteInstanceSnapshot(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Deletable")
	writeTestInstanceFiles(t, instance)
	operation, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshotDir, err := snapshotstore.New(fixture.root).SnapshotDir(instance.ID, operation.ID)
	if err != nil {
		t.Fatal(err)
	}

	if err := fixture.service.DeleteInstanceSnapshot(ctx, instance.ID, operation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(snapshotDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot directory still exists after deletion: %v", err)
	}
	snapshots, err := fixture.service.ListInstanceSnapshots(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("snapshot still listed after deletion: %d", len(snapshots))
	}
	if contents, err := os.ReadFile(filepath.Join(instance.Directory, "file.txt")); err != nil || string(contents) != "before" {
		t.Fatalf("instance was touched by snapshot deletion: %q, %v", contents, err)
	}

	err = fixture.service.DeleteInstanceSnapshot(ctx, instance.ID, operation.ID)
	if code := appErrorCode(t, err); code != domain.ErrSnapshotNotFound {
		t.Fatalf("expected SNAPSHOT_NOT_FOUND for a second delete, got %s", code)
	}
}

func TestSnapshotOperationsRejectUnsafeIdentifiers(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Safe ids")
	writeTestInstanceFiles(t, instance)

	_, err := fixture.service.CreateInstanceSnapshot(ctx, "..")
	if code := appErrorCode(t, err); code != instances.ErrInstanceNotFound {
		t.Fatalf("expected INSTANCE_NOT_FOUND, got %s", code)
	}

	err = fixture.service.RestoreInstanceSnapshot(ctx, instance.ID, "../escape")
	if code := appErrorCode(t, err); code != domain.ErrValidation {
		t.Fatalf("expected VALIDATION_ERROR for a traversal snapshot id, got %s", code)
	}
	err = fixture.service.DeleteInstanceSnapshot(ctx, instance.ID, "..\\escape")
	if code := appErrorCode(t, err); code != domain.ErrValidation {
		t.Fatalf("expected VALIDATION_ERROR for a traversal snapshot id, got %s", code)
	}
	err = fixture.service.RestoreInstanceSnapshot(ctx, instance.ID, "missing-snapshot")
	if code := appErrorCode(t, err); code != domain.ErrSnapshotNotFound {
		t.Fatalf("expected SNAPSHOT_NOT_FOUND, got %s", code)
	}
}

// failingDownloader reports every download as failed.
type failingDownloader struct{}

func (failingDownloader) Download(
	context.Context,
	downloads.Request,
	chan<- downloads.Progress,
) error {
	return errors.New("network unavailable")
}

func (failingDownloader) ContentLength(context.Context, string) (int64, error) {
	return 0, nil
}

// refusingDownloader fails when the network is touched; restore must never use
// it when the exact release is already cached.
type refusingDownloader struct{}

func (refusingDownloader) Download(
	context.Context,
	downloads.Request,
	chan<- downloads.Progress,
) error {
	return errors.New("the downloader must not be used")
}

func (refusingDownloader) ContentLength(context.Context, string) (int64, error) {
	return 0, nil
}
