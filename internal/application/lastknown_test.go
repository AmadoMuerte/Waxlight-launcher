package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/apptest"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/dataroot"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/filesystem"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/instancedirectory"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/modstorage"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/versionfs"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/launching"
	"github.com/waxlight/waxlight-launcher/internal/mods"
	"github.com/waxlight/waxlight-launcher/internal/mutations"
	"github.com/waxlight/waxlight-launcher/internal/operations"
	"github.com/waxlight/waxlight-launcher/internal/platform/process"
	platformsnapshots "github.com/waxlight/waxlight-launcher/internal/platform/snapshots"
	"github.com/waxlight/waxlight-launcher/internal/platform/sqlite"
	"github.com/waxlight/waxlight-launcher/internal/recovery"
	"github.com/waxlight/waxlight-launcher/internal/sessions"
	settingscore "github.com/waxlight/waxlight-launcher/internal/settings"
	"github.com/waxlight/waxlight-launcher/internal/snapshots"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

// The Last Known Good tests live in the application package so they can
// shorten gameStartupWindow and exercise the internal recording, comparison
// and recovery resolution helpers directly.

type lkgTestProcess struct {
	result chan processExit
}

type processExit struct {
	exitCode int
	err      error
}

func (process *lkgTestProcess) PID() int { return 7 }
func (process *lkgTestProcess) Wait() (int, error) {
	exit := <-process.result
	return exit.exitCode, exit.err
}
func (process *lkgTestProcess) Stop() error {
	process.result <- processExit{}
	return nil
}
func (process *lkgTestProcess) Kill() error {
	process.result <- processExit{exitCode: -1, err: errors.New("killed")}
	return nil
}

type lkgTestLauncher struct {
	process *lkgTestProcess
}

func (launcher *lkgTestLauncher) Start(
	_ context.Context,
	_ string,
	_ []string,
	_ string,
	_ map[string]string,
	_ io.Writer,
) (process.Running, error) {
	if launcher.process == nil {
		launcher.process = &lkgTestProcess{result: make(chan processExit, 1)}
	}
	return launcher.process, nil
}

func (launcher *lkgTestLauncher) crash() {
	launcher.process.result <- processExit{exitCode: 1, err: errors.New("crash")}
}

func (launcher *lkgTestLauncher) exit(exitCode int) {
	launcher.process.result <- processExit{exitCode: exitCode}
}

type lkgEventRecorder struct {
	mu     sync.Mutex
	events []recordedEvent
}

type recordedEvent struct {
	name    string
	payload any
}

func (recorder *lkgEventRecorder) Publish(name string, payload any) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.events = append(recorder.events, recordedEvent{name: name, payload: payload})
}

func (recorder *lkgEventRecorder) last(name string) (any, bool) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	for index := len(recorder.events) - 1; index >= 0; index-- {
		if recorder.events[index].name == name {
			return recorder.events[index].payload, true
		}
	}
	return nil, false
}

// waitForEvent polls until an event with the given name was published.
func (recorder *lkgEventRecorder) waitForEvent(t *testing.T, name string, timeout time.Duration) any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if payload, ok := recorder.last(name); ok {
			return payload
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("event %q was never published", name)
	return nil
}

func (recorder *lkgEventRecorder) has(name string) bool {
	_, ok := recorder.last(name)
	return ok
}

type lkgFixture struct {
	service   *Service
	store     *sqlite.SQLiteStore
	root      string
	launcher  *lkgTestLauncher
	launching *launching.Coordinator
	events    *lkgEventRecorder
}

