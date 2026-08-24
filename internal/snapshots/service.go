package snapshots

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/operations"
)

const (
	snapshotProgressInterval  = 250 * time.Millisecond
	instanceMarkerFile        = ".waxlight-instance"
	snapshotModDownloadLimit  = 4
	restoreDownloadPhaseStart = 0.4
)

// automaticRetentionCount is how many automatic snapshots per instance are
// kept. Manual snapshots are never touched by retention.
const automaticRetentionCount = 10

// createInput carries the metadata of a snapshot being created. The type,
// reason and context distinguish manual snapshots from automatic safety
// snapshots in the manifest; manual creation leaves them at their zero values.
type createInput struct {
	instanceID   string
	snapshotType Type
	reason       Reason
	context      map[string]string
}

// Service owns manual snapshots, automatic safety snapshots, restore
// coordination, pruning, and exact managed-mod restoration. All dependencies
// are immutable at construction.
type Service struct {
	storage             Storage
	instances           InstanceReader
	versions            VersionReader
	mods                ModStore
	catalog             Catalog
	archiveInfo         ArchiveInfoReader
	settings            SettingsReader
	operations          *operations.Manager
	gate                MutationGate
	slot                InstanceSlot
	lock                InstanceLock
	diskSpace           DiskSpaceChecker
	totalSize           TotalSizeFunc
	sanitizeSettings    ClientSettingsSanitizer
	hardenLogs          LogsHardener
	clearClientSettings ClientSettingsClearer
	removeDirectory     DirectoryRemover
	clearLastKnownGood  LastKnownGoodReference
	dataRoot            string
	now                 Clock
	newID               IDGenerator
}

// NewService builds the snapshot service with immutable dependencies.
func NewService(
	storage Storage,
	instances InstanceReader,
	versions VersionReader,
	mods ModStore,
	catalog Catalog,
	archiveInfo ArchiveInfoReader,
	settings SettingsReader,
	operationsManager *operations.Manager,
	gate MutationGate,
	slot InstanceSlot,
	lock InstanceLock,
	diskSpace DiskSpaceChecker,
	totalSize TotalSizeFunc,
	sanitizeSettings ClientSettingsSanitizer,
	hardenLogs LogsHardener,
	clearClientSettings ClientSettingsClearer,
	removeDirectory DirectoryRemover,
	clearLastKnownGood LastKnownGoodReference,
	dataRoot string,
	now Clock,
	newID IDGenerator,
) *Service {
	return &Service{
		storage:             storage,
		instances:           instances,
		versions:            versions,
		mods:                mods,
		catalog:             catalog,
		archiveInfo:         archiveInfo,
		settings:            settings,
		operations:          operationsManager,
		gate:                gate,
		slot:                slot,
		lock:                lock,
		diskSpace:           diskSpace,
		totalSize:           totalSize,
		sanitizeSettings:    sanitizeSettings,
		hardenLogs:          hardenLogs,
		clearClientSettings: clearClientSettings,
		removeDirectory:     removeDirectory,
		clearLastKnownGood:  clearLastKnownGood,
		dataRoot:            dataRoot,
		now:                 now,
		newID:               newID,
	}
}

// Create captures the current user data of an instance into a new manual
// snapshot. It guards against running games and concurrent snapshot
// operations.
func (s *Service) Create(ctx context.Context, instanceID string) (operations.Operation, error) {
	return s.create(ctx, createInput{
		instanceID:   instanceID,
		snapshotType: TypeManual,
	})
}

// CreateSafety creates one automatic snapshot of the instance right before a
// destructive operation, unless the user disabled automatic safety backups. A
// zero operation is returned when the setting is off; the caller must treat a
// nil error as "no snapshot needed". Any error means the snapshot was not
// created and the destructive operation must not start.
func (s *Service) CreateSafety(
	ctx context.Context,
	instanceID string,
	reason Reason,
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
	operation, err := s.createLocked(ctx, createInput{
		instanceID:   instanceID,
		snapshotType: TypeAutomatic,
		reason:       reason,
		context:      snapshotContext,
	})
	if err != nil {
		slog.Error("automatic safety snapshot failed; the destructive operation must not start", "instanceId", instanceID, "reason", reason, "error", err)
		return operation, err
	}
	s.enforceRetention(ctx, instanceID)
	return operation, nil
}

// instanceRunningCode is the error code returned when an instance must not be
// touched while its game is running. It must match instances.ErrInstanceRunning.
const instanceRunningCode = "INSTANCE_ALREADY_RUNNING"

