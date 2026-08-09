package application

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/database"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/filesystem"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/modstorage"
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
) (RunningProcess, error) {
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
	service  *Service
	store    *database.SQLiteStore
	root     string
	launcher *lkgTestLauncher
	events   *lkgEventRecorder
}

func newLKGFixture(t *testing.T) lkgFixture {
	t.Helper()
	root := t.TempDir()
	store, err := database.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	launcher := &lkgTestLauncher{}
	service := NewService(
		store,
		filesystem.ArchiveInstaller{},
		filesystem.ModFileManager{},
		launcher,
		root,
	)
	t.Cleanup(func() { _ = service.Close() })
	events := &lkgEventRecorder{}
	service.SetEventPublisher(events)

	versionDirectory := filepath.Join(root, "versions", "1.20")
	if err := os.MkdirAll(versionDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(versionDirectory, "Vintagestory")
	if err := os.WriteFile(executable, []byte("game"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVersion(context.Background(), domain.GameVersion{
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
	if err := store.SaveSettings(context.Background(), domain.Settings{AutomaticSafetySnapshots: true}); err != nil {
		t.Fatal(err)
	}
	return lkgFixture{
		service:  service,
		store:    store,
		root:     root,
		launcher: launcher,
		events:   events,
	}
}

// setStartupWindow shortens the launch success window for a test and restores
// the production value afterwards.
func setStartupWindow(t *testing.T, window time.Duration) {
	t.Helper()
	previous := gameStartupWindow
	gameStartupWindow = window
	t.Cleanup(func() { gameStartupWindow = previous })
}

func (fixture lkgFixture) createInstance(t *testing.T, name string) domain.Instance {
	t.Helper()
	instance, err := fixture.service.CreateInstance(context.Background(), CreateInstanceInput{
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
	instance domain.Instance,
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
	record := domain.InstalledMod{
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
	instance domain.Instance,
	modID string,
	newReleaseID string,
	newVersion string,
) {
	t.Helper()
	mods, err := fixture.store.ListMods(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, mod := range mods {
		if found, _, ok := parseModDBSource(mod.Source); ok && found == modID {
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
	downloaded := domain.DownloadedMod{
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
func (fixture lkgFixture) waitForLastKnownGood(t *testing.T, instanceID string) domain.LastKnownGood {
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
	return domain.LastKnownGood{}
}

// waitForLastKnownGoodMod waits until the Last Known Good marker records the
// given version of a managed mod (a newer successful launch replaced it).
func (fixture lkgFixture) waitForLastKnownGoodMod(t *testing.T, instanceID, modID, version string) domain.LastKnownGood {
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
	return domain.LastKnownGood{}
}

func (fixture lkgFixture) launch(t *testing.T, instanceID string) {
	t.Helper()
	if _, err := fixture.service.Launch(context.Background(), instanceID, nil); err != nil {
		t.Fatal(err)
	}
}

func (fixture lkgFixture) waitForGameExit(t *testing.T) {
	t.Helper()
	fixture.events.waitForEvent(t, "game:exited", 3*time.Second)
}

func installedVersions(t *testing.T, fixture lkgFixture, instanceID string) map[string]string {
	t.Helper()
	mods, err := fixture.store.ListMods(context.Background(), instanceID)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string, len(mods))
	for _, mod := range mods {
		if modID, _, ok := parseModDBSource(mod.Source); ok {
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
	byID := make(map[string]domain.SnapshotMod, len(lkg.Mods))
	for _, mod := range lkg.Mods {
		byID[mod.ModID] = mod
	}
	if mod := byID["A"]; mod.Source != domain.SnapshotModSourceModDB || mod.ReleaseID != "r1" || mod.Version != "1.0.0" {
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
	suggestion, ok := payload.(domain.RecoverySuggestion)
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
	time.Sleep(3 * gameStartupWindow)
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

func TestCompareConfigurations(t *testing.T) {
	lkg := domain.LastKnownGood{
		GameVersion: "1.20",
		Mods: []domain.SnapshotMod{
			{Source: domain.SnapshotModSourceModDB, ModID: "A", ReleaseID: "r1", Version: "1.0.0"},
			{Source: domain.SnapshotModSourceModDB, ModID: "B", ReleaseID: "r2", Version: "2.0.0"},
			{Source: domain.SnapshotModSourceModDB, ModID: "C", ReleaseID: "r3", Version: "3.0.0"},
		},
	}
	current := []domain.SnapshotMod{
		{Source: domain.SnapshotModSourceModDB, ModID: "A", ReleaseID: "r4", Version: "2.0.0"},
		{Source: domain.SnapshotModSourceModDB, ModID: "B", ReleaseID: "r2", Version: "2.0.0"},
		{Source: domain.SnapshotModSourceModDB, ModID: "D", ReleaseID: "r5", Version: "1.0.0"},
	}
	names := map[string]string{
		"moddb:A": "Alpha",
		"moddb:B": "Beta",
		"moddb:D": "Delta",
	}

	changes := compareConfigurations(lkg, current, names, "1.20")
	if len(changes.Updated) != 1 || changes.Updated[0].Name != "Alpha" ||
		changes.Updated[0].From != "1.0.0" || changes.Updated[0].To != "2.0.0" {
		t.Fatalf("unexpected updated mods: %#v", changes.Updated)
	}
	if len(changes.Added) != 1 || changes.Added[0].Name != "Delta" || changes.Added[0].To != "1.0.0" {
		t.Fatalf("unexpected added mods: %#v", changes.Added)
	}
	if len(changes.Removed) != 1 || changes.Removed[0].Name != "C" || changes.Removed[0].From != "3.0.0" {
		t.Fatalf("unexpected removed mods: %#v", changes.Removed)
	}
	if changes.GameVersionFrom != "" {
		t.Fatalf("unexpected game version change: %#v", changes)
	}
	if changes.Count() != 3 {
		t.Fatalf("expected 3 changes, got %d", changes.Count())
	}
	if changes.Empty() {
		t.Fatal("changes must not be empty")
	}
}

func TestGameVersionChangeDetection(t *testing.T) {
	lkg := domain.LastKnownGood{GameVersion: "1.21.5"}
	changes := compareConfigurations(lkg, nil, nil, "1.21.6")
	if changes.GameVersionFrom != "1.21.5" || changes.GameVersionTo != "1.21.6" {
		t.Fatalf("unexpected game version change: %#v", changes)
	}
	if changes.Count() != 1 {
		t.Fatalf("expected 1 change, got %d", changes.Count())
	}
	unchanged := compareConfigurations(lkg, nil, nil, "1.21.5")
	if !unchanged.Empty() {
		t.Fatalf("same game version must not be a change: %#v", unchanged)
	}
}

func TestRecoverySnapshotPrefersLinkedSnapshotNotNewest(t *testing.T) {
	fixture := newLKGFixture(t)
	ctx := context.Background()
	instance := fixture.createInstance(t, "Selection")
	fixture.installManagedMod(t, instance, "A", "r1", "1.0.0")

	// S1 captures the Last Known Good state.
	first, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.recordLastKnownGood(ctx, instance)

	// S2 is newer but captures a different, broken state.
	fixture.changeInstalledMod(t, instance, "A", "r2", "2.0.0")
	second, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
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
	snapshotID, ok := fixture.service.resolveRecoverySnapshot(ctx, instance.ID, lkg)
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
	fixture.service.recordLastKnownGood(ctx, instance)
	lkg, err := fixture.store.GetLastKnownGood(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lkg.SnapshotID != "" {
		t.Fatalf("no snapshot should be linked, got %q", lkg.SnapshotID)
	}

	// A safety snapshot of the same state is created later.
	first, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	fixture.changeInstalledMod(t, instance, "A", "r2", "2.0.0")

	snapshotID, ok := fixture.service.resolveRecoverySnapshot(ctx, instance.ID, lkg)
	if !ok || snapshotID != first.ID {
		t.Fatalf("expected the state-matching snapshot %s to enable recovery, got %q (ok=%v)", first.ID, snapshotID, ok)
	}
}

func TestRestoreLastKnownGoodUsesSnapshotRestore(t *testing.T) {
	setStartupWindow(t, time.Hour)
	fixture := newLKGFixture(t)
	ctx := context.Background()
	fixture.service.ConfigureMods(nil, modstorage.New(fixture.root))
	instance := fixture.createInstance(t, "Restore")
	fixture.installManagedMod(t, instance, "A", "r1", "1.0.0")
	fixture.cacheModRelease(t, "A", "r1", "1.0.0", "checksum-1")

	// Working state: launch succeeds, the safety snapshot S1 is linked.
	operation, err := fixture.service.createInstanceSnapshotLocked(ctx, createSnapshotInput{
		instanceID:   instance.ID,
		snapshotType: domain.SnapshotTypeAutomatic,
		reason:       domain.SnapshotReasonBeforeModUpdate,
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.recordLastKnownGood(ctx, instance)
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
	suggestion := payload.(domain.RecoverySuggestion)
	if !suggestion.SnapshotExists || suggestion.SnapshotID != operation.ID {
		t.Fatalf("expected the recovery suggestion to reference %s, got %q", operation.ID, suggestion.SnapshotID)
	}

	// Restore Last Known Good goes through the existing snapshot restore path.
	if err := fixture.service.RestoreInstanceSnapshot(ctx, instance.ID, suggestion.SnapshotID); err != nil {
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
	fixture.service.recordLastKnownGood(ctx, instance)

	fixture.changeInstalledMod(t, instance, "A", "r2", "2.0.0")
	fixture.service.handleFailedLaunch(instance)
	payload := fixture.events.waitForEvent(t, "game:recovery-suggestion", time.Second)
	suggestion := payload.(domain.RecoverySuggestion)
	if suggestion.SnapshotExists || suggestion.SnapshotID != "" {
		t.Fatalf("no snapshot must be offered: %#v", suggestion)
	}
	if len(suggestion.Changes.Updated) != 1 || suggestion.Changes.Updated[0].From != "1.0.0" {
		t.Fatalf("changes must still be reported: %#v", suggestion.Changes)
	}

	status, err := fixture.service.GetLastKnownGoodStatus(ctx, instance.ID)
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
	operation, err := fixture.service.CreateInstanceSnapshot(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.recordLastKnownGood(ctx, instance)
	lkg, err := fixture.store.GetLastKnownGood(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lkg.SnapshotID != operation.ID {
		t.Fatalf("expected the snapshot to be linked, got %q", lkg.SnapshotID)
	}

	if err := fixture.service.DeleteInstanceSnapshot(ctx, instance.ID, operation.ID); err != nil {
		t.Fatal(err)
	}
	lkg, err = fixture.store.GetLastKnownGood(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lkg.SnapshotID != "" {
		t.Fatalf("expected the snapshot reference to be cleared, got %q", lkg.SnapshotID)
	}
	status, err := fixture.service.GetLastKnownGoodStatus(ctx, instance.ID)
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
	first, err := fixture.service.createSafetySnapshot(ctx, instance.ID, domain.SnapshotReasonBeforeModUpdate, nil)
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.recordLastKnownGood(ctx, instance)
	lkg, err := fixture.store.GetLastKnownGood(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lkg.SnapshotID != first.ID {
		t.Fatalf("expected the first snapshot to be linked, got %q", lkg.SnapshotID)
	}

	// Ten more automatic snapshots push the protected one past the retention
	// limit; retention must keep it and delete other snapshots instead.
	for index := 0; index < automaticSnapshotRetentionCount; index++ {
		if _, err := fixture.service.createSafetySnapshot(ctx, instance.ID, domain.SnapshotReasonBeforeModUpdate, nil); err != nil {
			t.Fatal(err)
		}
	}
	snapshots, err := fixture.service.ListInstanceSnapshots(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	var automatic []domain.InstanceSnapshot
	for _, snapshot := range snapshots {
		if snapshot.Type == domain.SnapshotTypeAutomatic {
			automatic = append(automatic, snapshot)
		}
	}
	if len(automatic) != automaticSnapshotRetentionCount+1 {
		t.Fatalf("expected %d automatic snapshots with the protected one kept, got %d", automaticSnapshotRetentionCount+1, len(automatic))
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
	reopened, err := database.Open(filepath.Join(fixture.root, "test.db"))
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

	if err := fixture.service.DeleteInstance(ctx, instance.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.GetLastKnownGood(ctx, instance.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected the last known good to be cleaned up, got %v", err)
	}
}
