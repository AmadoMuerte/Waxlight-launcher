package snapshots

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/operations"
	settingscore "github.com/AmadoMuerte/Waxlight-launcher/internal/settings"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/versions"
)

// fakeRepository persists operations in memory.
type fakeRepository struct {
	mu   sync.Mutex
	rows map[string]operations.Operation
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{rows: make(map[string]operations.Operation)}
}

func (repository *fakeRepository) ListOperations(_ context.Context, _ int) ([]operations.Operation, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	result := make([]operations.Operation, 0, len(repository.rows))
	for _, operation := range repository.rows {
		result = append(result, operation)
	}
	return result, nil
}

func (repository *fakeRepository) SaveOperation(_ context.Context, operation operations.Operation) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.rows[operation.ID] = operation
	return nil
}

func (repository *fakeRepository) ReconcileInterruptedOperations(context.Context, time.Time, string, string) (int64, error) {
	return 0, nil
}

func (repository *fakeRepository) DeleteFinishedOperation(_ context.Context, _ string) error {
	return nil
}

func (repository *fakeRepository) ClearFinishedOperations(context.Context) (int64, error) {
	return 0, nil
}

type workerOwner struct{}

func (workerOwner) Go(func(context.Context)) bool { return true }

// fakeSettings serves the automatic safety-snapshot toggle.
type fakeSettings struct {
	settings settingscore.Settings
}

func (settings fakeSettings) Get(context.Context) (settingscore.Settings, error) {
	return settings.settings, nil
}

// fakeMods serves the mod-store port.
type fakeMods struct {
	installed []InstalledMod
}

func (mods *fakeMods) ListMods(context.Context, string) ([]InstalledMod, error) {
	return mods.installed, nil
}

func (mods *fakeMods) SaveMod(context.Context, InstalledMod) error { return nil }
func (mods *fakeMods) DeleteMod(context.Context, string) error     { return nil }

// fakeCatalog downloads catalog releases for restore.
type fakeCatalog struct {
	releases map[string]DownloadedRelease
}

func (catalog *fakeCatalog) GetDownloadedMod(context.Context, string, string) (DownloadedRelease, error) {
	return DownloadedRelease{}, errs.ErrNotFound
}

func (catalog *fakeCatalog) DownloadRelease(_ context.Context, modID, releaseID string) (DownloadedRelease, error) {
	release, ok := catalog.releases[modID+":"+releaseID]
	if !ok {
		return DownloadedRelease{}, errs.NewError(errs.ErrModVersionNotFound, "release missing")
	}
	return release, nil
}

type fakeArchiveInfo struct{}

func (fakeArchiveInfo) ReadArchiveInfo(string) (ArchiveInfo, error) {
	return ArchiveInfo{}, nil
}

type fakeVersion struct {
	names map[string]string
}

func (reader fakeVersion) Get(_ context.Context, id string) (versions.GameVersion, error) {
	name, ok := reader.names[id]
	if !ok {
		return versions.GameVersion{}, errs.ErrNotFound
	}
	return versions.GameVersion{ID: id, Name: name}, nil
}

type fakeLock struct{}

func (fakeLock) Guard(string, string, string) (func(), error) { return func() {}, nil }
func (fakeLock) Lock(string, string) (func(), error)          { return func() {}, nil }
func (fakeLock) Running(string) bool                          { return false }
func (fakeLock) Busy(string) bool                             { return false }

type fakeSlot struct{}

func (fakeSlot) TryAcquire(string, string) (func(), string) { return func() {}, "" }
func (fakeSlot) Set(string, string) func()                  { return func() {} }
func (fakeSlot) IsBusy(string) bool                         { return false }

type fakeGate struct{}

func (fakeGate) Begin() error { return nil }
func (fakeGate) End()         {}

type fakeDiskSpace int64

func (space fakeDiskSpace) Available(string) (int64, error) { return int64(space), nil }

// fakeLKG records snapshot-reference operations.
type fakeLKG struct {
	mu        sync.Mutex
	cleared   []string
	protected string
}

func (lkg *fakeLKG) ClearSnapshotReference(_ context.Context, _, snapshotID string) {
	lkg.mu.Lock()
	defer lkg.mu.Unlock()
	lkg.cleared = append(lkg.cleared, snapshotID)
}

