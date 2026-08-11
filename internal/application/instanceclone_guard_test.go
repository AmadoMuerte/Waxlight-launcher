package application

import (
	"errors"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/instances"
)

func TestCloneGuardReservesSnapshotMutationSlot(t *testing.T) {
	service := &Service{
		running:      make(map[string]runningGame),
		snapshotBusy: make(map[string]string),
	}

	release, err := service.guardInstanceClone("instance")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ensureNoSnapshotOperation("instance"); !isAppErrorCode(err, domain.ErrSnapshotInProgress) {
		t.Fatalf("ensureNoSnapshotOperation() error = %v", err)
	}
	release()
	if err := service.ensureNoSnapshotOperation("instance"); err != nil {
		t.Fatalf("reservation survived release: %v", err)
	}
}

func TestCloneGuardRejectsRunningInstance(t *testing.T) {
	service := &Service{
		running:      map[string]runningGame{"instance": {}},
		snapshotBusy: make(map[string]string),
	}

	if _, err := service.guardInstanceClone("instance"); !isAppErrorCode(err, instances.ErrInstanceRunning) {
		t.Fatalf("guardInstanceClone() error = %v", err)
	} else {
		var appError *domain.AppError
		if !errors.As(err, &appError) || appError.Message != "Stop the game before cloning this instance" {
			t.Fatalf("guardInstanceClone() error = %+v", appError)
		}
	}
}

func TestCloneAndSnapshotReservationsAreMutuallyExclusive(t *testing.T) {
	service := &Service{
		running:      make(map[string]runningGame),
		snapshotBusy: make(map[string]string),
	}

	cloneRelease, err := service.guardInstanceClone("instance")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.reserveSnapshotOperation("instance"); !isAppErrorCode(err, domain.ErrSnapshotInProgress) {
		t.Fatalf("snapshot reservation error = %v", err)
	}
	cloneRelease()

	snapshotRelease, err := service.reserveSnapshotOperation("instance")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.guardInstanceClone("instance"); !isAppErrorCode(err, domain.ErrSnapshotInProgress) {
		t.Fatalf("clone reservation error = %v", err)
	}
	snapshotRelease()
}
