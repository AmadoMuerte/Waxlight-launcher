package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

// gameStartupWindow is how long a game process must survive for its launch to
// be considered successful. Exits after this window are treated as a started
// game (a long play session followed by a crash is not a failed startup); a
// crashed exit inside the window is a failed startup. It is a variable so
// tests can shorten it. Launch snapshots the value once and hands it to the
// goroutines, so tests may restore it without racing background readers.
var gameStartupWindow = 60 * time.Second

// markLaunchEstablished records the Last Known Good state once a game process
// survives the startup window. The timer never fires for short-lived crashes
// because waitForGame removes the running entry when the process exits.
// startupWindow is the value captured by Launch so this goroutine never reads
// the mutable package variable.
func (s *Service) markLaunchEstablished(instance domain.Instance, sessionID string, startupWindow time.Duration) {
	timer := time.NewTimer(startupWindow)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-s.shutdownCtx.Done():
		return
	}
	s.runningMu.Lock()
	running, ok := s.running[instance.ID]
	s.runningMu.Unlock()
	if !ok || running.sessionID != sessionID {
		return
	}
	slog.Info("launch considered successful", "instance", instance.Name, "startupWindow", startupWindow.String())
	s.recordLastKnownGood(context.Background(), instance)
}

// recordEstablishedLaunches records the Last Known Good state of every game
// that was still running past the startup window when the launcher shut down.
// The game itself keeps running, so its configuration is a working one.
func (s *Service) recordEstablishedLaunches() {
	s.runningMu.Lock()
	var established []string
	for id, running := range s.running {
		if time.Since(running.started) >= gameStartupWindow {
			established = append(established, id)
		}
	}
	s.runningMu.Unlock()
	for _, instanceID := range established {
		instance, err := s.store.GetInstance(context.Background(), instanceID)
		if err != nil {
			slog.Warn("could not read an instance for the last known good state", "instanceId", instanceID, "error", err)
			continue
		}
		slog.Info("launch considered successful", "instance", instance.Name, "launcherShutdown", true)
		s.recordLastKnownGood(context.Background(), instance)
	}
}