func (lkg *fakeLKG) ProtectedSnapshotID(context.Context, string) string {
	lkg.mu.Lock()
	defer lkg.mu.Unlock()
	return lkg.protected
}

func sanitizeClientSettings(contents []byte) ([]byte, error) {
	return []byte(`{"removedCredentials":true}`), nil
}

var snapshotCounter = 0

func newTestSnapshotID() string {
	snapshotCounter++
	return fmt.Sprintf("snap-%08x", snapshotCounter)
}

// fakeStorage is an in-memory Storage double with the same layout contract as
// the platform adapter: manifest.json plus a data/ directory per snapshot.
type fakeStorage struct {
	root string
	mu   sync.Mutex
}

func (storage *fakeStorage) List(_ context.Context, instanceID string) ([]InstanceSnapshot, error) {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	instanceDir := filepath.Join(storage.root, instanceID)
	entries, err := os.ReadDir(instanceDir)
	if errors.Is(err, os.ErrNotExist) {
		return []InstanceSnapshot{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := []InstanceSnapshot{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		manifest, readErr := storage.readManifestLocked(instanceDir, entry.Name())
		if readErr != nil || manifest.ID != entry.Name() {
			continue
		}
		result = append(result, InstanceSnapshot{
			ID:           manifest.ID,
			InstanceID:   manifest.InstanceID,
			InstanceName: manifest.InstanceName,
			Type:         manifest.Type,
			Reason:       manifest.Reason,
			Context:      manifest.Context,
			GameVersion:  manifest.GameVersion,
			CreatedAt:    manifest.CreatedAt,
			SizeBytes:    manifest.SizeBytes,
			ModCount:     manifest.ModCount,
			WorldCount:   manifest.WorldCount,
		})
	}
	return result, nil
}

func (storage *fakeStorage) ReadManifest(snapshotDir string) (Manifest, error) {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	return storage.readManifestLocked(filepath.Dir(snapshotDir), filepath.Base(snapshotDir))
}

func (storage *fakeStorage) readManifestLocked(instanceDir, snapshotID string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(instanceDir, snapshotID, "manifest.json"))
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (storage *fakeStorage) WriteManifest(snapshotDir string, manifest Manifest) error {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), append(encoded, '\n'), 0o600)
}

func (storage *fakeStorage) SnapshotDir(instanceID, snapshotID string) (string, error) {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	if instanceID == "" || snapshotID == "" || strings.ContainsAny(instanceID+snapshotID, `/\.`) {
		return "", errs.NewError(errs.ErrValidation, "Invalid snapshot identifier")
	}
	return filepath.Join(storage.root, instanceID, snapshotID), nil
}

func (storage *fakeStorage) DataDir(snapshotDir string) string {
	return filepath.Join(snapshotDir, "data")
}

func (storage *fakeStorage) TempDir(instanceID string) (string, error) {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	dir := filepath.Join(storage.root, instanceID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	temporary, err := os.MkdirTemp(dir, ".tmp-")
	if err != nil {
		return "", err
	}
	return temporary, nil
}

func (storage *fakeStorage) Remove(instanceID, snapshotID string) error {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	if instanceID == "" || snapshotID == "" || strings.ContainsAny(instanceID+snapshotID, `/\.`) {
		return errs.NewError(errs.ErrValidation, "Invalid snapshot identifier")
	}
	dir := filepath.Join(storage.root, instanceID, snapshotID)
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		return errs.NewError(ErrSnapshotNotFound, "Snapshot not found")
	}
	return os.RemoveAll(dir)
}

