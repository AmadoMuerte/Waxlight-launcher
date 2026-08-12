package mutations

import "testing"

func TestSlotTryAcquireAndRelease(t *testing.T) {
	slot := NewSlot()
	release, holder := slot.TryAcquire("instance", "op-1")
	if release == nil || holder != "" {
		t.Fatalf("TryAcquire() = (%T, %q), want release, empty", release, holder)
	}
	if !slot.IsBusy("instance") {
		t.Fatal("slot should be busy after acquisition")
	}
	if got := slot.BusyMarker("instance"); got != "op-1" {
		t.Fatalf("BusyMarker() = %q, want op-1", got)
	}
	release()
	if slot.IsBusy("instance") {
		t.Fatal("slot should be free after release")
	}
}

func TestSlotRejectsSecondAcquirer(t *testing.T) {
	slot := NewSlot()
	release, _ := slot.TryAcquire("instance", "op-1")
	defer release()
	second, holder := slot.TryAcquire("instance", "op-2")
	if second != nil || holder != "op-1" {
		t.Fatalf("TryAcquire() = (%T, %q), want nil, op-1", second, holder)
	}
	if got := slot.BusyMarker("instance"); got != "op-1" {
		t.Fatalf("BusyMarker() = %q, want op-1", got)
	}
}

func TestSlotReleaseIgnoresStaleMarker(t *testing.T) {
	slot := NewSlot()
	_, _ = slot.TryAcquire("instance", "op-1")
	slot.Release("instance", "op-2")
	if !slot.IsBusy("instance") {
		t.Fatal("stale release must not clear the slot")
	}
	slot.Release("instance", "op-1")
	if slot.IsBusy("instance") {
		t.Fatal("matching release must clear the slot")
	}
}

func TestSlotIsPerInstance(t *testing.T) {
	slot := NewSlot()
	release, _ := slot.TryAcquire("one", "op-1")
	defer release()
	second, holder := slot.TryAcquire("two", "op-2")
	if second == nil || holder != "" {
		t.Fatalf("TryAcquire() = (%T, %q), want release, empty", second, holder)
	}
	second()
	if slot.IsBusy("two") {
		t.Fatal("unrelated instance must stay free")
	}
}

func TestSlotReleaseOutsideAcquirer(t *testing.T) {
	slot := NewSlot()
	release, _ := slot.TryAcquire("instance", "op-1")
	slot.Release("instance", "op-1")
	second, holder := slot.TryAcquire("instance", "op-2")
	if second == nil || holder != "" {
		t.Fatalf("TryAcquire() = (%T, %q), want release, empty", second, holder)
	}
	second()
	if release != nil {
		release()
	}
}
