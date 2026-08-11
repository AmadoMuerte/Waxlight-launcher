package application

import (
	"context"
	"log/slog"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/operations"
)

// automaticSnapshotRetentionCount is how many automatic snapshots per
// instance are kept. Manual snapshots are never touched by retention.
const automaticSnapshotRetentionCount = 10

// mutationLockMarker is the per-instance busy marker held while a destructive
// operation (or its safety snapshot) is running. It is stored in the same
// per-instance map as snapshot operations so every mutation of an instance is
// mutually exclusive without any process-global blocking.
const mutationLockMarker = "instance-mutation"

// lockInstanceMutations reserves the per-instance mutation slot. The returned
// release function must be called exactly once when the operation finishes.
// A second destructive operation or snapshot for the same instance is
// rejected while the slot is held.
func (s *Service) lockInstanceMutations(instanceID string) (func(), error) {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	if busy := s.snapshotBusy[instanceID]; busy != "" {
		return nil, domain.NewError(
			domain.ErrSnapshotInProgress,
			"Wait for the running operation on this instance to finish",
		)
	}
	s.snapshotBusy[instanceID] = mutationLockMarker
	return func() {
		s.releaseSnapshotBusy(instanceID, mutationLockMarker)
	}, nil
}

// createSafetySnapshot creates one automatic snapshot of the instance right
// before a destructive operation, unless the user disabled automatic safety
// backups. A zero operation is returned when the setting is off; the caller
// must treat a nil error as "no snapshot needed". Any error means the
// snapshot was not created and the destructive operation must not start.
func (s *Service) createSafetySnapshot(
	ctx context.Context,
	instanceID string,
	reason domain.SnapshotReason,
	snapshotContext map[string]string,
) (operations.Operation, error) {
	settings, err := s.settings.Get(ctx)
	if err != nil {
		slog.Warn("could not read the automatic snapshot setting", "instanceId", instanceID, "error", err)
		return operations.Operation{}, err
	}
	if !settings.AutomaticSafetySnapshots {
		slog.Debug("automatic safety snapshots are disabled; skipping the snapshot", "instanceId", instanceID, "reason", reason)
		return operations.Operation{}, nil
	}

	slog.Info("automatic safety snapshot requested", "instanceId", instanceID, "reason", reason)
	operation, err := s.createInstanceSnapshotLocked(ctx, createSnapshotInput{
		instanceID:   instanceID,
		snapshotType: domain.SnapshotTypeAutomatic,
		reason:       reason,
		context:      snapshotContext,
	})
	if err != nil {
		slog.Error("automatic safety snapshot failed; the destructive operation must not start", "instanceId", instanceID, "reason", reason, "error", err)
		return operation, err
	}
	s.enforceAutomaticSnapshotRetention(ctx, instanceID)
	return operation, nil
}

// enforceAutomaticSnapshotRetention keeps only the newest automatic snapshots
// of an instance and deletes older ones. Manual snapshots are never removed.
// The snapshot currently referenced by the Last Known Good marker is protected
// from retention while it is the active recovery snapshot; the next eligible
// automatic snapshot is removed instead. Retention is best-effort: a cleanup
// failure is logged and never invalidates the freshly created snapshot or the
// operation it protects.
func (s *Service) enforceAutomaticSnapshotRetention(ctx context.Context, instanceID string) {
	snapshots, err := s.snapshots.List(ctx, instanceID)
	if err != nil {
		slog.Warn("could not list snapshots for automatic retention", "instanceId", instanceID, "error", err)
		return
	}
	protected := ""
	if lkg, lkgErr := s.store.GetLastKnownGood(ctx, instanceID); lkgErr == nil {
		protected = lkg.SnapshotID
	}
	var automatic []domain.InstanceSnapshot
	for _, snapshot := range snapshots {
		if snapshot.Type == domain.SnapshotTypeAutomatic {
			automatic = append(automatic, snapshot)
		}
	}
	// List returns snapshots newest first.
	if len(automatic) <= automaticSnapshotRetentionCount {
		return
	}
	for _, old := range automatic[automaticSnapshotRetentionCount:] {
		if old.ID == protected {
			slog.Info("automatic retention kept the last known good recovery snapshot", "instanceId", instanceID, "snapshot", old.ID)
			continue
		}
		if removeErr := s.snapshots.Remove(instanceID, old.ID); removeErr != nil {
			slog.Warn("automatic retention could not remove an old snapshot", "instanceId", instanceID, "snapshot", old.ID, "error", removeErr)
			continue
		}
		slog.Info("automatic snapshot removed by retention", "instanceId", instanceID, "snapshot", old.ID, "reason", old.Reason)
	}
}
