package launching

import (
	"errors"
	"github.com/waxlight/waxlight-launcher/internal/snapshots"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/mutations"
)

func TestCloneGuardReservesSnapshotMutationSlot(t *testing.T) {
	registry := NewRegistry(mutations.NewSlot())

	release, err := registry.Guard("instance", MutationLockMarker, "Stop the game before cloning this instance")
	if err != nil {
		t.Fatal(err)
	}
	if !registry.Busy("instance") {
		t.Fatal("instance slot should be busy while the guard is held")
	}
	release()
	if registry.Busy("instance") {
		t.Fatal("instance slot should be free after release")
	}
}

func TestCloneGuardRejectsRunningInstance(t *testing.T) {
	registry := NewRegistry(mutations.NewSlot())
	registry.Start("instance", runningGame{})

	if _, err := registry.Guard("instance", MutationLockMarker, "Stop the game before cloning this instance"); !isAppErrorCode(err, instances.ErrInstanceRunning) {
		t.Fatalf("Guard() error = %v", err)
	} else {
		var appError *errs.AppError
		if !errors.As(err, &appError) || appError.Message != "Stop the game before cloning this instance" {
			t.Fatalf("Guard() error = %+v", appError)
		}
	}
}

func TestCloneAndSnapshotReservationsAreMutuallyExclusive(t *testing.T) {
	registry := NewRegistry(mutations.NewSlot())

	cloneRelease, err := registry.Guard("instance", MutationLockMarker, "Stop the game before cloning this instance")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Guard("instance", SnapshotReservationMarker, "Stop the game before modifying this instance"); !isAppErrorCode(err, snapshots.ErrSnapshotInProgress) {
		t.Fatalf("snapshot reservation error = %v", err)
	}
	cloneRelease()

	snapshotRelease, err := registry.Guard("instance", SnapshotReservationMarker, "Stop the game before modifying this instance")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Guard("instance", MutationLockMarker, "Stop the game before cloning this instance"); !isAppErrorCode(err, snapshots.ErrSnapshotInProgress) {
		t.Fatalf("clone reservation error = %v", err)
	}
	snapshotRelease()
}

func TestRegistryLockMessage(t *testing.T) {
	registry := NewRegistry(mutations.NewSlot())
	release, err := registry.Lock("instance", MutationLockMarker)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Lock("instance", MutationLockMarker); !isAppErrorCode(err, snapshots.ErrSnapshotInProgress) {
		t.Fatalf("Lock() error = %v", err)
	} else {
		var appError *errs.AppError
		if !errors.As(err, &appError) || appError.Message != "Wait for the running operation on this instance to finish" {
			t.Fatalf("Lock() error = %+v", appError)
		}
	}
	release()
}

func TestRegistryRunningAndStop(t *testing.T) {
	registry := NewRegistry(mutations.NewSlot())
	if registry.Running("instance") {
		t.Fatal("instance should not be running initially")
	}
	registry.Start("instance", runningGame{})
	if !registry.Running("instance") {
		t.Fatal("instance should be running after Start")
	}
	if got := registry.RunningInstanceIDs(); len(got) != 1 || got[0] != "instance" {
		t.Fatalf("RunningInstanceIDs() = %v", got)
	}
	registry.Stop("instance")
	if registry.Running("instance") {
		t.Fatal("instance should not be running after Stop")
	}
}

func isAppErrorCode(err error, code string) bool {
	var appError *errs.AppError
	return errors.As(err, &appError) && appError.Code == code
}