func (storage *fakeStorage) Size(snapshotDir string) (int64, error) {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	var total int64
	err := filepath.Walk(storage.DataDir(snapshotDir), func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	return total, err
}

// testService wires the snapshot service over the fake storage and the real
// operations manager with in-memory persistence.
func testService(t *testing.T, mods *fakeMods, catalog *fakeCatalog, settings settingscore.Settings) (*Service, *fakeLKG, *fakeStorage, string) {
	t.Helper()
	root := t.TempDir()
	storage := &fakeStorage{root: filepath.Join(root, "backups")}
	operationsManager := operations.NewManager(newFakeRepository(), workerOwner{}, nil)
	lkg := &fakeLKG{}
	service := NewService(
		storage,
		InstanceReaderFunc(func(_ context.Context, instanceID string) (InstanceRef, error) {
			return InstanceRef{
				ID:            instanceID,
				Name:          "Survival",
				Directory:     filepath.Join(root, "instances", instanceID),
				GameVersionID: "1.20",
			}, nil
		}),
		fakeVersion{names: map[string]string{"1.20": "1.20"}},
		mods,
		catalog,
		fakeArchiveInfo{},
		fakeSettings{settings: settings},
		operationsManager,
		fakeGate{},
		fakeSlot{},
		fakeLock{},
		fakeDiskSpace(1<<40),
		func(ctx context.Context, path string) (int64, error) {
			var total int64
			err := filepath.Walk(path, func(_ string, info os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if !info.IsDir() {
					total += info.Size()
				}
				return nil
			})
			return total, err
		},
		sanitizeClientSettings,
		func(string) error { return nil },
		func(string) error { return nil },
		func(path string) error { return os.RemoveAll(path) },
		lkg,
		root,
		time.Now,
		newTestSnapshotID,
	)
	return service, lkg, storage, root
}

// createTestInstanceDir builds a standard instance layout under the
// test-service data root, matching the fake instance reader's paths.
func createTestInstanceDir(t *testing.T, root, instanceID string) string {
	t.Helper()
	dir := filepath.Join(root, "instances", instanceID)
	for _, sub := range []string{"", "Mods", "ModsDisabled", "Logs", "SaveGame/world1"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "clientsettings.json"), []byte(`{"sessionkey":"secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".waxlight-instance"), []byte(instanceID), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCreateSnapshotSanitizesCredentialsAndWritesManifest(t *testing.T) {
	mods := &fakeMods{}
	service, _, _, root := testService(t, mods, &fakeCatalog{}, settingscore.Defaults())
	instanceID := "instance-1"
	createTestInstanceDir(t, root, instanceID)

	operation, err := service.Create(context.Background(), instanceID)
	if err != nil {
		t.Fatal(err)
	}

	snapshots, err := service.List(context.Background(), instanceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 || snapshots[0].ID != operation.ID {
		t.Fatalf("unexpected snapshot listing: %#v", snapshots)
	}
	if snapshots[0].Type != TypeManual {
		t.Fatalf("manual snapshot has type %q", snapshots[0].Type)
	}
	manifest, err := service.ReadManifest(context.Background(), instanceID, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.GameVersion != "1.20" || manifest.WorldCount != 1 {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	dir, err := service.storage.SnapshotDir(instanceID, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(service.storage.DataDir(dir), "clientsettings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "sessionkey") {
		t.Fatalf("snapshot contains credentials: %s", data)
	}
}

func TestCreateSnapshotRecordsManagedModReleases(t *testing.T) {
	mods := &fakeMods{installed: []InstalledMod{
		{Name: "Fancy", Version: "1.0.0", FileName: "fancy.zip", FilePath: "/instances/i1/Mods/fancy.zip", Enabled: true, Source: "moddb:100:1000"},
		{Name: "Local", Version: "2.0.0", FileName: "local.zip", FilePath: "/instances/i1/Mods/local.zip", Enabled: true, Source: "local"},
	}}
	service, _, _, root := testService(t, mods, &fakeCatalog{}, settingscore.Defaults())
	instanceID := "instance-1"
	createTestInstanceDir(t, root, instanceID)

	operation, err := service.Create(context.Background(), instanceID)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := service.ReadManifest(context.Background(), instanceID, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ModCount != 2 || len(manifest.Mods) != 2 {
		t.Fatalf("unexpected mod count: %#v", manifest)
	}
	if manifest.Mods[0].Source != ModSourceModDB || manifest.Mods[0].ModID != "100" || manifest.Mods[0].ReleaseID != "1000" {
		t.Fatalf("managed mod identity lost: %#v", manifest.Mods[0])
	}
	if manifest.Mods[1].Source != ModSourceUnknown {
		t.Fatalf("local mod must be recorded as unknown: %#v", manifest.Mods[1])
	}
}

func TestCreateSafetySnapshotSkipsWhenDisabled(t *testing.T) {
	mods := &fakeMods{}
	service, _, _, root := testService(t, mods, &fakeCatalog{}, settingscore.Settings{AutomaticSafetySnapshots: false})
	instanceID := "instance-1"
	createTestInstanceDir(t, root, instanceID)

	operation, err := service.CreateSafety(context.Background(), instanceID, ReasonBeforeModRemoval, nil)
	if err != nil {
		t.Fatal(err)
	}
	if operation.ID != "" {
		t.Fatal("expected a zero operation when safety snapshots are disabled")
	}
	snapshots, err := service.List(context.Background(), instanceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("no snapshot must be created, got %d", len(snapshots))
	}
}

func TestCreateSafetySnapshotRecordsReasonAndContext(t *testing.T) {
	mods := &fakeMods{}
	service, _, _, root := testService(t, mods, &fakeCatalog{}, settingscore.Defaults())
	instanceID := "instance-1"
	createTestInstanceDir(t, root, instanceID)

	operation, err := service.CreateSafety(context.Background(), instanceID, ReasonBeforeGameVersionChange, map[string]string{"fromGameVersion": "1.19", "toGameVersion": "1.20"})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := service.ReadManifest(context.Background(), instanceID, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Type != TypeAutomatic || manifest.Reason != ReasonBeforeGameVersionChange {
		t.Fatalf("unexpected automatic manifest: %#v", manifest)
	}
	if manifest.Context["fromGameVersion"] != "1.19" || manifest.Context["toGameVersion"] != "1.20" {
		t.Fatalf("snapshot context lost: %#v", manifest.Context)
	}
}

func TestAutomaticRetentionKeepsNewestAndProtectedSnapshot(t *testing.T) {
	mods := &fakeMods{}
	service, lkg, _, root := testService(t, mods, &fakeCatalog{}, settingscore.Defaults())
	instanceID := "instance-1"
	createTestInstanceDir(t, root, instanceID)

	// Create retentionLimit+2 automatic snapshots; the second one is protected.
	var protectedID string
	for index := 0; index <= automaticRetentionCount+1; index++ {
		operation, err := service.CreateSafety(context.Background(), instanceID, ReasonBeforeModRemoval, nil)
		if err != nil {
			t.Fatal(err)
		}
		if index == 1 {
			protectedID = operation.ID
		}
	}
	lkg.mu.Lock()
	lkg.protected = protectedID
	lkg.mu.Unlock()

	// One more creation triggers retention.
	if _, err := service.CreateSafety(context.Background(), instanceID, ReasonBeforeModRemoval, nil); err != nil {
		t.Fatal(err)
	}

	snapshots, err := service.List(context.Background(), instanceID)
	if err != nil {
		t.Fatal(err)
	}
	automatic := 0
	keptProtected := false
	for _, snapshot := range snapshots {
		if snapshot.Type == TypeAutomatic {
			automatic++
			if snapshot.ID == protectedID {
				keptProtected = true
			}
		}
	}
	if automatic != automaticRetentionCount {
		t.Fatalf("expected %d automatic snapshots after retention, got %d", automaticRetentionCount, automatic)
	}
	if !keptProtected {
		t.Fatal("retention removed the protected last known good snapshot")
	}
}

func TestDeleteClearsLastKnownGoodReference(t *testing.T) {
	mods := &fakeMods{}
	service, lkg, _, root := testService(t, mods, &fakeCatalog{}, settingscore.Defaults())
	instanceID := "instance-1"
	createTestInstanceDir(t, root, instanceID)

	operation, err := service.Create(context.Background(), instanceID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(context.Background(), instanceID, operation.ID); err != nil {
		t.Fatal(err)
	}
	lkg.mu.Lock()
	defer lkg.mu.Unlock()
	if len(lkg.cleared) != 1 || lkg.cleared[0] != operation.ID {
		t.Fatalf("snapshot reference was not cleared: %#v", lkg.cleared)
	}
	snapshots, err := service.List(context.Background(), instanceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("deleted snapshot is still listed: %d", len(snapshots))
	}
}

func TestRestoreRejectsUnknownModsBeforeStaging(t *testing.T) {
	mods := &fakeMods{}
	service, _, storage, root := testService(t, mods, &fakeCatalog{}, settingscore.Defaults())
	instanceID := "instance-1"
	dir := createTestInstanceDir(t, root, instanceID)

	operation, err := service.Create(context.Background(), instanceID)
	if err != nil {
		t.Fatal(err)
	}
	// Rewrite the manifest with an un-restorable mod.
	manifest, err := service.ReadManifest(context.Background(), instanceID, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Mods = []Mod{{Source: ModSourceUnknown, Identifier: "hand", Version: "1.0.0"}}
	if err := service.storage.WriteManifest(mustSnapshotDir(t, service, instanceID, operation.ID), manifest); err != nil {
		t.Fatal(err)
	}

	err = service.Restore(context.Background(), instanceID, operation.ID)
	if err == nil {
		t.Fatal("restore must reject a snapshot with un-restorable mods")
	}
	// The instance must be untouched.
	if contents, readErr := os.ReadFile(filepath.Join(dir, "clientsettings.json")); readErr != nil || !strings.Contains(string(contents), "sessionkey") {
		t.Fatalf("instance was touched by the rejected restore: %q, %v", contents, readErr)
	}
	// The snapshot itself must still exist.
	if _, readErr := storage.ReadManifest(mustSnapshotDir(t, service, instanceID, operation.ID)); readErr != nil {
		t.Fatalf("rejected restore removed the snapshot: %v", readErr)
	}
}

func TestRestoreDownloadFailsWhenReleaseMissing(t *testing.T) {
	mods := &fakeMods{}
	service, _, _, root := testService(t, mods, &fakeCatalog{}, settingscore.Defaults())
	instanceID := "instance-1"
	dir := createTestInstanceDir(t, root, instanceID)

	operation, err := service.Create(context.Background(), instanceID)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := service.ReadManifest(context.Background(), instanceID, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Mods = []Mod{{Source: ModSourceModDB, ModID: "100", ReleaseID: "missing", Identifier: "fancy", Version: "1.0.0"}}
	if err := service.storage.WriteManifest(mustSnapshotDir(t, service, instanceID, operation.ID), manifest); err != nil {
		t.Fatal(err)
	}

	err = service.Restore(context.Background(), instanceID, operation.ID)
	if err == nil {
		t.Fatal("restore must fail when the managed release is missing")
	}
	var appErr *errs.AppError
	if !errors.As(err, &appErr) || appErr.Code != ErrSnapshotInvalid {
		t.Fatalf("expected SNAPSHOT_INVALID, got %v", err)
	}
	if _, statErr := os.Stat(dir); statErr != nil {
		t.Fatalf("instance directory missing after failed restore: %v", statErr)
	}
}

func TestIsRestorableValidatesManifest(t *testing.T) {
	mods := &fakeMods{}
	service, _, _, root := testService(t, mods, &fakeCatalog{}, settingscore.Defaults())
	instanceID := "instance-1"
	createTestInstanceDir(t, root, instanceID)

	operation, err := service.Create(context.Background(), instanceID)
	if err != nil {
		t.Fatal(err)
	}
	if !service.IsRestorable(context.Background(), instanceID, operation.ID) {
		t.Fatal("a manual snapshot without mods must be restorable")
	}
	if service.IsRestorable(context.Background(), instanceID, "missing") {
		t.Fatal("a missing snapshot must not be restorable")
	}
}

func TestManifestJSONRoundTripPreservesContract(t *testing.T) {
	manifest := Manifest{
		FormatVersion: FormatVersion,
		ID:            "snap",
		InstanceID:    "inst",
		InstanceName:  "name",
		CreatedAt:     time.Now().UTC(),
		Type:          TypeAutomatic,
		Reason:        ReasonBeforeModUpdate,
		Context:       map[string]string{"k": "v"},
		GameVersion:   "1.20",
		SizeBytes:     10,
		ModCount:      1,
		WorldCount:    2,
		Mods:          []Mod{{Source: ModSourceModDB, ModID: "1", ReleaseID: "2", Enabled: true}},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["formatVersion"] != float64(FormatVersion) || parsed["type"] != "automatic" || parsed["reason"] != "before_mod_update" {
		t.Fatalf("manifest JSON contract changed: %s", data)
	}
}

func mustSnapshotDir(t *testing.T, service *Service, instanceID, snapshotID string) string {
	t.Helper()
	dir, err := service.storage.SnapshotDir(instanceID, snapshotID)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