// createLocked creates a snapshot while the caller already holds the
// per-instance mutation lock, so only the game-running rule still applies.
// Automatic safety snapshots use this path.
func (s *Service) createLocked(ctx context.Context, input createInput) (operations.Operation, error) {
	release, err := s.beginMutation()
	if err != nil {
		return operations.Operation{}, err
	}
	defer release()
	if s.lock.Running(input.instanceID) {
		return operations.Operation{}, errs.NewError(instanceRunningCode, "Stop the game before modifying this instance")
	}
	return s.createCore(ctx, input)
}

// create guards a manual snapshot creation against running games and
// concurrent snapshot operations before delegating to the shared creation
// core.
func (s *Service) create(ctx context.Context, input createInput) (operations.Operation, error) {
	release, err := s.beginMutation()
	if err != nil {
		return operations.Operation{}, err
	}
	defer release()
	reservationRelease, err := s.lock.Guard(input.instanceID, ReservationMarker, "Stop the game before modifying this instance")
	if err != nil {
		return operations.Operation{}, err
	}
	defer reservationRelease()
	return s.createCore(ctx, input)
}

// beginMutation coordinates launcher-wide writes with data-root relocation.
func (s *Service) beginMutation() (func(), error) {
	if err := s.gate.Begin(); err != nil {
		return nil, err
	}
	return s.gate.End, nil
}

// createCore captures the current user data of an instance into a new
// snapshot. Waxlight-managed ModDB mod binaries are not copied; the manifest
// records their exact releases so restore can download the same versions
// again. The copy is staged in a temporary directory first, the manifest is
// written and validated, and only then the staging directory is atomically
// renamed into place. A snapshot is therefore never visible half-written, and
// the source instance is never modified.
func (s *Service) createCore(ctx context.Context, input createInput) (operations.Operation, error) {
	instance, err := s.instances.GetInstance(ctx, input.instanceID)
	if err != nil {
		return operations.Operation{}, err
	}

	installedMods, err := s.mods.ListMods(ctx, input.instanceID)
	if err != nil {
		return operations.Operation{}, err
	}
	manifestMods, skipPaths := s.modManifest(ctx, installedMods)

	estimated, err := s.totalSize(ctx, instance.Directory)
	if err != nil {
		return operations.Operation{}, &errs.AppError{
			Code:    errs.ErrFilePermission,
			Message: "Could not read the instance files",
			Cause:   err,
		}
	}
	for path := range skipPaths {
		if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
			estimated -= info.Size()
		}
	}
	if err := s.ensureSpace(estimated); err != nil {
		return operations.Operation{}, err
	}

	now := s.now().UTC()
	resource := instance.ID
	operation := operations.Operation{
		ID:         s.newID(),
		Type:       "snapshot_create",
		ResourceID: &resource,
		Title:      "Creating snapshot",
		TitleKey:   TitleCreatingSnapshot,
		Status:     operations.StatusRunning,
		Progress:   0,
		TotalBytes: estimated,
		CreatedAt:  now,
		StartedAt:  &now,
	}
	if input.snapshotType == TypeAutomatic {
		operation.Title = "Creating safety backup..."
		operation.TitleKey = TitleCreatingSafetyBackup
	}
	if err := s.operations.Save(ctx, operation, operations.EventCreated); err != nil {
		slog.Warn("could not persist the snapshot operation", "operationId", operation.ID, "error", err)
	}

	slotRelease, holder := s.slot.TryAcquire(instance.ID, operation.ID)
	if holder == "" && slotRelease != nil {
		defer slotRelease()
	}

	var staging string
	defer func() {
		// A staging directory that was never renamed into place is a failed
		// creation; remove it so no half-written snapshot stays on disk.
		if staging != "" {
			_ = os.RemoveAll(staging)
		}
	}()

	fail := func(cause error, code string) (operations.Operation, error) {
		if staging != "" {
			if cleanupErr := os.RemoveAll(staging); cleanupErr != nil {
				slog.Warn("could not remove the failed snapshot staging directory", "instanceId", instance.ID, "error", cleanupErr)
			}
		}
		s.finishOperation(operation, cause, code)
		message := "Could not create snapshot"
		if input.snapshotType == TypeAutomatic {
			message = "Could not create a safety backup. The instance was not modified"
		}
		return operation, &errs.AppError{
			Code:    code,
			Message: message,
			Cause:   cause,
		}
	}

	staging, err = s.storage.TempDir(instance.ID)
	if err != nil {
		return fail(err, errs.ErrFilePermission)
	}
	final, err := s.storage.SnapshotDir(instance.ID, operation.ID)
	if err != nil {
		return fail(err, errs.ErrValidation)
	}

	snapshotID := operation.ID
	stats, err := copySnapshotData(
		ctx,
		instance.Directory,
		s.storage.DataDir(staging),
		skipPaths,
		operationProgress(s, &operation),
		s.sanitizeSettings,
	)
	if err != nil {
		return fail(err, errs.ErrFilePermission)
	}

	manifest := Manifest{
		FormatVersion: FormatVersion,
		ID:            snapshotID,
		InstanceID:    instance.ID,
		InstanceName:  instance.Name,
		CreatedAt:     now,
		Type:          input.snapshotType,
		Reason:        input.reason,
		Context:       input.context,
		GameVersion:   s.GameVersionName(ctx, instance.GameVersionID),
		SizeBytes:     stats.sizeBytes,
		ModCount:      len(installedMods),
		WorldCount:    stats.worldCount,
		Mods:          manifestMods,
	}
	if err := s.storage.WriteManifest(staging, manifest); err != nil {
		return fail(err, errs.ErrFilePermission)
	}
	if _, err := s.storage.ReadManifest(staging); err != nil {
		return fail(err, ErrSnapshotInvalid)
	}
	if err := os.Rename(staging, final); err != nil {
		return fail(err, errs.ErrFilePermission)
	}
	staging = ""

	finished := s.now().UTC()
	operation.FinishedAt = &finished
	operation.Status = operations.StatusCompleted
	operation.Progress = 1
	operation.CurrentBytes = stats.sizeBytes
	s.operations.SaveBestEffort(operation, operations.EventCompleted)
	if input.snapshotType == TypeAutomatic {
		slog.Info("automatic safety snapshot created", "instance", instance.Name, "snapshot", snapshotID, "reason", input.reason, "size", stats.sizeBytes, "mods", len(installedMods))
	} else {
		slog.Info("instance snapshot created", "instance", instance.Name, "snapshot", snapshotID, "size", stats.sizeBytes, "mods", len(installedMods))
	}
	return operation, nil
}

