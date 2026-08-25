package wails

import (
	"log/slog"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/snapshots"
)

// SnapshotController exposes instance snapshot management to the frontend.
// It stays limited to DTO conversion and feature invocation.
type SnapshotController struct {
	svc       *snapshots.Service
	lifecycle lifecycle
}

func NewSnapshotController(service *snapshots.Service, lifecycle lifecycle) *SnapshotController {
	return &SnapshotController{svc: service, lifecycle: lifecycle}
}

// CreateInstanceSnapshot captures the current instance data as a restorable backup.
func (controller *SnapshotController) CreateInstanceSnapshot(
	instanceID string,
) (OperationDTO, error) {
	operation, err := controller.svc.Create(
		controller.lifecycle.Context(),
		instanceID,
	)
	return operationDTO(operation), err
}

// ListInstanceSnapshots returns restorable backups for an instance in newest-first order.
func (controller *SnapshotController) ListInstanceSnapshots(
	instanceID string,
) ([]InstanceSnapshotDTO, error) {
	snapshots, err := controller.svc.List(
		controller.lifecycle.Context(),
		instanceID,
	)
	if err != nil {
		return nil, err
	}
	result := make([]InstanceSnapshotDTO, 0, len(snapshots))
	for _, snapshot := range snapshots {
		result = append(result, instanceSnapshotDTO(snapshot))
	}
	return result, nil
}

// RestoreInstanceSnapshot replaces current instance data with the selected backup.
func (controller *SnapshotController) RestoreInstanceSnapshot(
	instanceID string,
	snapshotID string,
) error {
	err := controller.svc.Restore(
		controller.lifecycle.Context(),
		instanceID,
		snapshotID,
	)
	if err != nil {
		slog.Warn("snapshot restore request failed", "instanceId", instanceID, "snapshotId", snapshotID, "error", err)
	}
	return err
}

// DeleteInstanceSnapshot permanently removes a selected instance backup.
func (controller *SnapshotController) DeleteInstanceSnapshot(
	instanceID string,
	snapshotID string,
) error {
	err := controller.svc.Delete(
		controller.lifecycle.Context(),
		instanceID,
		snapshotID,
	)
	if err != nil {
		slog.Warn("snapshot delete request failed", "instanceId", instanceID, "snapshotId", snapshotID, "error", err)
	}
	return err
}
