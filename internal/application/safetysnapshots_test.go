package application_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/downloads"
	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/mods"
	platformsnapshots "github.com/waxlight/waxlight-launcher/internal/platform/snapshots"
	"github.com/waxlight/waxlight-launcher/internal/snapshots"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

// automaticSnapshotRetentionCount mirrors the production retention constant;
// tests assert against the documented default of 10.
const automaticSnapshotRetentionCount = 10

// staticModCatalog serves fixed catalog details.
type staticModCatalog struct {
	details     mods.ModDetails
	detailsByID map[string]mods.ModDetails
}

func (catalog staticModCatalog) List(context.Context) ([]mods.ModSummary, error) {
	if len(catalog.detailsByID) > 0 {
		items := make([]mods.ModSummary, 0, len(catalog.detailsByID))
		for _, details := range catalog.detailsByID {
			items = append(items, details.ModSummary)
		}
		return items, nil
	}
	return []mods.ModSummary{catalog.details.ModSummary}, nil
}

func (catalog staticModCatalog) Search(
	context.Context,
	mods.ModSearchQuery,
) (mods.ModSearchResult, error) {
	return mods.ModSearchResult{Items: []mods.ModSummary{catalog.details.ModSummary}, Page: 1, PageSize: 24, TotalItems: 1}, nil
}

func (catalog staticModCatalog) Get(_ context.Context, modID string) (mods.ModDetails, error) {
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
	return mods.ModDetails{}, errs.NewError(mods.ErrModNotFound, "Mod not found")
}

func (catalog staticModCatalog) ListTags(context.Context) ([]mods.ModTag, error) {
	return []mods.ModTag{}, nil
}

// twoReleaseMod builds catalog details for a mod with an old and a new
// release.
func twoReleaseMod(modID string, oldRelease, oldVersion, newRelease, newVersion string) mods.ModDetails {
	return mods.ModDetails{
		ModSummary: mods.ModSummary{
			ID:            modID,
			Name:          "Mod " + modID,
			LatestVersion: newVersion,
		},
		Versions: []mods.ModVersion{
			{ID: oldRelease, Version: oldVersion, FileName: modID + "_" + oldVersion + ".zip", DownloadURL: "https://mods.example/" + modID + "_" + oldVersion + ".zip"},
			{ID: newRelease, Version: newVersion, FileName: modID + "_" + newVersion + ".zip", DownloadURL: "https://mods.example/" + modID + "_" + newVersion + ".zip"},
		},
	}
}

// manyReleaseMod builds catalog details for a mod with count releases named
// r1..rN.
func manyReleaseMod(modID string, count int) mods.ModDetails {
	details := mods.ModDetails{
		ModSummary: mods.ModSummary{ID: modID, Name: "Mod " + modID},
	}
	for index := 1; index <= count; index++ {
		releaseID := fmt.Sprintf("r%d", index)
		version := fmt.Sprintf("%d.0.0", index)
		details.Versions = append(details.Versions, mods.ModVersion{
			ID:           releaseID,
			Version:      version,
			FileName:     modID + "_" + releaseID + ".zip",
			DownloadURL:  "https://mods.example/" + modID + "_" + releaseID + ".zip",
			ReleaseType:  "stable",
			GameVersions: []string{"1.20"},
		})
		details.LatestVersion = version
	}
	return details
}

