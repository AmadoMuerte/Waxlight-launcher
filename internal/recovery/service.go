package recovery

import (
	"context"
	"errors"
	"log/slog"

	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/snapshots"
)

// Service owns Last Known Good recording, failed-launch analysis, recovery
// suggestions, and restore coordination. All dependencies are immutable at
// construction.
type Service struct {
	repository       Repository
	snapshotReader   SnapshotReader
	modConfiguration ModConfiguration
	instanceReader   InstanceReader
	gate             MutationGate
	publisher        Publisher
	now              Clock
}

// NewService builds the recovery service with immutable dependencies.
func NewService(
	repository Repository,
	snapshotReader SnapshotReader,
	modConfiguration ModConfiguration,
	instanceReader InstanceReader,
	gate MutationGate,
	publisher Publisher,
	now Clock,
) *Service {
	return &Service{
		repository:       repository,
		snapshotReader:   snapshotReader,
		modConfiguration: modConfiguration,
		instanceReader:   instanceReader,
		gate:             gate,
		publisher:        publisher,
		now:              now,
	}
}

// RecordLastKnownGood persists the current configuration of an instance as
// the Last Known Good state. It reuses the snapshot manifest representation
// of the installed mods, so the marker and the safety snapshots always agree
// on mod identity. When a restorable snapshot already captures the same
// configuration (typically the automatic safety snapshot taken before the
// changes that led to this launch), the marker references it instead of
// creating any new filesystem backup.
func (s *Service) RecordLastKnownGood(ctx context.Context, instance instances.Instance) {
	release, err := s.beginMutation()
	if err != nil {
		slog.Warn("could not record last known good state during data folder relocation", "instance", instance.Name)
		return
	}
	defer release()
	installedMods, err := s.modConfiguration.ListInstalledMods(ctx, instance.ID)
	if err != nil {
		slog.Warn("could not read the installed mods for the last known good state", "instance", instance.Name, "error", err)
		return
	}
	mods := s.modConfiguration.ModManifest(ctx, installedMods)
	gameVersion := s.modConfiguration.GameVersionName(ctx, instance.GameVersionID)

	previous, previousErr := s.repository.GetLastKnownGood(ctx, instance.ID)
	replaced := previousErr == nil

	lkg := LastKnownGood{
		InstanceID:  instance.ID,
		RecordedAt:  s.now().UTC(),
		GameVersion: gameVersion,
		Mods:        mods,
	}
	if snapshotID, linkErr := s.findMatchingSnapshot(ctx, instance.ID, gameVersion, mods); linkErr == nil {
		lkg.SnapshotID = snapshotID
	}
	if err := s.repository.SaveLastKnownGood(ctx, lkg); err != nil {
		slog.Warn("could not persist the last known good state", "instance", instance.Name, "error", err)
		return
	}
	s.publish("last-known-good:updated", map[string]string{"instanceId": instance.ID})
	if replaced {
		slog.Info("last known good replaced", "instance", instance.Name, "gameVersion", gameVersion, "mods", len(mods), "snapshot", lkg.SnapshotID != "")
	} else {
		slog.Info("last known good recorded", "instance", instance.Name, "gameVersion", gameVersion, "mods", len(mods), "snapshot", lkg.SnapshotID != "")
	}
	if previousErr == nil && previous.SnapshotID != "" && previous.SnapshotID != lkg.SnapshotID {
		slog.Info("previous last known good snapshot released from recovery protection", "instance", instance.Name, "snapshot", previous.SnapshotID)
	}
}

// findMatchingSnapshot returns the newest snapshot of an instance whose
// manifest captures exactly the given configuration, or "" when none exists.
// Snapshots that cannot be restored automatically are never linked.
func (s *Service) findMatchingSnapshot(ctx context.Context, instanceID, gameVersion string, mods []snapshots.Mod) (string, error) {
	listed, err := s.snapshotReader.List(ctx, instanceID)
	if err != nil {
		return "", err
	}
	for _, snapshot := range listed {
		manifest, readErr := s.snapshotReader.ReadManifest(ctx, instanceID, snapshot.ID)
		if readErr != nil || manifest.InstanceID != instanceID {
			continue
		}
		if manifest.GameVersion != gameVersion || !snapshots.SameModSet(manifest.Mods, mods) {
			continue
		}
		if snapshots.ValidateMods(manifest.Mods) != nil {
			continue
		}
		return snapshot.ID, nil
	}
	return "", nil
}

// currentConfiguration resolves the live instance state in the same exact
// release representation the snapshot manifest uses. The returned name map
// keys mod identity to the display name of the installed mod record.
func (s *Service) currentConfiguration(ctx context.Context, instance instances.Instance) (string, []snapshots.Mod, map[string]string, error) {
	installedMods, err := s.modConfiguration.ListInstalledMods(ctx, instance.ID)
	if err != nil {
		return "", nil, nil, err
	}
	mods := s.modConfiguration.ModManifest(ctx, installedMods)
	names := make(map[string]string, len(installedMods))
	for _, mod := range installedMods {
		names[installedModKey(mod)] = mod.Name
	}
	return s.modConfiguration.GameVersionName(ctx, instance.GameVersionID), mods, names, nil
}