func newLKGFixture(t *testing.T) lkgFixture {
	t.Helper()
	root := t.TempDir()
	store, err := sqlite.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	launcher := &lkgTestLauncher{}
	lifecycle := apptest.NewLifecycle()
	lifecycle.Startup(context.Background())
	operationManager := operations.NewManager(store, lifecycle, nil)
	sessionService := sessions.NewService(store, time.Now)
	gate := &mutations.Gate{}
	settingsReader := settingscore.NewReader(store)
	versionFilesystem := versionfs.New(root)
	archiveInstaller := filesystem.ArchiveInstaller{}
	versionRuntime := versions.NewInstallRuntime(versionFilesystem, gate, operationManager, time.Now, func() string { return "lkg-version-operation" })
	versionQueries := versions.NewQueryService(store, nil, archiveInstaller, versionFilesystem, time.Now)
	versionService := versions.NewCapabilities(
		versionQueries,
		versions.NewLocalInstallService(store, archiveInstaller, versionRuntime, runtime.GOOS, runtime.GOARCH),
		versions.NewCatalogInstallService(store, versionQueries, nil, nil, nil, versionRuntime, nil, root),
		versions.NewRemovalService(store, store, versionFilesystem, gate, nil),
	)
	instanceQueries := instances.NewQueryService(store)
	events := &lkgEventRecorder{}
	instanceCreator := instances.NewCreateService(
		store,
		versionService,
		store,
		func(ctx context.Context) (string, error) {
			settings, err := settingsReader.Get(ctx)
			return settings.Language, err
		},
		gate,
		instancedirectory.New(filesystem.ModFileManager{}),
		events,
		nil,
		root,
		time.Now,
		func() string { return fmt.Sprintf("lkg-instance-%d", time.Now().UnixNano()) },
	)
	instanceSlot := mutations.NewSlot()
	launchRegistry := launching.NewRegistry(instanceSlot)
	service := NewService(
		store,
		filesystem.ModFileManager{},
		root,
		platformsnapshots.New(root),
		dataroot.TotalSizeContext,
		filesystem.SanitizeClientSettings,
		instancedirectory.HardenLogs,
		operationManager,
		sessionService,
		instanceQueries,
		instanceCreator,
		instancedirectory.NewCloneStorage(filesystem.SanitizeClientSettings),
		versionService,
		nil,
		nil,
		gate,
		settingsReader,
		instanceSlot,
		launchRegistry,
		nil,
		modstorage.New(root),
		events,
		nil,
	)
	launchCoordinator := launching.NewCoordinator(
		launchRegistry,
		gate,
		store,
		versionService,
		nil,
		filesystem.ClientSettingsService{},
		settingsReader,
		filesystem.ModFileManager{},
		sessionService,
		launcher,
		instancedirectory.LaunchLogs{},
		events,
		nil,
		service.Recovery(),
		lifecycle,
		operationManager,
		time.Now,
		func() string { return fmt.Sprintf("lkg-session-%d", time.Now().UnixNano()) },
	)
	t.Cleanup(func() {
		lifecycle.Shutdown()
		launchCoordinator.RecordEstablishedOnShutdown()
		_ = service.Close()
	})
	service.SetEventPublisher(events)

	versionDirectory := filepath.Join(root, "versions", "1.20")
	if err := os.MkdirAll(versionDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(versionDirectory, "Vintagestory")
	if err := os.WriteFile(executable, []byte("game"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVersion(context.Background(), versions.GameVersion{
		ID:              "1.20",
		Name:            "1.20",
		Channel:         "stable",
		Platform:        "linux",
		Architecture:    "amd64",
		InstallationDir: versionDirectory,
		ExecutablePath:  executable,
		Status:          "installed",
		InstalledAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSettings(context.Background(), settingscore.Settings{AutomaticSafetySnapshots: true}); err != nil {
		t.Fatal(err)
	}
	return lkgFixture{
		service:   service,
		store:     store,
		root:      root,
		launcher:  launcher,
		launching: launchCoordinator,
		events:    events,
	}
}

// setStartupWindow shortens the launch success window for a test and restores
// the production value afterwards.
func setStartupWindow(t *testing.T, window time.Duration) {
	t.Helper()
	previous := launching.GameStartupWindow()
	launching.SetGameStartupWindow(window)
	t.Cleanup(func() { launching.SetGameStartupWindow(previous) })
}

func (fixture lkgFixture) createInstance(t *testing.T, name string) instances.Instance {
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

// installManagedMod writes a managed ModDB mod record and file into an
// instance. Counter allows installing several distinct mods.
var lkgModCounter int

func (fixture lkgFixture) installManagedMod(
	t *testing.T,
	instance instances.Instance,
	modID string,
	releaseID string,
	version string,
) {
	t.Helper()
	lkgModCounter++
	path := filepath.Join(instance.Directory, "Mods", modID+"-"+releaseID+".zip")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(modID+"-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	record := mods.InstalledMod{
		ID:          "lkg-mod-" + modID + "-" + releaseID,
		InstanceID:  instance.ID,
		Name:        "Mod " + modID,
		Version:     version,
		FileName:    filepath.Base(path),
		FilePath:    path,
		Enabled:     true,
		Managed:     true,
		Source:      "moddb:" + modID + ":" + releaseID,
		SizeBytes:   int64(len(modID + "-bytes")),
		InstalledAt: time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := fixture.store.SaveMod(context.Background(), record); err != nil {
		t.Fatal(err)
	}
}

// changeInstalledMod replaces the record and file of a managed mod with
// another release, simulating an update without the full download pipeline.
func (fixture lkgFixture) changeInstalledMod(
	t *testing.T,
	instance instances.Instance,
	modID string,
	newReleaseID string,
	newVersion string,
) {
	t.Helper()
	installedMods, err := fixture.store.ListMods(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, mod := range installedMods {
		if found, _, ok := mods.ParseModDBSource(mod.Source); ok && found == modID {
			path := filepath.Join(instance.Directory, "Mods", modID+"-"+newReleaseID+".zip")
			if err := os.WriteFile(path, []byte(modID+"-bytes"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(mod.FilePath); err != nil {
				t.Fatal(err)
			}
			mod.FileName = filepath.Base(path)
			mod.FilePath = path
			mod.Version = newVersion
			mod.Source = "moddb:" + modID + ":" + newReleaseID
			mod.UpdatedAt = time.Now().UTC()
			if err := fixture.store.SaveMod(context.Background(), mod); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("mod %s is not installed", modID)
}

// cacheModRelease stores a downloadable release in the shared mod cache so
// snapshot restore can fetch it without the network.
func (fixture lkgFixture) cacheModRelease(
	t *testing.T,
	modID string,
	releaseID string,
	version string,
	checksum string,
) {
	t.Helper()
	fileName := modID + "_" + releaseID + ".zip"
	path := filepath.Join(fixture.root, "cache", "mods", modID, releaseID, fileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(modID+"-cached-"+releaseID), 0o644); err != nil {
		t.Fatal(err)
	}
	downloaded := mods.DownloadedMod{
		SchemaVersion:     1,
		ModID:             modID,
		Slug:              strings.ToLower(modID),
		Name:              "Mod " + modID,
		VersionID:         releaseID,
		DownloadedVersion: version,
		FileName:          fileName,
		FilePath:          path,
		FileSize:          int64(len(modID + "-cached-" + releaseID)),
		Checksum:          checksum,
		DownloadURL:       "https://mods.example/" + modID + "_" + releaseID + ".zip",
		DownloadedAt:      time.Now().UTC(),
	}
	if err := modstorage.New(fixture.root).Save(context.Background(), downloaded); err != nil {
		t.Fatal(err)
	}
}

// waitForLastKnownGood polls until a Last Known Good marker exists.
func (fixture lkgFixture) waitForLastKnownGood(t *testing.T, instanceID string) recovery.LastKnownGood {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		lkg, err := fixture.store.GetLastKnownGood(context.Background(), instanceID)
		if err == nil {
			return lkg
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("last known good was never recorded")
	return recovery.LastKnownGood{}
}

// waitForLastKnownGoodMod waits until the Last Known Good marker records the
// given version of a managed mod (a newer successful launch replaced it).
func (fixture lkgFixture) waitForLastKnownGoodMod(t *testing.T, instanceID, modID, version string) recovery.LastKnownGood {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		lkg, err := fixture.store.GetLastKnownGood(context.Background(), instanceID)
		if err == nil {
			for _, mod := range lkg.Mods {
				if mod.ModID == modID && mod.Version == version {
					return lkg
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("last known good was never replaced with mod %s %s", modID, version)
	return recovery.LastKnownGood{}
}

func (fixture lkgFixture) launch(t *testing.T, instanceID string) {
	t.Helper()
	if _, err := fixture.launching.Launch(context.Background(), instanceID, nil); err != nil {
		t.Fatal(err)
	}
}

func (fixture lkgFixture) waitForGameExit(t *testing.T) {
	t.Helper()
	fixture.events.waitForEvent(t, "game:exited", 3*time.Second)
}

func installedVersions(t *testing.T, fixture lkgFixture, instanceID string) map[string]string {
	t.Helper()
	installed, err := fixture.store.ListMods(context.Background(), instanceID)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string, len(installed))
	for _, mod := range installed {
		if modID, _, ok := mods.ParseModDBSource(mod.Source); ok {
			result[modID] = mod.Version
		}
	}
	return result
}

func TestLastKnownGoodRecordedAfterSuccessfulLaunch(t *testing.T) {
	setStartupWindow(t, 50*time.Millisecond)
	fixture := newLKGFixture(t)
	instance := fixture.createInstance(t, "Working")
	fixture.installManagedMod(t, instance, "A", "r1", "1.0.0")
	fixture.installManagedMod(t, instance, "B", "r2", "2.0.0")

	fixture.launch(t, instance.ID)
	lkg := fixture.waitForLastKnownGood(t, instance.ID)
	if lkg.GameVersion != "1.20" {
		t.Fatalf("unexpected game version in last known good: %q", lkg.GameVersion)
	}
	if len(lkg.Mods) != 2 {
		t.Fatalf("expected 2 mods in the last known good, got %d", len(lkg.Mods))
	}
	byID := make(map[string]snapshots.Mod, len(lkg.Mods))
	for _, mod := range lkg.Mods {
		byID[mod.ModID] = mod
	}
	if mod := byID["A"]; mod.Source != snapshots.ModSourceModDB || mod.ReleaseID != "r1" || mod.Version != "1.0.0" {
		t.Fatalf("unexpected mod A entry: %#v", mod)
	}
	if mod := byID["B"]; mod.Version != "2.0.0" {
		t.Fatalf("unexpected mod B entry: %#v", mod)
	}
	if !fixture.events.has("last-known-good:updated") {
		t.Fatal("no last-known-good:updated event was published")
	}

	// A normal exit after the launch must not trigger recovery.
	fixture.launcher.exit(0)
	fixture.waitForGameExit(t)
	if fixture.events.has("game:recovery-suggestion") {
		t.Fatal("a normal exit triggered a recovery suggestion")
	}
}

func TestLastKnownGoodReplacedByNewSuccessfulConfiguration(t *testing.T) {
	setStartupWindow(t, 50*time.Millisecond)
	fixture := newLKGFixture(t)
	instance := fixture.createInstance(t, "Replacement")
	fixture.installManagedMod(t, instance, "A", "r1", "1.0.0")

	fixture.launch(t, instance.ID)
	lkg := fixture.waitForLastKnownGood(t, instance.ID)
	if mod := lkg.Mods[0]; mod.Version != "1.0.0" {
		t.Fatalf("expected the old configuration as last known good, got %#v", mod)
	}
	fixture.launcher.exit(0)
	fixture.waitForGameExit(t)

	fixture.changeInstalledMod(t, instance, "A", "r2", "2.0.0")
	fixture.launch(t, instance.ID)
	lkg = fixture.waitForLastKnownGoodMod(t, instance.ID, "A", "2.0.0")
	if len(lkg.Mods) != 1 || lkg.Mods[0].Version != "2.0.0" || lkg.Mods[0].ReleaseID != "r2" {
		t.Fatalf("expected the new working configuration as last known good, got %#v", lkg.Mods)
	}
}

func TestFailedLaunchDoesNotReplaceLastKnownGood(t *testing.T) {
	setStartupWindow(t, 50*time.Millisecond)
	fixture := newLKGFixture(t)
	ctx := context.Background()
	instance := fixture.createInstance(t, "Broken")
	fixture.installManagedMod(t, instance, "A", "r1", "1.0.0")

	fixture.launch(t, instance.ID)
	fixture.waitForLastKnownGood(t, instance.ID)
	fixture.launcher.exit(0)
	fixture.waitForGameExit(t)

	// The new configuration crashes during startup.
	fixture.changeInstalledMod(t, instance, "A", "r2", "2.0.0")
	setStartupWindow(t, time.Hour)
	fixture.launch(t, instance.ID)
	fixture.launcher.crash()
	fixture.waitForGameExit(t)

	persisted, err := fixture.store.GetLastKnownGood(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Mods) != 1 || persisted.Mods[0].Version != "1.0.0" {
		t.Fatalf("the failed configuration replaced the last known good: %#v", persisted.Mods)
	}
	if persisted.Mods[0].ReleaseID != "r1" {
		t.Fatalf("the failed configuration replaced the last known good release: %#v", persisted.Mods)
	}

	payload := fixture.events.waitForEvent(t, "game:recovery-suggestion", time.Second)
	suggestion, ok := payload.(recovery.RecoverySuggestion)
	if !ok {
		t.Fatalf("unexpected suggestion payload %T", payload)
	}
	if len(suggestion.Changes.Updated) != 1 || suggestion.Changes.Updated[0].Name != "Mod A" ||
		suggestion.Changes.Updated[0].From != "1.0.0" || suggestion.Changes.Updated[0].To != "2.0.0" {
		t.Fatalf("unexpected changes in the suggestion: %#v", suggestion.Changes)
	}
	if suggestion.StateSignature == "" {
		t.Fatal("the suggestion misses its state signature")
	}

	// The suggestion is published as a JSON event; the change lists must be
	// arrays even when empty, otherwise the frontend crashes rendering them.
	data, err := json.Marshal(suggestion)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	changes, ok := parsed["changes"].(map[string]any)
	if !ok {
		t.Fatalf("suggestion changes must be an object: %s", data)
	}
	for _, key := range []string{"updated", "added", "removed"} {
		if _, ok := changes[key].([]any); !ok {
			t.Fatalf("suggestion %q must serialize as an array: %s", key, data)
		}
	}
}

func TestSuccessfulLaunchThenCrashAfterWindowDoesNotSuggestRecovery(t *testing.T) {
	setStartupWindow(t, 50*time.Millisecond)
	fixture := newLKGFixture(t)
	instance := fixture.createInstance(t, "Long session")
	fixture.installManagedMod(t, instance, "A", "r1", "1.0.0")

	fixture.launch(t, instance.ID)
	fixture.waitForLastKnownGood(t, instance.ID)

	// A crash after a long session is not a configuration failure.
	time.Sleep(3 * launching.GameStartupWindow())
	fixture.launcher.crash()
	fixture.waitForGameExit(t)
	if fixture.events.has("game:recovery-suggestion") {
		t.Fatal("a crash after the startup window triggered a recovery suggestion")
	}
}

func TestFailedLaunchWithoutChangesEmitsNoSuggestion(t *testing.T) {
	setStartupWindow(t, 50*time.Millisecond)
	fixture := newLKGFixture(t)
	instance := fixture.createInstance(t, "Same state")
	fixture.installManagedMod(t, instance, "A", "r1", "1.0.0")

	fixture.launch(t, instance.ID)
	fixture.waitForLastKnownGood(t, instance.ID)
	fixture.launcher.exit(0)
	fixture.waitForGameExit(t)

	// The same configuration crashes during startup: no changes to report.
	setStartupWindow(t, time.Hour)
	fixture.launch(t, instance.ID)
	fixture.launcher.crash()
	fixture.waitForGameExit(t)
	if fixture.events.has("game:recovery-suggestion") {
		t.Fatal("a failed launch without configuration changes emitted a recovery suggestion")
	}
}

func TestRecoverySnapshotPrefersLinkedSnapshotNotNewest(t *testing.T) {
	fixture := newLKGFixture(t)
	ctx := context.Background()
	instance := fixture.createInstance(t, "Selection")
	fixture.installManagedMod(t, instance, "A", "r1", "1.0.0")

	// S1 captures the Last Known Good state.
	first, err := fixture.service.Snapshots().Create(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.Recovery().RecordLastKnownGood(ctx, instance)

	// S2 is newer but captures a different, broken state.
	fixture.changeInstalledMod(t, instance, "A", "r2", "2.0.0")
	second, err := fixture.service.Snapshots().Create(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}

	lkg, err := fixture.store.GetLastKnownGood(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lkg.SnapshotID != first.ID {
		t.Fatalf("expected the last known good to reference %s, got %q", first.ID, lkg.SnapshotID)
	}
	snapshotID, ok := fixture.service.Recovery().ResolveRecoverySnapshot(ctx, instance.ID, lkg)
	if !ok || snapshotID != first.ID {
		t.Fatalf("recovery must prefer the linked snapshot %s, got %q (ok=%v)", first.ID, snapshotID, ok)
	}
	if snapshotID == second.ID {
		t.Fatal("the newest snapshot was selected for recovery instead of the last known good one")
	}
}

func TestRecoveryFallsBackToSnapshotCreatedAfterMarker(t *testing.T) {
	fixture := newLKGFixture(t)
	ctx := context.Background()
	instance := fixture.createInstance(t, "Fallback")
	fixture.installManagedMod(t, instance, "A", "r1", "1.0.0")

	// No snapshot exists when the marker is recorded.
	fixture.service.Recovery().RecordLastKnownGood(ctx, instance)
	lkg, err := fixture.store.GetLastKnownGood(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lkg.SnapshotID != "" {
		t.Fatalf("no snapshot should be linked, got %q", lkg.SnapshotID)
	}

	// A safety snapshot of the same state is created later.
	first, err := fixture.service.Snapshots().Create(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	fixture.changeInstalledMod(t, instance, "A", "r2", "2.0.0")

	snapshotID, ok := fixture.service.Recovery().ResolveRecoverySnapshot(ctx, instance.ID, lkg)
	if !ok || snapshotID != first.ID {
		t.Fatalf("expected the state-matching snapshot %s to enable recovery, got %q (ok=%v)", first.ID, snapshotID, ok)
	}
}

func TestRestoreLastKnownGoodUsesSnapshotRestore(t *testing.T) {
	setStartupWindow(t, time.Hour)
	fixture := newLKGFixture(t)
	ctx := context.Background()
	instance := fixture.createInstance(t, "Restore")
	fixture.installManagedMod(t, instance, "A", "r1", "1.0.0")
	fixture.cacheModRelease(t, "A", "r1", "1.0.0", "checksum-1")

	// Working state: launch succeeds, the safety snapshot S1 is linked.
	operation, err := fixture.service.Snapshots().CreateSafety(ctx, instance.ID, snapshots.ReasonBeforeModUpdate, nil)
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.Recovery().RecordLastKnownGood(ctx, instance)
	lkg, err := fixture.store.GetLastKnownGood(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lkg.SnapshotID != operation.ID {
		t.Fatalf("expected the last known good to link the safety snapshot, got %q", lkg.SnapshotID)
	}

	// The update breaks the instance; the suggestion points at S1.
	fixture.changeInstalledMod(t, instance, "A", "r2", "2.0.0")
	fixture.launch(t, instance.ID)
	fixture.launcher.crash()
	fixture.waitForGameExit(t)
	payload := fixture.events.waitForEvent(t, "game:recovery-suggestion", time.Second)
	suggestion := payload.(recovery.RecoverySuggestion)
	if !suggestion.SnapshotExists || suggestion.SnapshotID != operation.ID {
		t.Fatalf("expected the recovery suggestion to reference %s, got %q", operation.ID, suggestion.SnapshotID)
	}

	// Restore Last Known Good goes through the existing snapshot restore path.
	if err := fixture.service.Snapshots().Restore(ctx, instance.ID, suggestion.SnapshotID); err != nil {
		t.Fatal(err)
	}
	versions := installedVersions(t, fixture, instance.ID)
	if versions["A"] != "1.0.0" {
		t.Fatalf("restore did not return the mod to the last known good release: %#v", versions)
	}
	persisted, err := fixture.store.GetLastKnownGood(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Mods) != 1 || persisted.Mods[0].Version != "1.0.0" || persisted.Mods[0].ReleaseID != "r1" {
		t.Fatalf("restore must not replace the last known good marker: %#v", persisted.Mods)
	}
}

func TestRecoveryWithoutSnapshotShowsChangesOnly(t *testing.T) {
	fixture := newLKGFixture(t)
	ctx := context.Background()
	instance := fixture.createInstance(t, "No snapshot")
	fixture.installManagedMod(t, instance, "A", "r1", "1.0.0")
	fixture.service.Recovery().RecordLastKnownGood(ctx, instance)

	fixture.changeInstalledMod(t, instance, "A", "r2", "2.0.0")
	fixture.service.Recovery().HandleFailedLaunch(instance)
	payload := fixture.events.waitForEvent(t, "game:recovery-suggestion", time.Second)
	suggestion := payload.(recovery.RecoverySuggestion)
	if suggestion.SnapshotExists || suggestion.SnapshotID != "" {
		t.Fatalf("no snapshot must be offered: %#v", suggestion)
	}
	if len(suggestion.Changes.Updated) != 1 || suggestion.Changes.Updated[0].From != "1.0.0" {
		t.Fatalf("changes must still be reported: %#v", suggestion.Changes)
	}

	status, err := fixture.service.Recovery().Status(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.MatchesCurrent || status.Changes.Count() == 0 {
		t.Fatalf("unexpected status: %#v", status)
	}
	if status.SnapshotExists || status.SnapshotID != "" {
		t.Fatalf("status must not offer a recovery snapshot: %#v", status)
	}
}

func TestSnapshotDeletionClearsLastKnownGoodReference(t *testing.T) {
	fixture := newLKGFixture(t)
	ctx := context.Background()
	instance := fixture.createInstance(t, "Reference")
	fixture.installManagedMod(t, instance, "A", "r1", "1.0.0")
	operation, err := fixture.service.Snapshots().Create(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.Recovery().RecordLastKnownGood(ctx, instance)
	lkg, err := fixture.store.GetLastKnownGood(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lkg.SnapshotID != operation.ID {
		t.Fatalf("expected the snapshot to be linked, got %q", lkg.SnapshotID)
	}

	if err := fixture.service.Snapshots().Delete(ctx, instance.ID, operation.ID); err != nil {
		t.Fatal(err)
	}
	lkg, err = fixture.store.GetLastKnownGood(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lkg.SnapshotID != "" {
		t.Fatalf("expected the snapshot reference to be cleared, got %q", lkg.SnapshotID)
	}
	status, err := fixture.service.Recovery().Status(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.SnapshotExists {
		t.Fatal("a deleted snapshot must not stay available for recovery")
	}
}

func TestAutomaticRetentionKeepsLastKnownGoodRecoverySnapshot(t *testing.T) {
	fixture := newLKGFixture(t)
	ctx := context.Background()
	instance := fixture.createInstance(t, "Retention")
	fixture.installManagedMod(t, instance, "A", "r1", "1.0.0")

	// The first automatic snapshot becomes the Last Known Good recovery point.
	first, err := fixture.service.Snapshots().CreateSafety(ctx, instance.ID, snapshots.ReasonBeforeModUpdate, nil)
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.Recovery().RecordLastKnownGood(ctx, instance)
	lkg, err := fixture.store.GetLastKnownGood(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lkg.SnapshotID != first.ID {
		t.Fatalf("expected the first snapshot to be linked, got %q", lkg.SnapshotID)
	}

	// Ten more automatic snapshots push the protected one past the retention
	// limit; retention must keep it and delete other snapshots instead.
	const retentionLimit = 10
	for index := 0; index < retentionLimit; index++ {
		if _, err := fixture.service.Snapshots().CreateSafety(ctx, instance.ID, snapshots.ReasonBeforeModUpdate, nil); err != nil {
			t.Fatal(err)
		}
	}
	listed, err := fixture.service.Snapshots().List(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	var automatic []snapshots.InstanceSnapshot
	for _, snapshot := range listed {
		if snapshot.Type == snapshots.TypeAutomatic {
			automatic = append(automatic, snapshot)
		}
	}
	if len(automatic) != retentionLimit+1 {
		t.Fatalf("expected %d automatic snapshots with the protected one kept, got %d", retentionLimit+1, len(automatic))
	}
	found := false
	for _, snapshot := range automatic {
		if snapshot.ID == first.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("retention removed the active last known good recovery snapshot")
	}
}

func TestLastKnownGoodSurvivesLauncherRestart(t *testing.T) {
	setStartupWindow(t, 50*time.Millisecond)
	fixture := newLKGFixture(t)
	instance := fixture.createInstance(t, "Restart")
	fixture.installManagedMod(t, instance, "A", "r1", "1.0.0")
	fixture.launch(t, instance.ID)
	fixture.waitForLastKnownGood(t, instance.ID)
	fixture.launcher.exit(0)
	fixture.waitForGameExit(t)
	if err := fixture.service.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen the same database: the marker must still be there.
	reopened, err := sqlite.Open(filepath.Join(fixture.root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	lkg, err := reopened.GetLastKnownGood(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(lkg.Mods) != 1 || lkg.Mods[0].Version != "1.0.0" {
		t.Fatalf("the last known good did not survive the restart: %#v", lkg.Mods)
	}
}

func TestDeletedInstanceCleansLastKnownGood(t *testing.T) {
	setStartupWindow(t, 50*time.Millisecond)
	fixture := newLKGFixture(t)
	ctx := context.Background()
	instance := fixture.createInstance(t, "Cleanup")
	fixture.installManagedMod(t, instance, "A", "r1", "1.0.0")
	fixture.launch(t, instance.ID)
	fixture.waitForLastKnownGood(t, instance.ID)
	fixture.launcher.exit(0)
	fixture.waitForGameExit(t)

	if err := fixture.service.InstanceDeleter().Delete(ctx, instance.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.GetLastKnownGood(ctx, instance.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected the last known good to be cleaned up, got %v", err)
	}
}
