package recovery

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/snapshots"
)

// fakeRepository persists Last Known Good markers in memory.
type fakeRepository struct {
	mu     sync.Mutex
	marker map[string]LastKnownGood
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{marker: make(map[string]LastKnownGood)}
}

func (repository *fakeRepository) GetLastKnownGood(_ context.Context, instanceID string) (LastKnownGood, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	marker, ok := repository.marker[instanceID]
	if !ok {
		return LastKnownGood{}, domain.ErrNotFound
	}
	return marker, nil
}

func (repository *fakeRepository) SaveLastKnownGood(_ context.Context, marker LastKnownGood) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.marker[marker.InstanceID] = marker
	return nil
}

func (repository *fakeRepository) DeleteLastKnownGood(_ context.Context, instanceID string) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	delete(repository.marker, instanceID)
	return nil
}

// fakeSnapshots serves the snapshot reader and mod-configuration ports.
type fakeSnapshots struct {
	installedMods map[string][]snapshots.InstalledMod
	manifests     map[string]snapshots.Manifest
	restorable    map[string]bool
}

func (fake *fakeSnapshots) List(_ context.Context, instanceID string) ([]snapshots.InstanceSnapshot, error) {
	result := []snapshots.InstanceSnapshot{}
	for id := range fake.manifests {
		if fake.manifests[id].InstanceID == instanceID {
			result = append(result, snapshots.InstanceSnapshot{
				ID:         fake.manifests[id].ID,
				InstanceID: instanceID,
				CreatedAt:  fake.manifests[id].CreatedAt,
			})
		}
	}
	return result, nil
}

func (fake *fakeSnapshots) ReadManifest(_ context.Context, _, snapshotID string) (snapshots.Manifest, error) {
	manifest, ok := fake.manifests[snapshotID]
	if !ok {
		return snapshots.Manifest{}, domain.ErrNotFound
	}
	return manifest, nil
}

func (fake *fakeSnapshots) IsRestorable(_ context.Context, _, snapshotID string) bool {
	return fake.restorable[snapshotID]
}

func (fake *fakeSnapshots) ListInstalledMods(_ context.Context, instanceID string) ([]snapshots.InstalledMod, error) {
	return fake.installedMods[instanceID], nil
}

func (fake *fakeSnapshots) ModManifest(_ context.Context, installed []snapshots.InstalledMod) []snapshots.Mod {
	result := make([]snapshots.Mod, 0, len(installed))
	for _, mod := range installed {
		if modID, releaseID, ok := snapshots.ParseModDBSource(mod.Source); ok {
			result = append(result, snapshots.Mod{Source: snapshots.ModSourceModDB, ModID: modID, ReleaseID: releaseID, Identifier: mod.Name, Version: mod.Version, Enabled: mod.Enabled})
		} else {
			result = append(result, snapshots.Mod{Source: snapshots.ModSourceUnknown, Identifier: mod.Name, Version: mod.Version, FileName: mod.FileName, Enabled: mod.Enabled})
		}
	}
	return result
}

func (fake *fakeSnapshots) GameVersionName(_ context.Context, gameVersionID string) string {
	return gameVersionID
}

// fakeEvents records published events.
type fakeEvents struct {
	mu     sync.Mutex
	events map[string][]any
}

func (events *fakeEvents) Publish(name string, payload any) {
	events.mu.Lock()
	defer events.mu.Unlock()
	if events.events == nil {
		events.events = make(map[string][]any)
	}
	events.events[name] = append(events.events[name], payload)
}

func (events *fakeEvents) count(name string) int {
	events.mu.Lock()
	defer events.mu.Unlock()
	return len(events.events[name])
}

func (events *fakeEvents) last(name string) any {
	events.mu.Lock()
	defer events.mu.Unlock()
	values := events.events[name]
	if len(values) == 0 {
		return nil
	}
	return values[len(values)-1]
}

func newTestService(repository *fakeRepository, fakeSnapshots *fakeSnapshots, events *fakeEvents) *Service {
	return NewService(
		repository,
		fakeSnapshots,
		fakeSnapshots,
		fakeInstances{},
		fakeGate{},
		events,
		time.Now,
	)
}

type fakeInstances struct{}

func (fakeInstances) GetInstance(_ context.Context, id string) (instances.Instance, error) {
	return instances.Instance{ID: id, Name: "Survival", GameVersionID: "1.20"}, nil
}

type fakeGate struct{}

func (fakeGate) Begin() error { return nil }
func (fakeGate) End()         {}