// ModManifest maps the installed mods of an instance to manifest entries.
// Managed ModDB mods are enriched with the cached release metadata when it is
// still available; manual mods are recorded without a downloadable source.
// The manifest never stores credentials.
func (s *Service) ModManifest(ctx context.Context, installedMods []InstalledMod) []Mod {
	mods, _ := s.modManifest(ctx, installedMods)
	return mods
}

// ListInstalledMods lists the installed mods of an instance for manifest
// mapping.
func (s *Service) ListInstalledMods(ctx context.Context, instanceID string) ([]InstalledMod, error) {
	return s.mods.ListMods(ctx, instanceID)
}

// modManifest maps the installed mods of an instance to manifest entries and
// collects the file paths that must be skipped during the data copy. Managed
// ModDB mods are enriched with the cached release metadata when it is still
// available; manual mods are recorded without a downloadable source. The
// manifest never stores credentials.
func (s *Service) modManifest(ctx context.Context, installedMods []InstalledMod) ([]Mod, map[string]struct{}) {
	manifestMods := make([]Mod, 0, len(installedMods))
	skipPaths := make(map[string]struct{}, len(installedMods))
	for _, mod := range installedMods {
		if modID, versionID, ok := ParseModDBSource(mod.Source); ok {
			entry := Mod{
				Source:     ModSourceModDB,
				ModID:      modID,
				ReleaseID:  versionID,
				Identifier: mod.Name,
				Version:    mod.Version,
				FileName:   mod.FileName,
				Enabled:    mod.Enabled,
			}
			if cached, cacheErr := s.catalog.GetDownloadedMod(ctx, modID, versionID); cacheErr == nil {
				if strings.TrimSpace(cached.Slug) != "" {
					entry.Identifier = cached.Slug
				}
				if strings.TrimSpace(cached.Version) != "" {
					entry.Version = cached.Version
				}
				if strings.TrimSpace(cached.FileName) != "" {
					entry.FileName = cached.FileName
				}
				entry.SHA256 = cached.Checksum
			}
			manifestMods = append(manifestMods, entry)
		} else {
			manifestMods = append(manifestMods, Mod{
				Source:     ModSourceUnknown,
				Identifier: mod.Name,
				Version:    mod.Version,
				FileName:   mod.FileName,
				Enabled:    mod.Enabled,
			})
		}
		if strings.TrimSpace(mod.FilePath) != "" {
			if absolute, absErr := filepath.Abs(mod.FilePath); absErr == nil {
				skipPaths[filepath.Clean(absolute)] = struct{}{}
			}
		}
	}
	return manifestMods, skipPaths
}

