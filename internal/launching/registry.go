package launching

import (
	"io"
	"sort"
	"sync"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/mutations"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/process"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/snapshots"
)

const (
	// MutationLockMarker is the per-instance marker held while a destructive
	// mutation (or its safety snapshot) is running.
	MutationLockMarker = "instance-mutation"
	// SnapshotReservationMarker is held while a snapshot operation reserves the
	// instance slot.
	SnapshotReservationMarker = "snapshot-reservation"
)

type runningGame struct {
	process   process.Running
	sessionID string
	started   time.Time
	client    instances.GameClient
	log       io.Closer
	cleanup   func() error
}

// Registry tracks running games and coordinates per-instance launches with
// destructive operations and snapshots. launchMu serializes a launch start
// against guard acquisition so a game cannot start while an instance is being
// modified, and vice versa; mu protects the running-game map; the busy slot
// keeps every mutation of one instance mutually exclusive without
// process-global blocking.
type Registry struct {
	launchMu sync.Mutex
	mu       sync.Mutex
	slot     *mutations.Slot
	running  map[string]runningGame
}

func NewRegistry(slot *mutations.Slot) *Registry {
	return &Registry{slot: slot, running: make(map[string]runningGame)}
}

// BeginLaunch reserves the launch serialization for the whole launch flow. The
// returned release function must be called exactly once when the process has
// been registered as running (or the launch failed).
func (registry *Registry) BeginLaunch() func() {
	registry.launchMu.Lock()
	return registry.launchMu.Unlock
}

// Guard rejects the instance when its game is running and otherwise reserves
// the per-instance busy slot. The returned release function must be called
// exactly once when the operation finishes.
func (registry *Registry) Guard(instanceID string, marker string, runningMessage string) (func(), error) {
	release := registry.BeginLaunch()
	defer release()
	registry.mu.Lock()
	_, running := registry.running[instanceID]
	registry.mu.Unlock()
	if running {
		return nil, errs.NewError(instances.ErrInstanceRunning, runningMessage)
	}
	slotRelease, holder := registry.slot.TryAcquire(instanceID, marker)
	if holder != "" {
		return nil, errs.NewError(snapshots.ErrSnapshotInProgress, "Wait for the running operation on this instance to finish")
	}
	return slotRelease, nil
}

// Lock reserves the per-instance busy slot without a running-game check. It is
// used by mutations that are allowed while the game is not the concern, such
// as mod changes and instance metadata updates. The returned release function
// must be called exactly once when the operation finishes.
func (registry *Registry) Lock(instanceID string, marker string) (func(), error) {
	release, holder := registry.slot.TryAcquire(instanceID, marker)
	if holder != "" {
		return nil, errs.NewError(snapshots.ErrSnapshotInProgress, "Wait for the running operation on this instance to finish")
	}
	return release, nil
}

// Busy reports whether any operation holds the instance slot.
func (registry *Registry) Busy(instanceID string) bool {
	return registry.slot.IsBusy(instanceID)
}

// Running reports whether a game process is running for the instance.
func (registry *Registry) Running(instanceID string) bool {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	_, running := registry.running[instanceID]
	return running
}

func (registry *Registry) ClientRunning(client instances.GameClient) bool {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	for _, game := range registry.running {
		if game.client == client {
			return true
		}
	}
	return false
}

// Start registers a started game process.
func (registry *Registry) Start(instanceID string, game runningGame) {
	registry.mu.Lock()
	registry.running[instanceID] = game
	registry.mu.Unlock()
}

// Get returns the running game of an instance.
func (registry *Registry) Get(instanceID string) (runningGame, bool) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	game, ok := registry.running[instanceID]
	return game, ok
}

// Stop removes the running game of an instance.
func (registry *Registry) Stop(instanceID string) {
	registry.mu.Lock()
	delete(registry.running, instanceID)
	registry.mu.Unlock()
}

// RunningInstanceIDs returns the sorted instance IDs with a running game.
func (registry *Registry) RunningInstanceIDs() []string {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	ids := make([]string, 0, len(registry.running))
	for id := range registry.running {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