func testInstance() instances.Instance {
	return instances.Instance{ID: "instance-1", Name: "Survival", GameVersionID: "1.20"}
}

func TestRecordLastKnownGoodPersistsAndPublishes(t *testing.T) {
	repository := newFakeRepository()
	events := &fakeEvents{}
	service := newTestService(repository, &fakeSnapshots{}, events)

	service.RecordLastKnownGood(context.Background(), testInstance())

	marker, err := repository.GetLastKnownGood(context.Background(), "instance-1")
	if err != nil {
		t.Fatal(err)
	}
	if marker.GameVersion != "1.20" || len(marker.Mods) != 0 || marker.SnapshotID != "" {
		t.Fatalf("unexpected marker: %#v", marker)
	}
	if events.count("last-known-good:updated") != 1 {
		t.Fatal("last-known-good:updated event was not published")
	}
}

func TestRecordLastKnownGoodLinksMatchingSnapshot(t *testing.T) {
	repository := newFakeRepository()
	mods := []snapshots.Mod{{Source: snapshots.ModSourceModDB, ModID: "100", ReleaseID: "1000", Identifier: "fancy", Version: "1.0.0"}}
	snapshotsService := &fakeSnapshots{
		manifests: map[string]snapshots.Manifest{
			"snap-1": {
				FormatVersion: snapshots.FormatVersion,
				ID:            "snap-1",
				InstanceID:    "instance-1",
				CreatedAt:     time.Now().UTC(),
				Type:          snapshots.TypeAutomatic,
				Reason:        snapshots.ReasonBeforeModUpdate,
				GameVersion:   "1.20",
				Mods:          mods,
			},
		},
		restorable: map[string]bool{"snap-1": true},
		installedMods: map[string][]snapshots.InstalledMod{
			"instance-1": {{Name: "fancy", Version: "1.0.0", Source: "moddb:100:1000", Enabled: true}},
		},
	}
	events := &fakeEvents{}
	service := newTestService(repository, snapshotsService, events)

	service.RecordLastKnownGood(context.Background(), testInstance())

	marker, err := repository.GetLastKnownGood(context.Background(), "instance-1")
	if err != nil {
		t.Fatal(err)
	}
	if marker.SnapshotID != "snap-1" {
		t.Fatalf("expected the matching snapshot to be linked, got %q", marker.SnapshotID)
	}
}

func TestHandleFailedLaunchEmitsSuggestionOnlyWhenChanged(t *testing.T) {
	repository := newFakeRepository()
	marker := LastKnownGood{InstanceID: "instance-1", GameVersion: "1.20", Mods: nil}
	if err := repository.SaveLastKnownGood(context.Background(), marker); err != nil {
		t.Fatal(err)
	}
	events := &fakeEvents{}
	service := newTestService(repository, &fakeSnapshots{}, events)

	// Same configuration: no suggestion.
	service.HandleFailedLaunch(testInstance())
	if events.count("game:recovery-suggestion") != 0 {
		t.Fatal("a failed launch without changes emitted a suggestion")
	}

	// A changed configuration emits exactly one suggestion.
	changed := &fakeSnapshots{
		installedMods: map[string][]snapshots.InstalledMod{
			"instance-1": {{Name: "fancy", Version: "2.0.0", Source: "moddb:100:2000", Enabled: true}},
		},
	}
	service = newTestService(repository, changed, events)
	service.HandleFailedLaunch(testInstance())
	if events.count("game:recovery-suggestion") != 1 {
		t.Fatal("a changed configuration must emit a recovery suggestion")
	}
	suggestion, ok := events.last("game:recovery-suggestion").(RecoverySuggestion)
	if !ok {
		t.Fatal("unexpected suggestion payload")
	}
	if suggestion.InstanceID != "instance-1" || suggestion.SnapshotExists || len(suggestion.Changes.Added) != 1 {
		t.Fatalf("unexpected suggestion: %#v", suggestion)
	}
	if suggestion.StateSignature == "" {
		t.Fatal("suggestion misses its state signature")
	}
}