// List returns every readable snapshot of an instance, newest first.
func (s *Service) List(ctx context.Context, instanceID string) ([]InstanceSnapshot, error) {
	if _, err := s.instances.GetInstance(ctx, instanceID); err != nil {
		return nil, err
	}
	return s.storage.List(ctx, instanceID)
}

// ReadManifest loads the manifest of a snapshot directory. The instance
// ownership check stays with the caller.
func (s *Service) ReadManifest(ctx context.Context, instanceID, snapshotID string) (Manifest, error) {
	if _, err := s.instances.GetInstance(ctx, instanceID); err != nil {
		return Manifest{}, err
	}
	dir, err := s.storage.SnapshotDir(instanceID, snapshotID)
	if err != nil {
		return Manifest{}, err
	}
	return s.storage.ReadManifest(dir)
}

// IsRestorable reports whether a snapshot belongs to the instance and can be
// restored automatically.
func (s *Service) IsRestorable(ctx context.Context, instanceID, snapshotID string) bool {
	dir, err := s.storage.SnapshotDir(instanceID, snapshotID)
	if err != nil {
		return false
	}
	manifest, err := s.storage.ReadManifest(dir)
	if err != nil || manifest.InstanceID != instanceID {
		return false
	}
	return ValidateMods(manifest.Mods) == nil
}

// GameVersionName resolves the display name of a game version, falling back
// to the version ID when it is no longer installed.
func (s *Service) GameVersionName(ctx context.Context, gameVersionID string) string {
	version, err := s.versions.Get(ctx, gameVersionID)
	if err != nil {
		return gameVersionID
	}
	if strings.TrimSpace(version.Name) != "" {
		return version.Name
	}
	return gameVersionID
}

// Delete removes a snapshot of an instance. It only ever touches the snapshot
// directory and never the instance itself.
func (s *Service) Delete(ctx context.Context, instanceID, snapshotID string) error {
	release, err := s.beginMutation()
	if err != nil {
		return err
	}
	defer release()
	instance, err := s.instances.GetInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	if s.slot.IsBusy(instanceID) {
		return errs.NewError(ErrSnapshotInProgress, "Wait for the running snapshot operation to finish")
	}
	if err := s.storage.Remove(instanceID, snapshotID); err != nil {
		return err
	}
	s.clearLastKnownGood.ClearSnapshotReference(ctx, instanceID, snapshotID)
	slog.Info("instance snapshot deleted", "instance", instance.Name, "snapshot", snapshotID)
	return nil
}

// enforceRetention keeps only the newest automatic snapshots of an instance
// and deletes older ones. Manual snapshots are never removed. The snapshot
// currently referenced by the Last Known Good marker is protected from
// retention while it is the active recovery snapshot; the next eligible
// automatic snapshot is removed instead. Retention is best-effort: a cleanup
// failure is logged and never invalidates the freshly created snapshot or the
// operation it protects.
func (s *Service) enforceRetention(ctx context.Context, instanceID string) {
	snapshots, err := s.storage.List(ctx, instanceID)
	if err != nil {
		slog.Warn("could not list snapshots for automatic retention", "instanceId", instanceID, "error", err)
		return
	}
	var automatic []InstanceSnapshot
	for _, snapshot := range snapshots {
		if snapshot.Type == TypeAutomatic {
			automatic = append(automatic, snapshot)
		}
	}
	// List returns snapshots newest first.
	if len(automatic) <= automaticRetentionCount {
		return
	}
	protected := s.clearLastKnownGood.ProtectedSnapshotID(ctx, instanceID)
	for _, old := range automatic[automaticRetentionCount:] {
		if old.ID == protected {
			slog.Info("automatic retention kept the last known good recovery snapshot", "instanceId", instanceID, "snapshot", old.ID)
			continue
		}
		if removeErr := s.storage.Remove(instanceID, old.ID); removeErr != nil {
			slog.Warn("automatic retention could not remove an old snapshot", "instanceId", instanceID, "snapshot", old.ID, "error", removeErr)
			continue
		}
		slog.Info("automatic snapshot removed by retention", "instanceId", instanceID, "snapshot", old.ID, "reason", old.Reason)
	}
}

// ensureSpace rejects a snapshot operation when the free space on the data
// volume cannot hold the estimated data.
func (s *Service) ensureSpace(required int64) error {
	if s.diskSpace == nil {
		return nil
	}
	available, err := s.diskSpace.Available(s.dataRoot)
	if err != nil {
		return &errs.AppError{
			Code:    errs.ErrFilePermission,
			Message: "Could not check available disk space",
			Cause:   err,
		}
	}
	if available < required {
		return errs.NewError(errs.ErrInsufficientSpace, "Not enough free disk space")
	}
	return nil
}