// HandleFailedLaunch assesses a startup failure against the Last Known Good
// state and emits a recovery suggestion when the configuration changed. It
// never rolls anything back and never claims a specific mod caused the crash.
func (s *Service) HandleFailedLaunch(instance instances.Instance) {
	ctx := context.Background()
	lkg, err := s.repository.GetLastKnownGood(ctx, instance.ID)
	if err != nil {
		slog.Info("launch considered failed; no last known good state exists", "instance", instance.Name)
		return
	}
	currentGameVersion, currentMods, currentNames, configErr := s.currentConfiguration(ctx, instance)
	if configErr != nil {
		slog.Warn("could not compare the failed launch against the last known good state", "instance", instance.Name, "error", configErr)
		return
	}
	changes := compareConfigurations(lkg, currentMods, currentNames, currentGameVersion)
	if changes.Empty() {
		slog.Info("launch considered failed; configuration unchanged since the last known good state", "instance", instance.Name)
		return
	}

	snapshotID, snapshotExists := s.ResolveRecoverySnapshot(ctx, instance.ID, lkg)
	suggestion := RecoverySuggestion{
		InstanceID:     instance.ID,
		RecordedAt:     lkg.RecordedAt,
		SnapshotID:     snapshotID,
		SnapshotExists: snapshotExists,
		Changes:        changes,
		StateSignature: configurationSignature(currentMods, currentGameVersion),
	}
	s.publish("game:recovery-suggestion", suggestion)
	if snapshotExists {
		slog.Info("recovery candidate found", "instance", instance.Name, "changes", changes.Count(), "snapshot", snapshotID)
	} else {
		slog.Info("recovery snapshot unavailable; showing changes only", "instance", instance.Name, "changes", changes.Count())
	}
}

// ResolveRecoverySnapshot returns the snapshot that captures the Last Known
// Good state and can be restored automatically. The marker's own reference is
// preferred; when it is missing or stale, the newest snapshot whose manifest
// matches the Last Known Good state is used, so a safety snapshot created
// after the marker was recorded still enables one-click recovery.
func (s *Service) ResolveRecoverySnapshot(ctx context.Context, instanceID string, lkg LastKnownGood) (string, bool) {
	if lkg.SnapshotID != "" && s.snapshotReader.IsRestorable(ctx, instanceID, lkg.SnapshotID) {
		return lkg.SnapshotID, true
	}
	snapshotID, err := s.findMatchingSnapshot(ctx, instanceID, lkg.GameVersion, lkg.Mods)
	if err != nil || snapshotID == "" {
		return "", false
	}
	return snapshotID, true
}

// ClearSnapshotReference drops the snapshot reference of the Last Known Good
// marker after the referenced snapshot is deleted. The metadata stays intact
// so change comparison keeps working without one-click recovery.
func (s *Service) ClearSnapshotReference(ctx context.Context, instanceID, snapshotID string) {
	release, gateErr := s.beginMutation()
	if gateErr != nil {
		slog.Warn("could not clear last known good snapshot during data folder relocation", "instanceId", instanceID)
		return
	}
	defer release()
	lkg, err := s.repository.GetLastKnownGood(ctx, instanceID)
	if err != nil || lkg.SnapshotID != snapshotID {
		return
	}
	lkg.SnapshotID = ""
	if err := s.repository.SaveLastKnownGood(ctx, lkg); err != nil {
		slog.Warn("could not clear the last known good snapshot reference", "instanceId", instanceID, "snapshot", snapshotID, "error", err)
		return
	}
	slog.Info("last known good snapshot reference cleared", "instanceId", instanceID, "snapshot", snapshotID)
}

// ProtectedSnapshotID returns the snapshot referenced by the Last Known Good
// marker so automatic retention never removes the active recovery snapshot.
func (s *Service) ProtectedSnapshotID(ctx context.Context, instanceID string) string {
	lkg, err := s.repository.GetLastKnownGood(ctx, instanceID)
	if err != nil {
		return ""
	}
	return lkg.SnapshotID
}

// Status returns the Last Known Good marker of an instance together with the
// live comparison against the current configuration. A zero status with no
// error is returned when no marker was recorded yet.
func (s *Service) Status(ctx context.Context, instanceID string) (LastKnownGoodStatus, error) {
	if _, err := s.instanceReader.GetInstance(ctx, instanceID); err != nil {
		return LastKnownGoodStatus{}, err
	}
	lkg, err := s.repository.GetLastKnownGood(ctx, instanceID)
	if errors.Is(err, errs.ErrNotFound) {
		return LastKnownGoodStatus{}, nil
	}
	if err != nil {
		return LastKnownGoodStatus{}, err
	}
	instance, err := s.instanceReader.GetInstance(ctx, instanceID)
	if err != nil {
		return LastKnownGoodStatus{}, err
	}
	currentGameVersion, currentMods, currentNames, configErr := s.currentConfiguration(ctx, instance)
	if configErr != nil {
		return LastKnownGoodStatus{}, configErr
	}
	changes := compareConfigurations(lkg, currentMods, currentNames, currentGameVersion)
	snapshotID, snapshotExists := s.ResolveRecoverySnapshot(ctx, instanceID, lkg)
	return LastKnownGoodStatus{
		RecordedAt:     lkg.RecordedAt,
		GameVersion:    lkg.GameVersion,
		ModCount:       len(lkg.Mods),
		SnapshotID:     snapshotID,
		SnapshotExists: snapshotExists,
		MatchesCurrent: changes.Empty(),
		Changes:        changes,
	}, nil
}

func (s *Service) publish(name string, payload any) {
	if s.publisher != nil {
		s.publisher.Publish(name, payload)
	}
}

// beginMutation coordinates launcher-wide writes with data-root relocation.
func (s *Service) beginMutation() (func(), error) {
	if err := s.gate.Begin(); err != nil {
		return nil, err
	}
	return s.gate.End, nil
}
