package mutations

import "sync"

// Slot coordinates per-instance busy markers so destructive operations,
// snapshots, cloning, and launches of one instance are mutually exclusive
// without any process-global blocking. The marker identifies the operation
// holding the slot; Release only clears a matching marker so a stale release
// can never free a newer operation.
type Slot struct {
	mu   sync.Mutex
	busy map[string]string
}

func NewSlot() *Slot {
	return &Slot{busy: make(map[string]string)}
}

// TryAcquire reserves the slot for marker when the instance is free. The
// returned release function must be called exactly once when the operation
// finishes. When the instance is already busy, nil and the current marker are
// returned without changing anything.
func (slot *Slot) TryAcquire(instanceID string, marker string) (func(), string) {
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if current := slot.busy[instanceID]; current != "" {
		return nil, current
	}
	slot.busy[instanceID] = marker
	return func() {
		slot.Release(instanceID, marker)
	}, ""
}

// Release clears the marker of instanceID only when it matches marker.
func (slot *Slot) Release(instanceID string, marker string) {
	slot.mu.Lock()
	if slot.busy[instanceID] == marker {
		delete(slot.busy, instanceID)
	}
	slot.mu.Unlock()
}

// Set overwrites the marker of instanceID with marker regardless of the
// current holder and returns a release function that only clears a matching
// marker. It is used by restore flows that already hold the reservation slot
// but want the operation ID to become the visible holder.
func (slot *Slot) Set(instanceID string, marker string) func() {
	slot.mu.Lock()
	slot.busy[instanceID] = marker
	slot.mu.Unlock()
	return func() {
		slot.Release(instanceID, marker)
	}
}

// IsBusy reports whether any operation currently holds the instance slot.
func (slot *Slot) IsBusy(instanceID string) bool {
	slot.mu.Lock()
	defer slot.mu.Unlock()
	return slot.busy[instanceID] != ""
}

// BusyMarker returns the marker currently holding the instance slot, or an
// empty string when the instance is free.
func (slot *Slot) BusyMarker(instanceID string) string {
	slot.mu.Lock()
	defer slot.mu.Unlock()
	return slot.busy[instanceID]
}