// finishOperation marks a failed snapshot operation and persists it.
func (s *Service) finishOperation(operation operations.Operation, cause error, code string) {
	finishedAt := s.now().UTC()
	operation.FinishedAt = &finishedAt
	operation.Status = operations.StatusFailed
	operation.ErrorCode = &code
	message := cause.Error()
	operation.ErrorMessage = &message
	s.operations.SaveBestEffort(operation, operations.EventFailed)
}

// operationProgress returns a copy callback that throttles persisted
// operation progress updates while a snapshot copy runs.
func operationProgress(service *Service, operation *operations.Operation) func(int64) {
	return operationScaledProgress(service, operation, func(fraction float64) float64 {
		return fraction
	})
}

// operationScaledProgress returns a copy callback that maps the copied byte
// fraction through scale. Every update is published to the UI as an
// operation:progress event; the operation record is persisted at most every
// snapshotProgressInterval so high-frequency updates stay cheap.
func operationScaledProgress(
	service *Service,
	operation *operations.Operation,
	scale func(float64) float64,
) func(int64) {
	var copied int64
	lastSaved := time.Time{}
	return func(n int64) {
		copied += n
		operation.CurrentBytes = copied
		if operation.TotalBytes > 0 {
			operation.Progress = scale(float64(copied) / float64(operation.TotalBytes))
		}
		service.operations.Publish(operations.EventProgress, *operation)
		if time.Since(lastSaved) < snapshotProgressInterval {
			return
		}
		lastSaved = time.Now()
		service.operations.Persist(*operation)
	}
}

// SameModSet reports whether two snapshot mod lists describe the same
// configuration: the same mod identities with the same versions.
func SameModSet(left, right []Mod) bool {
	if len(left) != len(right) {
		return false
	}
	byKey := make(map[string]string, len(left))
	for _, mod := range left {
		byKey[ModKey(mod)] = mod.Version
	}
	for _, mod := range right {
		if version, ok := byKey[ModKey(mod)]; !ok || version != mod.Version {
			return false
		}
	}
	return true
}

// ModKey derives the stable identity of a snapshot mod entry. ModDB mods are
// identified by their catalog ID, never by filename, so an updated release
// still compares as the same mod. Manually installed mods fall back to their
// file name because they have no catalog identity.
func ModKey(mod Mod) string {
	if mod.Source == ModSourceModDB {
		return "moddb:" + strings.TrimSpace(mod.ModID)
	}
	name := strings.TrimSpace(mod.Identifier)
	if name == "" {
		name = strings.TrimSpace(mod.FileName)
	}
	return "local:" + name
}

// ModDisplayName renders the best available display name of a snapshot mod
// entry.
func ModDisplayName(mod Mod) string {
	if strings.TrimSpace(mod.Identifier) != "" {
		return mod.Identifier
	}
	if strings.TrimSpace(mod.ModID) != "" {
		return mod.ModID
	}
	return mod.FileName
}

// ValidateMods rejects snapshots whose mod list cannot be recovered
// automatically, before anything is staged.
func ValidateMods(mods []Mod) error {
	for _, mod := range mods {
		switch mod.Source {
		case ModSourceModDB:
			if strings.TrimSpace(mod.ModID) == "" || strings.TrimSpace(mod.ReleaseID) == "" {
				return errs.NewError(ErrSnapshotInvalid, "Snapshot mod metadata is incomplete")
			}
		case ModSourceUnknown:
			return errs.NewError(
				ErrSnapshotInvalid,
				"This snapshot contains a mod that Waxlight cannot download automatically: "+ModDisplayName(mod)+" "+mod.Version,
			)
		default:
			return errs.NewError(ErrSnapshotInvalid, "Snapshot contains an unsupported mod source")
		}
	}
	return nil
}

// ParseModDBSource splits a persisted mod source marker of the form
// "moddb:<modID>:<versionID>" into its parts. The marker format is shared
// with the mods feature.
func ParseModDBSource(source string) (string, string, bool) {
	parts := strings.Split(source, ":")
	if len(parts) != 3 || parts[0] != "moddb" || parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

// ModDBSource renders the persisted source marker of a catalog mod release.
// The marker format is shared with the mods feature.
func ModDBSource(modID, versionID string) string {
	return "moddb:" + modID + ":" + versionID
}
