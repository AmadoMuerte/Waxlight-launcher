package presentation

import (
	"log/slog"

	"github.com/waxlight/waxlight-launcher/internal/app"
	"github.com/waxlight/waxlight-launcher/internal/snapshots"
)

type SnapshotController struct {
	svc       *snapshots.Service
	lifecycle *app.Lifecycle
}

func NewSnapshotController(service *snapshots.Service, lifecycle *app.Lifecycle) *SnapshotController {
	return &SnapshotController{svc: service, lifecycle: lifecycle}
}

func (controller *SnapshotController) CreateInstanceSnapshot(
	instanceID string,
) (OperationDTO, error) {
	operation, err := controller.svc.Create(
		controller.lifecycle.Context(),
		instanceID,
	)
	return operationDTO(operation), err
}

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