// updateModTo requests an update of one mod and fails the test on error.
func updateModTo(t *testing.T, fixture testFixture, instanceID, modID, releaseID string) mods.ModUpdateResult {
	t.Helper()
	result, err := fixture.modsCatalog.UpdateInstanceMods(
		context.Background(),
		instanceID,
		[]mods.ModUpdateTarget{{ModID: modID, VersionID: releaseID}},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// listAutomaticSnapshots returns the automatic snapshots of an instance,
// newest first.
func listAutomaticSnapshots(t *testing.T, fixture testFixture, instanceID string) []snapshots.InstanceSnapshot {
	t.Helper()
	listed, err := fixture.service.Snapshots().List(context.Background(), instanceID)
	if err != nil {
		t.Fatal(err)
	}
	var automatic []snapshots.InstanceSnapshot
	for _, snapshot := range listed {
		if snapshot.Type == snapshots.TypeAutomatic {
			automatic = append(automatic, snapshot)
		}
	}
	return automatic
}

// newestAutomaticSnapshotManifest finds the latest automatic snapshot of an
// instance and reads its manifest.
func newestAutomaticSnapshotManifest(t *testing.T, fixture testFixture, instanceID string) snapshots.Manifest {
	t.Helper()
	automatic := listAutomaticSnapshots(t, fixture, instanceID)
	if len(automatic) == 0 {
		t.Fatal("no automatic snapshot was created")
	}
	dir, err := platformsnapshots.New(fixture.root).SnapshotDir(instanceID, automatic[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	return snapshotManifest(t, dir)
}

// installedSource returns the moddb source of the installed mod with the
// given modID, or "" when it is not installed.
func installedSource(t *testing.T, fixture testFixture, instanceID, modID string) string {
	t.Helper()
	mods, err := fixture.store.ListMods(context.Background(), instanceID)
	if err != nil {
		t.Fatal(err)
	}
	for _, mod := range mods {
		if found, _, ok := parseTestModDBSource(mod.Source); ok && found == modID {
			return mod.Source
		}
	}
	return ""
}

func parseTestModDBSource(source string) (string, string, bool) {
	parts := strings.Split(source, ":")
	if len(parts) != 3 || parts[0] != "moddb" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func TestAutomaticSnapshotCreatedBeforeSingleModUpdate(t *testing.T) {
	fixture := newTestFixtureWithMods(t, staticModCatalog{details: twoReleaseMod("100", "1000", "1.0.0", "1001", "2.0.0")})
	instance := createSnapshotTestInstance(t, fixture, "Single update")
	writeTestInstanceFiles(t, instance)
	installManagedMod(t, fixture, instance, "Mod A", "mod-a.zip", "100", "1000", "1.0.0", true)

	result := updateModTo(t, fixture, instance.ID, "100", "1001")
	if result.Updated != 1 {
		t.Fatalf("expected 1 updated mod, got %d", result.Updated)
	}

	// The update itself must have happened.
	if source := installedSource(t, fixture, instance.ID, "100"); source != "moddb:100:1001" {
		t.Fatalf("mod was not updated: installed source %q", source)
	}

	// Exactly one automatic snapshot with the OLD release inside.
	automatic := listAutomaticSnapshots(t, fixture, instance.ID)
	if len(automatic) != 1 {
		t.Fatalf("expected exactly 1 automatic snapshot, got %d", len(automatic))
	}
	manifest := newestAutomaticSnapshotManifest(t, fixture, instance.ID)
	if manifest.Type != snapshots.TypeAutomatic {
		t.Fatalf("unexpected snapshot type %q", manifest.Type)
	}
	if manifest.Reason != snapshots.ReasonBeforeModUpdate {
		t.Fatalf("unexpected snapshot reason %q", manifest.Reason)
	}
	if manifest.Context["affectedMods"] != "1" {
		t.Fatalf("unexpected snapshot context %#v", manifest.Context)
	}
	if len(manifest.Mods) != 1 {
		t.Fatalf("expected 1 mod in the snapshot, got %d", len(manifest.Mods))
	}
	entry := manifest.Mods[0]
	if entry.Source != snapshots.ModSourceModDB || entry.ModID != "100" || entry.ReleaseID != "1000" {
		t.Fatalf("snapshot does not contain the old release: %#v", entry)
	}
	if entry.Version != "1.0.0" {
		t.Fatalf("snapshot records version %q, want the old 1.0.0", entry.Version)
	}
}

func TestBulkUpdateCreatesExactlyOneSnapshot(t *testing.T) {
	ctx := context.Background()
	detailsByID := make(map[string]mods.ModDetails, 20)
	targets := make([]mods.ModUpdateTarget, 0, 20)
	for index := 0; index < 20; index++ {
		modID := fmt.Sprintf("100-%02d", index)
		detailsByID[modID] = twoReleaseMod(modID, "r0", "1.0.0", "r1", "2.0.0")
		targets = append(targets, mods.ModUpdateTarget{ModID: modID, VersionID: "r1"})
	}
	fixture := newTestFixtureWithMods(t, staticModCatalog{detailsByID: detailsByID})
	instance := createSnapshotTestInstance(t, fixture, "Bulk update")
	writeTestInstanceFiles(t, instance)
	for index := 0; index < 20; index++ {
		modID := fmt.Sprintf("100-%02d", index)
		installManagedMod(t, fixture, instance, "Mod "+modID, modID+".zip", modID, "r0", "1.0.0", true)
	}

	// The bulk use case internally drives one mod update per target; it must
	// still produce exactly ONE safety snapshot for the whole operation.
	result, err := fixture.modsCatalog.UpdateInstanceMods(ctx, instance.ID, targets, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated != 20 {
		t.Fatalf("expected 20 updated mods, got %d", result.Updated)
	}
	automatic := listAutomaticSnapshots(t, fixture, instance.ID)
	if len(automatic) != 1 {
		t.Fatalf("bulk update created %d automatic snapshots, want exactly 1", len(automatic))
	}
	manifest := newestAutomaticSnapshotManifest(t, fixture, instance.ID)
	if manifest.Reason != snapshots.ReasonBeforeModUpdate {
		t.Fatalf("unexpected snapshot reason %q", manifest.Reason)
	}
	if manifest.Context["affectedMods"] != "20" {
		t.Fatalf("unexpected snapshot context %#v", manifest.Context)
	}
	if manifest.ModCount != 20 {
		t.Fatalf("snapshot records %d mods, want 20", manifest.ModCount)
	}
	for _, entry := range manifest.Mods {
		if entry.ReleaseID != "r0" {
			t.Fatalf("snapshot does not contain the old release: %#v", entry)
		}
	}
}

func TestUpdateWithNoChangesSkipsSnapshot(t *testing.T) {
	fixture := newTestFixtureWithMods(t, staticModCatalog{details: twoReleaseMod("100", "1000", "1.0.0", "1001", "2.0.0")})
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "No changes")
	writeTestInstanceFiles(t, instance)
	installManagedMod(t, fixture, instance, "Mod A", "mod-a.zip", "100", "1000", "1.0.0", true)

	// The instance is already at the requested release: no update, no snapshot.
	result, err := fixture.modsCatalog.UpdateInstanceMods(
		ctx,
		instance.ID,
		[]mods.ModUpdateTarget{{ModID: "100", VersionID: "1000"}},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated != 0 {
		t.Fatalf("expected no updates, got %d", result.Updated)
	}
	if snapshots, listErr := fixture.service.Snapshots().List(ctx, instance.ID); listErr != nil || len(snapshots) != 0 {
		t.Fatalf("a snapshot was created without changes: %d, %v", len(snapshots), listErr)
	}
}

func TestAutomaticSnapshotCreatedBeforeModRemoval(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Removal")
	writeTestInstanceFiles(t, instance)
	for _, mod := range []struct{ modID, releaseID string }{
		{"100", "1000"}, {"200", "2000"}, {"300", "3000"},
	} {
		installManagedMod(t, fixture, instance, "Mod "+mod.modID, mod.modID+".zip", mod.modID, mod.releaseID, "1.0.0", true)
	}

	installedMods, err := fixture.store.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	var removed mods.InstalledMod
	for _, mod := range installedMods {
		if modID, _, ok := parseTestModDBSource(mod.Source); ok && modID == "200" {
			removed = mod
		}
	}
	if removed.ID == "" {
		t.Fatal("mod to remove was not installed")
	}

	if err := fixture.mods.DeleteMod(ctx, removed.ID, false); err != nil {
		t.Fatal(err)
	}

	// The removal happened; the snapshot keeps the full pre-removal set.
	if source := installedSource(t, fixture, instance.ID, "200"); source != "" {
		t.Fatalf("mod 200 was not removed: %q", source)
	}
	manifest := newestAutomaticSnapshotManifest(t, fixture, instance.ID)
	if manifest.Reason != snapshots.ReasonBeforeModRemoval {
		t.Fatalf("unexpected snapshot reason %q", manifest.Reason)
	}
	if manifest.ModCount != 3 || len(manifest.Mods) != 3 {
		t.Fatalf("snapshot must contain all three mods, got %d entries", len(manifest.Mods))
	}
	byModID := make(map[string]snapshots.Mod, len(manifest.Mods))
	for _, entry := range manifest.Mods {
		byModID[entry.ModID] = entry
	}
	for _, modID := range []string{"100", "200", "300"} {
		if _, ok := byModID[modID]; !ok {
			t.Fatalf("snapshot misses installed mod %s", modID)
		}
	}
}

func TestBulkRemovalCreatesExactlyOneSnapshot(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Bulk removal")
	writeTestInstanceFiles(t, instance)
	for _, mod := range []struct{ modID, releaseID string }{
		{"100", "1000"}, {"200", "2000"}, {"300", "3000"},
	} {
		installManagedMod(t, fixture, instance, "Mod "+mod.modID, mod.modID+".zip", mod.modID, mod.releaseID, "1.0.0", true)
	}
	mods, err := fixture.store.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(mods))
	for _, mod := range mods {
		ids = append(ids, mod.ID)
	}

	if err := fixture.mods.RemoveMods(ctx, instance.ID, ids, false); err != nil {
		t.Fatal(err)
	}
	remaining, err := fixture.store.ListMods(ctx, instance.ID)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("mods were not removed: %d, %v", len(remaining), err)
	}
	automatic := listAutomaticSnapshots(t, fixture, instance.ID)
	if len(automatic) != 1 {
		t.Fatalf("bulk removal created %d automatic snapshots, want exactly 1", len(automatic))
	}
	manifest := newestAutomaticSnapshotManifest(t, fixture, instance.ID)
	if manifest.Reason != snapshots.ReasonBeforeModRemoval {
		t.Fatalf("unexpected snapshot reason %q", manifest.Reason)
	}
	if manifest.ModCount != 3 {
		t.Fatalf("snapshot records %d mods, want 3", manifest.ModCount)
	}
}

func TestAutomaticSnapshotCreatedBeforeGameVersionChange(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Version switch")
	writeTestInstanceFiles(t, instance)
	saveAdditionalGameVersion(t, fixture, "1.21.6")

	instance.GameVersionID = "1.21.6"
	if _, err := fixture.service.InstanceUpdater().Update(ctx, instance); err != nil {
		t.Fatal(err)
	}

	stored, err := fixture.store.GetInstance(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.GameVersionID != "1.21.6" {
		t.Fatalf("game version was not changed: %q", stored.GameVersionID)
	}

	manifest := newestAutomaticSnapshotManifest(t, fixture, instance.ID)
	if manifest.Reason != snapshots.ReasonBeforeGameVersionChange {
		t.Fatalf("unexpected snapshot reason %q", manifest.Reason)
	}
	if manifest.GameVersion != "1.20" {
		t.Fatalf("snapshot records the new game version %q, want the old 1.20", manifest.GameVersion)
	}
	if manifest.Context["fromGameVersion"] != "1.20" || manifest.Context["toGameVersion"] != "1.21.6" {
		t.Fatalf("unexpected version context %#v", manifest.Context)
	}
}

// saveAdditionalGameVersion registers another installed game version.
func saveAdditionalGameVersion(t *testing.T, fixture testFixture, id string) {
	t.Helper()
	if err := fixture.store.SaveVersion(context.Background(), versions.GameVersion{
		ID:              id,
		Name:            id,
		Channel:         "stable",
		Platform:        "linux",
		Architecture:    "amd64",
		InstallationDir: filepath.Join(fixture.root, "versions", id),
		ExecutablePath:  filepath.Join(fixture.root, "versions", id, "Vintagestory"),
		Status:          "installed",
		InstalledAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticSnapshotFailureBlocksDestructiveOperations(t *testing.T) {
	t.Run("mod update", func(t *testing.T) {
		fixture := newTestFixtureWithMods(t, staticModCatalog{details: twoReleaseMod("100", "1000", "1.0.0", "1001", "2.0.0")})
		ctx := context.Background()
		instance := createSnapshotTestInstance(t, fixture, "Blocked update")
		writeTestInstanceFiles(t, instance)
		installManagedMod(t, fixture, instance, "Mod A", "mod-a.zip", "100", "1000", "1.0.0", true)
		fixture.setDiskSpace(fixedDiskSpace(0))

		_, err := fixture.modsCatalog.UpdateInstanceMods(
			ctx,
			instance.ID,
			[]mods.ModUpdateTarget{{ModID: "100", VersionID: "1001"}},
			false,
		)
		if code := appErrorCode(t, err); code != errs.ErrInsufficientSpace {
			t.Fatalf("expected INSUFFICIENT_DISK_SPACE, got %s", code)
		}
		if source := installedSource(t, fixture, instance.ID, "100"); source != "moddb:100:1000" {
			t.Fatalf("mod was modified despite the failed snapshot: %q", source)
		}
		if snapshots, listErr := fixture.service.Snapshots().List(ctx, instance.ID); listErr != nil || len(snapshots) != 0 {
			t.Fatalf("a snapshot appeared despite the failure: %d, %v", len(snapshots), listErr)
		}
	})

	t.Run("mod removal", func(t *testing.T) {
		fixture := newTestFixtureWithMods(t, nil)
		ctx := context.Background()
		instance := createSnapshotTestInstance(t, fixture, "Blocked removal")
		writeTestInstanceFiles(t, instance)
		installManagedMod(t, fixture, instance, "Mod A", "mod-a.zip", "100", "1000", "1.0.0", true)
		fixture.setDiskSpace(fixedDiskSpace(0))

		mods, err := fixture.store.ListMods(ctx, instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		err = fixture.mods.DeleteMod(ctx, mods[0].ID, false)
		if code := appErrorCode(t, err); code != errs.ErrInsufficientSpace {
			t.Fatalf("expected INSUFFICIENT_DISK_SPACE, got %s", code)
		}
		if _, statErr := os.Stat(filepath.Join(instance.Directory, "Mods", "mod-a.zip")); statErr != nil {
			t.Fatalf("mod file was removed despite the failed snapshot: %v", statErr)
		}
	})

	t.Run("game version change", func(t *testing.T) {
		fixture := newTestFixture(t)
		ctx := context.Background()
		instance := createSnapshotTestInstance(t, fixture, "Blocked version")
		writeTestInstanceFiles(t, instance)
		saveAdditionalGameVersion(t, fixture, "1.21.6")
		fixture.setDiskSpace(fixedDiskSpace(0))

		instance.GameVersionID = "1.21.6"
		if _, err := fixture.service.InstanceUpdater().Update(ctx, instance); err == nil {
			t.Fatal("expected the version change to fail")
		}
		stored, err := fixture.store.GetInstance(ctx, instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if stored.GameVersionID != "1.20" {
			t.Fatalf("game version was changed despite the failed snapshot: %q", stored.GameVersionID)
		}
	})
}

func TestDisabledAutomaticSnapshotsSkipCreation(t *testing.T) {
	fixture := newTestFixtureWithMods(t, staticModCatalog{details: twoReleaseMod("100", "1000", "1.0.0", "1001", "2.0.0")})
	ctx := context.Background()
	settings, err := fixture.settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.AutomaticSafetySnapshots = false
	if _, err := fixture.updates.Update(ctx, settings); err != nil {
		t.Fatal(err)
	}

	instance := createSnapshotTestInstance(t, fixture, "Disabled backups")
	writeTestInstanceFiles(t, instance)
	installManagedMod(t, fixture, instance, "Mod A", "mod-a.zip", "100", "1000", "1.0.0", true)

	result := updateModTo(t, fixture, instance.ID, "100", "1001")
	if result.Updated != 1 {
		t.Fatalf("expected 1 updated mod, got %d", result.Updated)
	}
	if snapshots, listErr := fixture.service.Snapshots().List(ctx, instance.ID); listErr != nil || len(snapshots) != 0 {
		t.Fatalf("disabled setting still created snapshots: %d, %v", len(snapshots), listErr)
	}

	mods, err := fixture.store.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.mods.DeleteMod(ctx, mods[0].ID, false); err != nil {
		t.Fatal(err)
	}
	if remaining, listErr := fixture.store.ListMods(ctx, instance.ID); listErr != nil || len(remaining) != 0 {
		t.Fatalf("disabled setting blocked removal: %d, %v", len(remaining), listErr)
	}
	if snapshots, listErr := fixture.service.Snapshots().List(ctx, instance.ID); listErr != nil || len(snapshots) != 0 {
		t.Fatalf("disabled setting still created snapshots: %d, %v", len(snapshots), listErr)
	}

	saveAdditionalGameVersion(t, fixture, "1.21.6")
	instance.GameVersionID = "1.21.6"
	if _, err := fixture.service.InstanceUpdater().Update(ctx, instance); err != nil {
		t.Fatal(err)
	}
	if snapshots, listErr := fixture.service.Snapshots().List(ctx, instance.ID); listErr != nil || len(snapshots) != 0 {
		t.Fatalf("disabled setting still created snapshots: %d, %v", len(snapshots), listErr)
	}

	// Manual snapshots stay available while the automatic ones are off.
	operation, err := fixture.service.Snapshots().Create(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if operation.Status != "completed" {
		t.Fatalf("manual snapshot failed with status %q", operation.Status)
	}
}

// blockingDownloader blocks every download until released, mimicking a slow
// mod download.
type blockingDownloader struct {
	release chan struct{}
}

func (downloader blockingDownloader) Download(
	ctx context.Context,
	request downloads.Request,
	progress chan<- downloads.Progress,
) error {
	select {
	case <-downloader.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := os.MkdirAll(filepath.Dir(request.DestinationPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(request.DestinationPath, []byte("package"), 0o644); err != nil {
		return err
	}
	progress <- downloads.Progress{DownloadedBytes: 7, TotalBytes: 7}
	return nil
}

func (blockingDownloader) ContentLength(context.Context, string) (int64, error) {
	return 0, nil
}

func TestConcurrentDestructiveOperationsRejectedPerInstance(t *testing.T) {
	fixture := newTestFixtureWithMods(t, staticModCatalog{details: twoReleaseMod("100", "1000", "1.0.0", "1001", "2.0.0")})
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Concurrent")
	writeTestInstanceFiles(t, instance)
	installManagedMod(t, fixture, instance, "Mod A", "mod-a.zip", "100", "1000", "1.0.0", true)
	release := make(chan struct{})
	fixture.downloader.Set(blockingDownloader{release: release})

	updateDone := make(chan error, 1)
	go func() {
		_, err := fixture.modsCatalog.UpdateInstanceMods(
			context.Background(),
			instance.ID,
			[]mods.ModUpdateTarget{{ModID: "100", VersionID: "1001"}},
			false,
		)
		updateDone <- err
	}()

	// Wait until the safety snapshot exists, i.e. the update transaction is
	// holding the per-instance mutation slot.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if len(listAutomaticSnapshots(t, fixture, instance.ID)) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the update never created its safety snapshot")
		}
		time.Sleep(5 * time.Millisecond)
	}

	mods, err := fixture.store.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	err = fixture.mods.DeleteMod(ctx, mods[0].ID, false)
	if code := appErrorCode(t, err); code != snapshots.ErrSnapshotInProgress {
		t.Fatalf("expected SNAPSHOT_IN_PROGRESS for a concurrent removal, got %s", code)
	}
	if _, err := fixture.service.Snapshots().Create(ctx, instance.ID); err == nil {
		t.Fatal("a manual snapshot must not run concurrently with a mutation")
	}

	close(release)
	if err := <-updateDone; err != nil {
		t.Fatal(err)
	}
	if source := installedSource(t, fixture, instance.ID, "100"); source != "moddb:100:1001" {
		t.Fatalf("update did not finish after the lock was released: %q", source)
	}

	// After the transaction finished, a new destructive operation works again.
	// The update replaced the old mod record, so re-fetch the current one.
	mods, err = fixture.store.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.mods.DeleteMod(ctx, mods[0].ID, false); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticRetentionKeepsLatestTen(t *testing.T) {
	fixture := newTestFixtureWithMods(t, staticModCatalog{details: manyReleaseMod("100", 12)})
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Retention")
	writeTestInstanceFiles(t, instance)
	installManagedMod(t, fixture, instance, "Mod A", "mod-a.zip", "100", "r1", "1.0.0", true)

	// Update the same mod eleven times (r1 -> r2 -> ... -> r12); every update
	// creates one automatic snapshot and retention trims the oldest ones.
	firstSnapshotID := ""
	for index := 2; index <= 12; index++ {
		updateModTo(t, fixture, instance.ID, "100", fmt.Sprintf("r%d", index))
		if index == 2 {
			automatic := listAutomaticSnapshots(t, fixture, instance.ID)
			if len(automatic) == 0 {
				t.Fatal("update did not create a snapshot")
			}
			firstSnapshotID = automatic[0].ID
		}
	}

	listed, err := fixture.service.Snapshots().List(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != automaticSnapshotRetentionCount {
		t.Fatalf("expected %d snapshots after retention, got %d", automaticSnapshotRetentionCount, len(listed))
	}
	for _, snapshot := range listed {
		if snapshot.Type != snapshots.TypeAutomatic {
			t.Fatalf("retention kept a non-automatic snapshot: %#v", snapshot)
		}
		if snapshot.ID == firstSnapshotID {
			t.Fatal("the oldest automatic snapshot was not removed by retention")
		}
	}
	if source := installedSource(t, fixture, instance.ID, "100"); source != "moddb:100:r12" {
		t.Fatalf("the instance was not updated to the newest release: %q", source)
	}
}

func TestAutomaticRetentionNeverRemovesManualSnapshots(t *testing.T) {
	fixture := newTestFixtureWithMods(t, staticModCatalog{details: manyReleaseMod("100", 12)})
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Mixed retention")
	writeTestInstanceFiles(t, instance)
	installManagedMod(t, fixture, instance, "Mod A", "mod-a.zip", "100", "r1", "1.0.0", true)

	for index := 0; index < 5; index++ {
		if _, err := fixture.service.Snapshots().Create(ctx, instance.ID); err != nil {
			t.Fatal(err)
		}
	}
	for index := 2; index <= 12; index++ {
		updateModTo(t, fixture, instance.ID, "100", fmt.Sprintf("r%d", index))
	}

	listed, err := fixture.service.Snapshots().List(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	manual := 0
	automatic := 0
	for _, snapshot := range listed {
		switch snapshot.Type {
		case snapshots.TypeManual:
			manual++
		case snapshots.TypeAutomatic:
			automatic++
		default:
			t.Fatalf("unexpected snapshot type %q", snapshot.Type)
		}
	}
	if manual != 5 {
		t.Fatalf("retention removed manual snapshots: %d remaining", manual)
	}
	if automatic != automaticSnapshotRetentionCount {
		t.Fatalf("expected %d automatic snapshots, got %d", automaticSnapshotRetentionCount, automatic)
	}
}

func TestAutomaticSnapshotRestoresThroughExistingSystem(t *testing.T) {
	fixture := newTestFixtureWithMods(t, staticModCatalog{details: twoReleaseMod("100", "1000", "1.0.0", "1001", "2.0.0")})
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Restorable automatic")
	writeTestInstanceFiles(t, instance)
	installManagedMod(t, fixture, instance, "Mod A", "mod-a.zip", "100", "1000", "1.0.0", true)

	// The automatic snapshot is created by the update; it must restore exactly
	// like a manual one through the shared restore pipeline.
	updateModTo(t, fixture, instance.ID, "100", "1001")
	automatic := listAutomaticSnapshots(t, fixture, instance.ID)
	if len(automatic) != 1 {
		t.Fatalf("expected one automatic snapshot, got %d", len(automatic))
	}
	if err := os.WriteFile(filepath.Join(instance.Directory, "file.txt"), []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := fixture.service.Snapshots().Restore(ctx, instance.ID, automatic[0].ID); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(instance.Directory, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "before" {
		t.Fatalf("automatic snapshot did not restore the instance data: %q", contents)
	}
	if source := installedSource(t, fixture, instance.ID, "100"); source != "moddb:100:1000" {
		t.Fatalf("automatic snapshot did not restore the old release: %q", source)
	}
}

func TestAutomaticSnapshotRejectedWhileGameRunning(t *testing.T) {
	fixture := newTestFixtureWithMods(t, staticModCatalog{details: twoReleaseMod("100", "1000", "1.0.0", "1001", "2.0.0")})
	ctx := context.Background()
	instance := createSnapshotTestInstance(t, fixture, "Running")
	writeTestInstanceFiles(t, instance)
	installManagedMod(t, fixture, instance, "Mod A", "mod-a.zip", "100", "1000", "1.0.0", true)

	if _, err := fixture.launching.Launch(ctx, instance.ID, nil); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fixture.launching.Stop(ctx, instance.ID, true) }()

	_, err := fixture.modsCatalog.UpdateInstanceMods(
		ctx,
		instance.ID,
		[]mods.ModUpdateTarget{{ModID: "100", VersionID: "1001"}},
		false,
	)
	if code := appErrorCode(t, err); code != instances.ErrInstanceRunning {
		t.Fatalf("expected INSTANCE_ALREADY_RUNNING, got %s", code)
	}
	if source := installedSource(t, fixture, instance.ID, "100"); source != "moddb:100:1000" {
		t.Fatalf("mod was modified while the game was running: %q", source)
	}
	if snapshots, listErr := fixture.service.Snapshots().List(ctx, instance.ID); listErr != nil || len(snapshots) != 0 {
		t.Fatalf("a snapshot was created while the game was running: %d, %v", len(snapshots), listErr)
	}
}