func TestResolveRecoverySnapshotFallsBackToMatchingManifest(t *testing.T) {
	repository := newFakeRepository()
	mods := []snapshots.Mod{{Source: snapshots.ModSourceModDB, ModID: "100", ReleaseID: "1000", Identifier: "fancy", Version: "1.0.0"}}
	snapshotsService := &fakeSnapshots{
		manifests: map[string]snapshots.Manifest{
			"snap-1": {
				FormatVersion: snapshots.FormatVersion,
				ID:            "snap-1",
				InstanceID:    "instance-1",
				CreatedAt:     time.Now().UTC(),
				Type:          snapshots.TypeAutomatic,
				Reason:        snapshots.ReasonBeforeModUpdate,
				GameVersion:   "1.20",
				Mods:          mods,
			},
		},
		restorable: map[string]bool{"snap-1": true},
	}
	service := newTestService(repository, snapshotsService, &fakeEvents{})

	// The marker references a stale snapshot; the matching manifest wins.
	stale := LastKnownGood{InstanceID: "instance-1", GameVersion: "1.20", SnapshotID: "stale", Mods: mods}
	snapshotID, ok := service.ResolveRecoverySnapshot(context.Background(), "instance-1", stale)
	if !ok || snapshotID != "snap-1" {
		t.Fatalf("expected the matching snapshot to win, got %q (ok=%v)", snapshotID, ok)
	}
}

func TestClearSnapshotReferenceOnlyWhenMatching(t *testing.T) {
	repository := newFakeRepository()
	if err := repository.SaveLastKnownGood(context.Background(), LastKnownGood{
		InstanceID:  "instance-1",
		GameVersion: "1.20",
		SnapshotID:  "snap-1",
	}); err != nil {
		t.Fatal(err)
	}
	service := newTestService(repository, &fakeSnapshots{}, &fakeEvents{})

	service.ClearSnapshotReference(context.Background(), "instance-1", "other")
	marker, err := repository.GetLastKnownGood(context.Background(), "instance-1")
	if err != nil {
		t.Fatal(err)
	}
	if marker.SnapshotID != "snap-1" {
		t.Fatal("clearing a non-matching reference must not touch the marker")
	}

	service.ClearSnapshotReference(context.Background(), "instance-1", "snap-1")
	marker, err = repository.GetLastKnownGood(context.Background(), "instance-1")
	if err != nil {
		t.Fatal(err)
	}
	if marker.SnapshotID != "" {
		t.Fatalf("expected the reference to be cleared, got %q", marker.SnapshotID)
	}
}

func TestProtectedSnapshotID(t *testing.T) {
	repository := newFakeRepository()
	if err := repository.SaveLastKnownGood(context.Background(), LastKnownGood{
		InstanceID:  "instance-1",
		GameVersion: "1.20",
		SnapshotID:  "snap-1",
	}); err != nil {
		t.Fatal(err)
	}
	service := newTestService(repository, &fakeSnapshots{}, &fakeEvents{})
	if protected := service.ProtectedSnapshotID(context.Background(), "instance-1"); protected != "snap-1" {
		t.Fatalf("expected snap-1, got %q", protected)
	}
}

func TestStatusWithoutMarkerIsZero(t *testing.T) {
	service := newTestService(newFakeRepository(), &fakeSnapshots{}, &fakeEvents{})
	status, err := service.Status(context.Background(), "instance-1")
	if err != nil {
		t.Fatal(err)
	}
	if !status.RecordedAt.IsZero() || status.SnapshotExists {
		t.Fatalf("expected a zero status, got %#v", status)
	}
}

func TestStatusReportsChangesAndRecoveryAvailability(t *testing.T) {
	repository := newFakeRepository()
	if err := repository.SaveLastKnownGood(context.Background(), LastKnownGood{
		InstanceID:  "instance-1",
		GameVersion: "1.20",
		SnapshotID:  "snap-1",
	}); err != nil {
		t.Fatal(err)
	}
	snapshotsService := &fakeSnapshots{
		restorable: map[string]bool{"snap-1": true},
		installedMods: map[string][]snapshots.InstalledMod{
			"instance-1": {{Name: "fancy", Version: "2.0.0", Source: "moddb:100:2000", Enabled: true}},
		},
	}
	service := newTestService(repository, snapshotsService, &fakeEvents{})

	status, err := service.Status(context.Background(), "instance-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.MatchesCurrent {
		t.Fatal("a changed configuration must not match the marker")
	}
	if !status.SnapshotExists || status.SnapshotID != "snap-1" {
		t.Fatalf("expected the recovery snapshot to be available: %#v", status)
	}
	if status.ModCount != 0 || status.GameVersion != "1.20" {
		t.Fatalf("unexpected status: %#v", status)
	}
	if errors.Is(err, domain.ErrNotFound) {
		t.Fatal("marker lookup must succeed")
	}
}