// recordLastKnownGood persists the current configuration of an instance as the
// Last Known Good state. It reuses the snapshot manifest representation of the
// installed mods, so the marker and the safety snapshots always agree on mod
// identity. When a restorable snapshot already captures the same configuration
// (typically the automatic safety snapshot taken before the changes that led
// to this launch), the marker references it instead of creating any new
// filesystem backup.
func (s *Service) recordLastKnownGood(ctx context.Context, instance domain.Instance) {
	installedMods, err := s.ListMods(ctx, instance.ID)
	if err != nil {
		slog.Warn("could not read the installed mods for the last known good state", "instance", instance.Name, "error", err)
		return
	}
	mods, _ := s.snapshotModManifest(ctx, instance.ID, installedMods)
	gameVersion := s.instanceGameVersionName(ctx, instance)

	previous, previousErr := s.store.GetLastKnownGood(ctx, instance.ID)
	replaced := previousErr == nil

	lkg := domain.LastKnownGood{
		InstanceID:  instance.ID,
		RecordedAt:  time.Now().UTC(),
		GameVersion: gameVersion,
		Mods:        mods,
	}
	if snapshotID, linkErr := s.findMatchingSnapshot(ctx, instance.ID, gameVersion, mods); linkErr == nil {
		lkg.SnapshotID = snapshotID
	}
	if err := s.store.SaveLastKnownGood(ctx, lkg); err != nil {
		slog.Warn("could not persist the last known good state", "instance", instance.Name, "error", err)
		return
	}
	s.emit("last-known-good:updated", map[string]string{"instanceId": instance.ID})
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
func (s *Service) findMatchingSnapshot(ctx context.Context, instanceID, gameVersion string, mods []domain.SnapshotMod) (string, error) {
	snapshots, err := s.snapshots.List(ctx, instanceID)
	if err != nil {
		return "", err
	}
	for _, snapshot := range snapshots {
		dir, dirErr := s.snapshots.SnapshotDir(instanceID, snapshot.ID)
		if dirErr != nil {
			continue
		}
		manifest, readErr := s.snapshots.ReadManifest(dir)
		if readErr != nil || manifest.InstanceID != instanceID {
			continue
		}
		if manifest.GameVersion != gameVersion || !sameModSet(manifest.Mods, mods) {
			continue
		}
		if validateSnapshotMods(manifest.Mods) != nil {
			continue
		}
		return snapshot.ID, nil
	}
	return "", nil
}

// sameModSet reports whether two snapshot mod lists describe the same
// configuration: the same mod identities with the same versions.
func sameModSet(left, right []domain.SnapshotMod) bool {
	if len(left) != len(right) {
		return false
	}
	byKey := make(map[string]string, len(left))
	for _, mod := range left {
		byKey[snapshotModKey(mod)] = mod.Version
	}
	for _, mod := range right {
		if version, ok := byKey[snapshotModKey(mod)]; !ok || version != mod.Version {
			return false
		}
	}
	return true
}

// currentConfiguration resolves the live instance state in the same exact
// release representation the snapshot manifest uses. The returned name map
// keys mod identity to the display name of the installed mod record.
func (s *Service) currentConfiguration(ctx context.Context, instance domain.Instance) (string, []domain.SnapshotMod, map[string]string, error) {
	installedMods, err := s.ListMods(ctx, instance.ID)
	if err != nil {
		return "", nil, nil, err
	}
	mods, _ := s.snapshotModManifest(ctx, instance.ID, installedMods)
	names := make(map[string]string, len(installedMods))
	for _, mod := range installedMods {
		names[installedModKey(mod)] = mod.Name
	}
	return s.instanceGameVersionName(ctx, instance), mods, names, nil
}

// snapshotModKey derives the stable identity of a snapshot mod entry. ModDB
// mods are identified by their catalog ID, never by filename, so an updated
// release still compares as the same mod. Manually installed mods fall back to
// their file name because they have no catalog identity.
func snapshotModKey(mod domain.SnapshotMod) string {
	if mod.Source == domain.SnapshotModSourceModDB {
		return "moddb:" + strings.TrimSpace(mod.ModID)
	}
	name := strings.TrimSpace(mod.Identifier)
	if name == "" {
		name = strings.TrimSpace(mod.FileName)
	}
	return "local:" + name
}

// installedModKey derives the stable identity of an installed mod record,
// matching snapshotModKey for the same mod.
func installedModKey(mod domain.InstalledMod) string {
	if modID, _, ok := parseModDBSource(mod.Source); ok {
		return "moddb:" + modID
	}
	name := strings.TrimSpace(mod.Name)
	if name == "" {
		name = strings.TrimSpace(mod.FileName)
	}
	return "local:" + name
}

// compareConfigurations reports the facts that differ between the Last Known
// Good state and the current instance state. It never attributes the
// differences to a cause.
func compareConfigurations(
	lkg domain.LastKnownGood,
	currentMods []domain.SnapshotMod,
	currentNames map[string]string,
	currentGameVersion string,
) domain.ConfigurationChanges {
	changes := domain.ConfigurationChanges{
		Updated: []domain.ModChange{},
		Added:   []domain.ModChange{},
		Removed: []domain.ModChange{},
	}
	if lkg.GameVersion != currentGameVersion {
		changes.GameVersionFrom = lkg.GameVersion
		changes.GameVersionTo = currentGameVersion
	}

	lkgByKey := make(map[string]domain.SnapshotMod, len(lkg.Mods))
	for _, mod := range lkg.Mods {
		lkgByKey[snapshotModKey(mod)] = mod
	}
	currentByKey := make(map[string]domain.SnapshotMod, len(currentMods))
	for _, mod := range currentMods {
		currentByKey[snapshotModKey(mod)] = mod
	}

	for key, mod := range currentByKey {
		previous, ok := lkgByKey[key]
		if !ok {
			changes.Added = append(changes.Added, domain.ModChange{
				Name: configurationModName(key, currentNames, mod),
				To:   mod.Version,
			})
			continue
		}
		if previous.Version != mod.Version {
			changes.Updated = append(changes.Updated, domain.ModChange{
				Name: configurationModName(key, currentNames, mod),
				From: previous.Version,
				To:   mod.Version,
			})
		}
	}
	for key, mod := range lkgByKey {
		if _, ok := currentByKey[key]; !ok {
			changes.Removed = append(changes.Removed, domain.ModChange{
				Name: snapshotModDisplayName(mod),
				From: mod.Version,
			})
		}
	}

	sortModChanges(changes.Updated)
	sortModChanges(changes.Added)
	sortModChanges(changes.Removed)
	return changes
}

func configurationModName(key string, names map[string]string, mod domain.SnapshotMod) string {
	if name := names[key]; strings.TrimSpace(name) != "" {
		return name
	}
	return snapshotModDisplayName(mod)
}

func sortModChanges(changes []domain.ModChange) {
	sort.Slice(changes, func(left, right int) bool {
		return strings.ToLower(changes[left].Name) < strings.ToLower(changes[right].Name)
	})
}

// configurationSignature is a stable fingerprint of an instance configuration
// (game version and exact mod releases). The frontend uses it to suppress
// repeat recovery prompts for the same failed state.
func configurationSignature(mods []domain.SnapshotMod, gameVersion string) string {
	hash := sha256.New()
	hash.Write([]byte(gameVersion))
	entries := make([]string, 0, len(mods))
	for _, mod := range mods {
		entries = append(entries, snapshotModKey(mod)+"::"+mod.Version)
	}
	sort.Strings(entries)
	for _, entry := range entries {
		hash.Write([]byte{0})
		hash.Write([]byte(entry))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// handleFailedLaunch assesses a startup failure against the Last Known Good
// state and emits a recovery suggestion when the configuration changed. It
// never rolls anything back and never claims a specific mod caused the crash.
func (s *Service) handleFailedLaunch(instance domain.Instance) {
	ctx := context.Background()
	lkg, err := s.store.GetLastKnownGood(ctx, instance.ID)
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

	snapshotID, snapshotExists := s.resolveRecoverySnapshot(ctx, instance.ID, lkg)
	suggestion := domain.RecoverySuggestion{
		InstanceID:     instance.ID,
		RecordedAt:     lkg.RecordedAt,
		SnapshotID:     snapshotID,
		SnapshotExists: snapshotExists,
		Changes:        changes,
		StateSignature: configurationSignature(currentMods, currentGameVersion),
	}
	s.emit("game:recovery-suggestion", suggestion)
	if snapshotExists {
		slog.Info("recovery candidate found", "instance", instance.Name, "changes", changes.Count(), "snapshot", snapshotID)
	} else {
		slog.Info("recovery snapshot unavailable; showing changes only", "instance", instance.Name, "changes", changes.Count())
	}
}

// resolveRecoverySnapshot returns the snapshot that captures the Last Known
// Good state and can be restored automatically. The marker's own reference is
// preferred; when it is missing or stale, the newest snapshot whose manifest
// matches the Last Known Good state is used, so a safety snapshot created
// after the marker was recorded still enables one-click recovery.
func (s *Service) resolveRecoverySnapshot(ctx context.Context, instanceID string, lkg domain.LastKnownGood) (string, bool) {
	if lkg.SnapshotID != "" && s.snapshotIsRestorable(ctx, instanceID, lkg.SnapshotID) {
		return lkg.SnapshotID, true
	}
	snapshotID, err := s.findMatchingSnapshot(ctx, instanceID, lkg.GameVersion, lkg.Mods)
	if err != nil || snapshotID == "" {
		return "", false
	}
	return snapshotID, true
}

// snapshotIsRestorable reports whether a snapshot belongs to the instance and
// can be restored automatically.
func (s *Service) snapshotIsRestorable(ctx context.Context, instanceID, snapshotID string) bool {
	dir, err := s.snapshots.SnapshotDir(instanceID, snapshotID)
	if err != nil {
		return false
	}
	manifest, err := s.snapshots.ReadManifest(dir)
	if err != nil || manifest.InstanceID != instanceID {
		return false
	}
	return validateSnapshotMods(manifest.Mods) == nil
}

// ClearLastKnownGoodSnapshotReference drops the snapshot reference of the Last
// Known Good marker after the referenced snapshot is deleted. The metadata
// stays intact so change comparison keeps working without one-click recovery.
func (s *Service) ClearLastKnownGoodSnapshotReference(ctx context.Context, instanceID, snapshotID string) {
	lkg, err := s.store.GetLastKnownGood(ctx, instanceID)
	if err != nil || lkg.SnapshotID != snapshotID {
		return
	}
	lkg.SnapshotID = ""
	if err := s.store.SaveLastKnownGood(ctx, lkg); err != nil {
		slog.Warn("could not clear the last known good snapshot reference", "instanceId", instanceID, "snapshot", snapshotID, "error", err)
		return
	}
	slog.Info("last known good snapshot reference cleared", "instanceId", instanceID, "snapshot", snapshotID)
}

// GetLastKnownGoodStatus returns the Last Known Good marker of an instance
// together with the live comparison against the current configuration. A zero
// status with no error is returned when no marker was recorded yet.
func (s *Service) GetLastKnownGoodStatus(ctx context.Context, instanceID string) (domain.LastKnownGoodStatus, error) {
	if _, err := s.store.GetInstance(ctx, instanceID); err != nil {
		return domain.LastKnownGoodStatus{}, err
	}
	lkg, err := s.store.GetLastKnownGood(ctx, instanceID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.LastKnownGoodStatus{}, nil
	}
	if err != nil {
		return domain.LastKnownGoodStatus{}, err
	}
	instance, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return domain.LastKnownGoodStatus{}, err
	}
	currentGameVersion, currentMods, currentNames, configErr := s.currentConfiguration(ctx, instance)
	if configErr != nil {
		return domain.LastKnownGoodStatus{}, configErr
	}
	changes := compareConfigurations(lkg, currentMods, currentNames, currentGameVersion)
	snapshotID, snapshotExists := s.resolveRecoverySnapshot(ctx, instanceID, lkg)
	return domain.LastKnownGoodStatus{
		RecordedAt:     lkg.RecordedAt,
		GameVersion:    lkg.GameVersion,
		ModCount:       len(lkg.Mods),
		SnapshotID:     snapshotID,
		SnapshotExists: snapshotExists,
		MatchesCurrent: changes.Empty(),
		Changes:        changes,
	}, nil
}
