package presentation

import (
	"context"
	"log/slog"

	"github.com/waxlight/waxlight-launcher/internal/application"
)

type SnapshotController struct {
	svc *application.Service
}

func NewSnapshotController(service *application.Service) *SnapshotController {
	return &SnapshotController{svc: service}
}

func (controller *SnapshotController) CreateInstanceSnapshot(
	instanceID string,
) (OperationDTO, error) {
	operation, err := controller.svc.CreateInstanceSnapshot(
		context.Background(),
		instanceID,
	)
	return operationDTO(operation), err
}

func (controller *SnapshotController) ListInstanceSnapshots(
	instanceID string,
) ([]InstanceSnapshotDTO, error) {
	snapshots, err := controller.svc.ListInstanceSnapshots(
		context.Background(),
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
	err := controller.svc.RestoreInstanceSnapshot(
		context.Background(),
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
	err := controller.svc.DeleteInstanceSnapshot(
		context.Background(),
		instanceID,
		snapshotID,
	)
	if err != nil {
		slog.Warn("snapshot delete request failed", "instanceId", instanceID, "snapshotId", snapshotID, "error", err)
	}
	return err
}
