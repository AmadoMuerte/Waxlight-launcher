package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/dataroot"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/filesystem"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/snapshotstore"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/operations"
)

const (
	snapshotProgressInterval  = 250 * time.Millisecond
	instanceMarkerFile        = ".waxlight-instance"
	snapshotModDownloadLimit  = 4
	restoreDownloadPhaseStart = 0.4
)

// createSnapshotInput carries the metadata of a snapshot being created. The
// type, reason and context distinguish manual snapshots from automatic safety
// snapshots in the manifest; manual creation leaves them at their zero values.
type createSnapshotInput struct {
	instanceID   string
	snapshotType domain.SnapshotType
	reason       domain.SnapshotReason
	context      map[string]string
}

// CreateInstanceSnapshot captures the current user data of an instance into a
// new manual snapshot.
func (s *Service) CreateInstanceSnapshot(
	ctx context.Context,
	instanceID string,
) (operations.Operation, error) {
	return s.createInstanceSnapshot(ctx, createSnapshotInput{
		instanceID:   instanceID,
		snapshotType: domain.SnapshotTypeManual,
	})
}

// createInstanceSnapshotLocked creates a snapshot while the caller already
// holds the per-instance mutation lock, so only the game-running rule still
// applies. Automatic safety snapshots use this path.
func (s *Service) createInstanceSnapshotLocked(
	ctx context.Context,
	input createSnapshotInput,
) (operations.Operation, error) {
	release, err := s.beginMutation()
	if err != nil {
		return operations.Operation{}, err
	}
	defer release()
	if err := s.ensureInstanceNotRunning(input.instanceID); err != nil {
		return operations.Operation{}, err
	}
	return s.createInstanceSnapshotCore(ctx, input)
}

// createInstanceSnapshot guards a manual snapshot creation against running
// games and concurrent snapshot operations before delegating to the shared
// creation core.
func (s *Service) createInstanceSnapshot(
	ctx context.Context,
	input createSnapshotInput,
) (operations.Operation, error) {
	release, err := s.beginMutation()
	if err != nil {
		return operations.Operation{}, err
	}
	defer release()
	if err := s.ensureInstanceSnapshotSafe(input.instanceID); err != nil {
		return operations.Operation{}, err
	}
	return s.createInstanceSnapshotCore(ctx, input)
}

// createInstanceSnapshotCore captures the current user data of an instance
// into a new snapshot. Waxlight-managed ModDB mod binaries are not copied; the
// manifest records their exact releases so restore can download the same
// versions again. The copy is staged in a temporary directory first, the
// manifest is written and validated, and only then the staging directory is
// atomically renamed into place. A snapshot is therefore never visible
// half-written, and the source instance is never modified.
func (s *Service) createInstanceSnapshotCore(
	ctx context.Context,
	input createSnapshotInput,
) (operations.Operation, error) {
	instance, err := s.store.GetInstance(ctx, input.instanceID)
	if err != nil {
		return operations.Operation{}, err
	}

	installedMods, err := s.ListMods(ctx, input.instanceID)
	if err != nil {
		return operations.Operation{}, err
	}
	manifestMods, skipPaths := s.snapshotModManifest(ctx, input.instanceID, installedMods)

	estimated, err := dataroot.TotalSizeContext(ctx, instance.Directory)
	if err != nil {
		return operations.Operation{}, &domain.AppError{
			Code:    domain.ErrFilePermission,
			Message: "Could not read the instance files",
			Cause:   err,
		}
	}
	for path := range skipPaths {
		if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
			estimated -= info.Size()
		}
	}
	if s.diskSpace != nil {
		if err := s.ensureSnapshotSpace(estimated); err != nil {
			return operations.Operation{}, err
		}
	}

	now := time.Now().UTC()
	resource := instance.ID
	operation := operations.Operation{
		ID:         newID(),
		Type:       "snapshot_create",
		ResourceID: &resource,
		Title:      "Creating snapshot",
		TitleKey:   operationTitleCreatingSnapshot,
		Status:     operations.StatusRunning,
		Progress:   0,
		TotalBytes: estimated,
		CreatedAt:  now,
		StartedAt:  &now,
	}
	if input.snapshotType == domain.SnapshotTypeAutomatic {
		operation.Title = "Creating safety backup..."
		operation.TitleKey = operationTitleCreatingSafetyBackup
	}
	if err := s.operations.Save(ctx, operation, operations.EventCreated); err != nil {
		slog.Warn("could not persist the snapshot operation", "operationId", operation.ID, "error", err)
	}

	s.snapshotMu.Lock()
	_, busy := s.snapshotBusy[instance.ID]
	if !busy {
		s.snapshotBusy[instance.ID] = operation.ID
	}
	s.snapshotMu.Unlock()
	defer func() {
		if !busy {
			s.releaseSnapshotBusy(instance.ID, operation.ID)
		}
	}()

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
		s.finishSnapshotOperation(operation, cause, code)
		message := "Could not create snapshot"
		if input.snapshotType == domain.SnapshotTypeAutomatic {
			message = "Could not create a safety backup. The instance was not modified"
		}
		return operation, &domain.AppError{
			Code:    code,
			Message: message,
			Cause:   cause,
		}
	}

	staging, err = s.snapshots.TempDir(instance.ID)
	if err != nil {
		return fail(err, domain.ErrFilePermission)
	}
	final, err := s.snapshots.SnapshotDir(instance.ID, operation.ID)
	if err != nil {
		return fail(err, domain.ErrValidation)
	}

	snapshotID := operation.ID
	stats, err := copySnapshotData(
		ctx,
		instance.Directory,
		snapshotstore.DataDir(staging),
		skipPaths,
		operationProgress(s, &operation),
	)
	if err != nil {
		return fail(err, domain.ErrFilePermission)
	}

	manifest := domain.SnapshotManifest{
		FormatVersion: domain.SnapshotFormatVersion,
		ID:            snapshotID,
		InstanceID:    instance.ID,
		InstanceName:  instance.Name,
		CreatedAt:     now,
		Type:          input.snapshotType,
		Reason:        input.reason,
		Context:       input.context,
		GameVersion:   s.instanceGameVersionName(ctx, instance),
		SizeBytes:     stats.sizeBytes,
		ModCount:      len(installedMods),
		WorldCount:    stats.worldCount,
		Mods:          manifestMods,
	}
	if err := s.snapshots.WriteManifest(staging, manifest); err != nil {
		return fail(err, domain.ErrFilePermission)
	}
	if _, err := s.snapshots.ReadManifest(staging); err != nil {
		return fail(err, domain.ErrSnapshotInvalid)
	}
	if err := os.Rename(staging, final); err != nil {
		return fail(err, domain.ErrFilePermission)
	}
	staging = ""

	finished := time.Now().UTC()
	operation.FinishedAt = &finished
	operation.Status = operations.StatusCompleted
	operation.Progress = 1
	operation.CurrentBytes = stats.sizeBytes
	s.operations.SaveBestEffort(operation, operations.EventCompleted)
	if input.snapshotType == domain.SnapshotTypeAutomatic {
		slog.Info("automatic safety snapshot created", "instance", instance.Name, "snapshot", snapshotID, "reason", input.reason, "size", stats.sizeBytes, "mods", len(installedMods))
	} else {
		slog.Info("instance snapshot created", "instance", instance.Name, "snapshot", snapshotID, "size", stats.sizeBytes, "mods", len(installedMods))
	}
	return operation, nil
}

// snapshotModManifest maps the installed mods of an instance to manifest
// entries and collects the file paths that must be skipped during the data
// copy. Managed ModDB mods are enriched with the cached release metadata when
// it is still available; manual mods are recorded without a downloadable
// source. The manifest never stores credentials.
func (s *Service) snapshotModManifest(
	ctx context.Context,
	instanceID string,
	installedMods []domain.InstalledMod,
) ([]domain.SnapshotMod, map[string]struct{}) {
	manifestMods := make([]domain.SnapshotMod, 0, len(installedMods))
	skipPaths := make(map[string]struct{}, len(installedMods))
	for _, mod := range installedMods {
		if modID, versionID, ok := parseModDBSource(mod.Source); ok {
			entry := domain.SnapshotMod{
				Source:     domain.SnapshotModSourceModDB,
				ModID:      modID,
				ReleaseID:  versionID,
				Identifier: mod.Name,
				Version:    mod.Version,
				FileName:   mod.FileName,
				Enabled:    mod.Enabled,
			}
			if s.modDownloads != nil {
				if cached, cacheErr := s.modDownloads.Get(ctx, modID, versionID); cacheErr == nil {
					if strings.TrimSpace(cached.Slug) != "" {
						entry.Identifier = cached.Slug
					}
					if strings.TrimSpace(cached.DownloadedVersion) != "" {
						entry.Version = cached.DownloadedVersion
					}
					if strings.TrimSpace(cached.FileName) != "" {
						entry.FileName = cached.FileName
					}
					entry.SHA256 = cached.Checksum
				}
			}
			manifestMods = append(manifestMods, entry)
		} else {
			manifestMods = append(manifestMods, domain.SnapshotMod{
				Source:     domain.SnapshotModSourceUnknown,
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

// ListInstanceSnapshots returns every readable snapshot of an instance, newest
// first. Unreadable snapshot directories are logged and skipped.
func (s *Service) ListInstanceSnapshots(
	ctx context.Context,
	instanceID string,
) ([]domain.InstanceSnapshot, error) {
	if _, err := s.store.GetInstance(ctx, instanceID); err != nil {
		return nil, err
	}
	return s.snapshots.List(ctx, instanceID)
}

// RestoreInstanceSnapshot replaces the current user data of an instance with
// the captured state of a snapshot. The restored copy is prepared in a
// temporary directory and swapped with the live directory only after it is
// complete, so a failed restore never leaves the instance destroyed.
func (s *Service) RestoreInstanceSnapshot(
	ctx context.Context,
	instanceID string,
	snapshotID string,
) error {
	release, err := s.beginMutation()
	if err != nil {
		return err
	}
	defer release()
	instance, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	if err := s.ensureInstanceSnapshotSafe(instanceID); err != nil {
		return err
	}

	snapshotDir, err := s.snapshots.SnapshotDir(instanceID, snapshotID)
	if err != nil {
		return err
	}
	manifest, err := s.snapshots.ReadManifest(snapshotDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.NewError(domain.ErrSnapshotNotFound, "Snapshot not found")
		}
		slog.Warn("snapshot manifest could not be read", "instanceId", instanceID, "snapshotId", snapshotID, "error", err)
		return domain.NewError(domain.ErrSnapshotInvalid, "Snapshot is corrupted or incomplete")
	}
	if manifest.InstanceID != instance.ID {
		return domain.NewError(domain.ErrSnapshotInvalid, "This snapshot does not belong to the selected instance")
	}

	if manifest.FormatVersion == domain.SnapshotFormatVersion1 {
		return s.restoreInstanceSnapshotV1(ctx, instance, snapshotDir, manifest)
	}
	return s.restoreInstanceSnapshotV2(ctx, instance, snapshotDir, manifest)
}

// restoreInstanceSnapshotV1 restores a legacy snapshot whose data directory
// physically contains the instance's Mods. Every file is copied back verbatim.
func (s *Service) restoreInstanceSnapshotV1(
	ctx context.Context,
	instance instances.Instance,
	snapshotDir string,
	manifest domain.SnapshotManifest,
) error {
	size, err := snapshotstore.Size(snapshotDir)
	if err != nil {
		return &domain.AppError{
			Code:    domain.ErrSnapshotInvalid,
			Message: "Snapshot is corrupted or incomplete",
			Cause:   err,
		}
	}
	if s.diskSpace != nil {
		if err := s.ensureSnapshotSpace(size); err != nil {
			return err
		}
	}

	operation := s.beginSnapshotRestore(instance, size)
	s.snapshotMu.Lock()
	s.snapshotBusy[instance.ID] = operation.ID
	s.snapshotMu.Unlock()
	defer s.releaseSnapshotBusy(instance.ID, operation.ID)

	fail := func(cause error, code string) error {
		s.finishSnapshotOperation(operation, cause, code)
		return &domain.AppError{
			Code:    code,
			Message: "Could not restore snapshot",
			Cause:   cause,
		}
	}

	staging, err := prepareRestoreStaging(ctx, instance, snapshotDir, nil)
	if err != nil {
		if errors.Is(err, errRestoreStaging) {
			return fail(restoreStagingCause(err), domain.ErrSnapshotInvalid)
		}
		return fail(err, domain.ErrFilePermission)
	}

	if err := swapRestoredInstance(ctx, instance, staging, s.dataRoot); err != nil {
		return fail(err, domain.ErrFilePermission)
	}
	s.finalizeRestore(instance, operation, size)
	return nil
}

// restoreInstanceSnapshotV2 restores a lightweight snapshot: instance data is
// copied first, then every managed ModDB release is downloaded into the
// staging directory, and only a complete staging directory replaces the live
// instance. Manual mods without a downloadable source are rejected before
// anything is touched.
func (s *Service) restoreInstanceSnapshotV2(
	ctx context.Context,
	instance instances.Instance,
	snapshotDir string,
	manifest domain.SnapshotManifest,
) error {
	if err := validateSnapshotMods(manifest.Mods); err != nil {
		return err
	}
	size, err := snapshotstore.Size(snapshotDir)
	if err != nil {
		return &domain.AppError{
			Code:    domain.ErrSnapshotInvalid,
			Message: "Snapshot is corrupted or incomplete",
			Cause:   err,
		}
	}
	if s.diskSpace != nil {
		if err := s.ensureSnapshotSpace(size); err != nil {
			return err
		}
	}

	operation := s.beginSnapshotRestore(instance, size)
	s.snapshotMu.Lock()
	s.snapshotBusy[instance.ID] = operation.ID
	s.snapshotMu.Unlock()
	defer s.releaseSnapshotBusy(instance.ID, operation.ID)

	fail := func(cause error, code string) error {
		s.finishSnapshotOperation(operation, cause, code)
		return &domain.AppError{
			Code:    code,
			Message: "Could not restore snapshot",
			Cause:   cause,
		}
	}

	operation.Title = "Restoring files..."
	operation.TitleKey = operationTitleRestoringFiles
	operation.TitleParams = nil
	s.operations.SaveBestEffort(operation, operations.EventProgress)
	staging, err := prepareRestoreStaging(
		ctx,
		instance,
		snapshotDir,
		operationScaledProgress(s, &operation, func(fraction float64) float64 {
			return restoreDownloadPhaseStart * fraction
		}),
	)
	if err != nil {
		if errors.Is(err, errRestoreStaging) {
			return fail(restoreStagingCause(err), domain.ErrSnapshotInvalid)
		}
		return fail(err, domain.ErrFilePermission)
	}

	restored := []restoredSnapshotMod{}
	if len(manifest.Mods) > 0 {
		operation.Title = "Downloading mods..."
		operation.TitleKey = operationTitleDownloadingMods
		operation.TitleParams = nil
		s.operations.SaveBestEffort(operation, operations.EventProgress)
		var restoreErr error
		restored, restoreErr = s.restoreSnapshotMods(ctx, staging, manifest.Mods, &operation)
		if restoreErr != nil {
			_ = os.RemoveAll(staging)
			message := snapshotModRestoreMessage(restoreErr)
			s.finishSnapshotOperation(operation, restoreErr, domain.ErrSnapshotInvalid)
			return &domain.AppError{
				Code:    domain.ErrSnapshotInvalid,
				Message: message,
				Cause:   restoreErr,
			}
		}
		if err := s.validateRestoredMods(staging, restored); err != nil {
			_ = os.RemoveAll(staging)
			s.finishSnapshotOperation(operation, err, domain.ErrSnapshotInvalid)
			return &domain.AppError{
				Code:    domain.ErrSnapshotInvalid,
				Message: "Could not restore snapshot",
				Cause:   err,
			}
		}
	}

	operation.Title = "Finishing restore..."
	operation.TitleKey = operationTitleFinishingRestore
	operation.TitleParams = nil
	operation.Progress = 0.95
	s.operations.SaveBestEffort(operation, operations.EventProgress)

	if err := swapRestoredInstance(ctx, instance, staging, s.dataRoot); err != nil {
		return fail(err, domain.ErrFilePermission)
	}
	if err := s.rebuildInstanceMods(ctx, instance, staging, restored); err != nil {
		slog.Warn("could not rebuild the restored instance mod records", "instance", instance.Name, "error", err)
	}

	s.finalizeRestore(instance, operation, size)
	return nil
}

// beginSnapshotRestore creates the persisted operation record for a restore.
func (s *Service) beginSnapshotRestore(
	instance instances.Instance,
	size int64,
) operations.Operation {
	now := time.Now().UTC()
	resource := instance.ID
	operation := operations.Operation{
		ID:         newID(),
		Type:       "snapshot_restore",
		ResourceID: &resource,
		Title:      "Restoring snapshot",
		TitleKey:   operationTitleRestoringSnapshot,
		Status:     operations.StatusRunning,
		Progress:   0,
		TotalBytes: size,
		CreatedAt:  now,
		StartedAt:  &now,
	}
	if err := s.operations.Save(context.Background(), operation, operations.EventCreated); err != nil {
		slog.Warn("could not persist the restore operation", "operationId", operation.ID, "error", err)
	}
	return operation
}

// prepareRestoreStaging copies a snapshot's data directory into a fresh
// staging directory next to the instance and writes the instance marker. The
// returned error wraps errRestoreStaging when the copy itself failed.
func prepareRestoreStaging(
	ctx context.Context,
	instance instances.Instance,
	snapshotDir string,
	progress func(int64),
) (string, error) {
	parent := filepath.Dir(instance.Directory)
	staging, err := os.MkdirTemp(parent, ".waxlight-restore-")
	if err != nil {
		return "", err
	}
	if _, err := copySnapshotData(
		ctx,
		snapshotstore.DataDir(snapshotDir),
		staging,
		nil,
		progress,
	); err != nil {
		_ = os.RemoveAll(staging)
		return "", fmt.Errorf("%w: %v", errRestoreStaging, err)
	}
	if err := os.WriteFile(filepath.Join(staging, instanceMarkerFile), []byte(instance.ID), 0o600); err != nil {
		_ = os.RemoveAll(staging)
		return "", err
	}
	if err := hardenLogs(filepath.Join(staging, "Logs")); err != nil {
		_ = os.RemoveAll(staging)
		return "", err
	}
	return staging, nil
}

// errRestoreStaging separates data copy failures from filesystem plumbing
// failures so callers can map them to the snapshot corruption error.
var errRestoreStaging = errors.New("snapshot data could not be restored")

func restoreStagingCause(err error) error {
	return errors.Unwrap(err)
}

// swapRestoredInstance atomically replaces the live instance directory with
// the prepared staging directory. On failure the previous directory is moved
// back so the instance stays intact.
func swapRestoredInstance(ctx context.Context, instance instances.Instance, staging string, dataRoot string) error {
	parent := filepath.Dir(instance.Directory)
	previous := filepath.Join(parent, ".waxlight-restore-old-"+newID()[:12])
	if err := os.Rename(instance.Directory, previous); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	if err := os.Rename(staging, instance.Directory); err != nil {
		rollbackErr := os.Rename(previous, instance.Directory)
		if rollbackErr != nil {
			slog.Error(
				"restore failed and the previous instance directory could not be moved back",
				"instance", instance.Name,
				"previousDirectory", previous,
				"error", rollbackErr,
			)
		}
		return err
	}
	if err := safeRemoveAll(previous, dataRoot, instanceMarkerFile); err != nil {
		slog.Warn("could not remove the replaced instance directory", "instance", instance.Name, "error", err)
	}
	return nil
}

// finalizeRestore applies the post-swap hardening and completes the operation.
func (s *Service) finalizeRestore(instance instances.Instance, operation operations.Operation, size int64) {
	if s.clientSettings != nil {
		if err := s.clientSettings.Clear(filepath.Join(instance.Directory, "clientsettings.json")); err != nil {
			slog.Warn("could not clear authentication from the restored instance", "instance", instance.Name, "error", err)
		}
	}
	if err := hardenLogs(filepath.Join(instance.Directory, "Logs")); err != nil {
		slog.Warn("could not harden the restored instance logs", "instance", instance.Name, "error", err)
	}
	finished := time.Now().UTC()
	operation.FinishedAt = &finished
	operation.Status = operations.StatusCompleted
	operation.Progress = 1
	operation.CurrentBytes = size
	operation.Title = "Restoring snapshot"
	operation.TitleKey = operationTitleRestoringSnapshot
	operation.TitleParams = nil
	s.operations.SaveBestEffort(operation, operations.EventCompleted)
	slog.Info("instance restored from snapshot", "instance", instance.Name)
}

// validateSnapshotMods rejects snapshots whose mod list cannot be recovered
// automatically, before anything is staged.
func validateSnapshotMods(mods []domain.SnapshotMod) error {
	for _, mod := range mods {
		switch mod.Source {
		case domain.SnapshotModSourceModDB:
			if strings.TrimSpace(mod.ModID) == "" || strings.TrimSpace(mod.ReleaseID) == "" {
				return domain.NewError(domain.ErrSnapshotInvalid, "Snapshot mod metadata is incomplete")
			}
		case domain.SnapshotModSourceUnknown:
			return domain.NewError(
				domain.ErrSnapshotInvalid,
				fmt.Sprintf(
					"This snapshot contains a mod that Waxlight cannot download automatically: %s %s",
					snapshotModDisplayName(mod),
					mod.Version,
				),
			)
		default:
			return domain.NewError(domain.ErrSnapshotInvalid, "Snapshot contains an unsupported mod source")
		}
	}
	return nil
}

type restoredSnapshotMod struct {
	entry         domain.SnapshotMod
	displayName   string
	installedPath string
}

// snapshotModDownloadError carries the mod that failed during restore.
type snapshotModDownloadError struct {
	mod   domain.SnapshotMod
	cause error
}

func (e *snapshotModDownloadError) Error() string {
	return fmt.Sprintf("could not restore the mod %s %s: %v", snapshotModDisplayName(e.mod), e.mod.Version, e.cause)
}

func (e *snapshotModDownloadError) Unwrap() error { return e.cause }

// snapshotModRestoreMessage renders the user-facing failure message for a mod
// that could not be recovered.
func snapshotModRestoreMessage(err error) string {
	var downloadError *snapshotModDownloadError
	if !errors.As(err, &downloadError) {
		return "Snapshot could not be restored: " + err.Error()
	}
	mod := downloadError.mod
	if isAppErrorCode(downloadError.cause, domain.ErrModVersionNotFound) {
		return fmt.Sprintf(
			"Snapshot could not be restored. The following mod release is no longer available: %s %s",
			snapshotModDisplayName(mod),
			mod.Version,
		)
	}
	detail := downloadError.cause.Error()
	if appError, ok := downloadError.cause.(*domain.AppError); ok {
		detail = appError.Message
	}
	return fmt.Sprintf(
		"Snapshot could not be restored. Could not download the mod %s %s: %s",
		snapshotModDisplayName(mod),
		mod.Version,
		detail,
	)
}

func snapshotModDisplayName(mod domain.SnapshotMod) string {
	if strings.TrimSpace(mod.Identifier) != "" {
		return mod.Identifier
	}
	if strings.TrimSpace(mod.ModID) != "" {
		return mod.ModID
	}
	return mod.FileName
}

// restoreSnapshotMods downloads and installs every managed release of a
// snapshot into the staging directory. Downloads run with a bounded worker
// count; the first failure cancels the remaining work.
func (s *Service) restoreSnapshotMods(
	ctx context.Context,
	staging string,
	mods []domain.SnapshotMod,
	operation *operations.Operation,
) ([]restoredSnapshotMod, error) {
	downloadCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan restoredSnapshotMod, len(mods))
	failures := make(chan snapshotModDownloadError, len(mods))
	semaphore := make(chan struct{}, snapshotModDownloadLimit)
	var completed atomic.Int64
	var progressMu sync.Mutex

	for index := range mods {
		semaphore <- struct{}{}
		go func(mod domain.SnapshotMod) {
			defer func() { <-semaphore }()
			installed, err := s.restoreSnapshotMod(downloadCtx, staging, mod)
			done := completed.Add(1)
			if err != nil {
				failures <- snapshotModDownloadError{mod: mod, cause: err}
				return
			}
			progressMu.Lock()
			operation.Title = fmt.Sprintf("Restoring mods %d / %d", done, len(mods))
			operation.TitleKey = operationTitleRestoringModsProgress
			operation.TitleParams = titleParams(
				"done", strconv.FormatInt(done, 10),
				"total", strconv.Itoa(len(mods)),
			)
			operation.Progress = restoreDownloadPhaseStart +
				0.5*float64(done)/float64(len(mods))
			snapshot := *operation
			progressMu.Unlock()
			s.operations.SaveBestEffort(snapshot, operations.EventProgress)
			results <- installed
		}(mods[index])
	}

	restored := make([]restoredSnapshotMod, 0, len(mods))
	var firstFailure *snapshotModDownloadError
	for range mods {
		select {
		case installed := <-results:
			restored = append(restored, installed)
		case failure := <-failures:
			if firstFailure == nil {
				copy := failure
				firstFailure = &copy
				cancel()
			}
		}
	}
	if firstFailure != nil {
		return nil, firstFailure
	}
	return restored, nil
}

// restoreSnapshotMod fetches the exact release of a managed mod and installs
// it into the staging directory.
func (s *Service) restoreSnapshotMod(
	ctx context.Context,
	staging string,
	mod domain.SnapshotMod,
) (restoredSnapshotMod, error) {
	downloaded, err := s.downloadModRelease(ctx, mod.ModID, mod.ReleaseID)
	if err != nil {
		return restoredSnapshotMod{}, err
	}
	if mod.SHA256 != "" && downloaded.Checksum != "" && !strings.EqualFold(mod.SHA256, downloaded.Checksum) {
		return restoredSnapshotMod{}, fmt.Errorf(
			"the downloaded release does not match the snapshot checksum",
		)
	}
	if info, infoErr := readModArchiveInfo(downloaded.FilePath); infoErr == nil {
		if strings.TrimSpace(info.ModID) != "" && strings.TrimSpace(mod.Identifier) != "" &&
			!strings.EqualFold(info.ModID, mod.Identifier) {
			slog.Warn("snapshot mod identifier mismatch", "snapshot", mod.Identifier, "archive", info.ModID, "modId", mod.ModID)
		}
		if strings.TrimSpace(info.Version) != "" && strings.TrimSpace(mod.Version) != "" &&
			!strings.EqualFold(info.Version, mod.Version) {
			slog.Warn("snapshot mod version mismatch", "snapshot", mod.Version, "archive", info.Version, "modId", mod.ModID)
		}
	}

	targetDir := filepath.Join(staging, modsDirectoryFor(mod.Enabled))
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return restoredSnapshotMod{}, err
	}
	fileName := filepath.Base(downloaded.FilePath)
	target := filepath.Join(targetDir, fileName)
	if _, err := copySnapshotFileContext(ctx, downloaded.FilePath, target, 0o644); err != nil {
		return restoredSnapshotMod{}, err
	}
	displayName := downloaded.Name
	if strings.TrimSpace(displayName) == "" {
		displayName = mod.Identifier
	}
	return restoredSnapshotMod{
		entry:         mod,
		displayName:   displayName,
		installedPath: target,
	}, nil
}

// validateRestoredMods verifies that every expected mod file exists in the
// staging directory after the download phase.
func (s *Service) validateRestoredMods(staging string, restored []restoredSnapshotMod) error {
	for _, mod := range restored {
		if _, err := os.Stat(mod.installedPath); err != nil {
			return fmt.Errorf("restored mod file is missing: %w", err)
		}
	}
	return nil
}

// rebuildInstanceMods replaces the instance's mod records with the set
// restored from the snapshot. The restored files on disk are authoritative.
func (s *Service) rebuildInstanceMods(
	ctx context.Context,
	instance instances.Instance,
	staging string,
	restored []restoredSnapshotMod,
) error {
	existing, err := s.store.ListMods(ctx, instance.ID)
	if err != nil {
		return err
	}
	for _, stale := range existing {
		if err := s.store.DeleteMod(ctx, stale.ID); err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	for _, restoredMod := range restored {
		relative, relErr := filepath.Rel(staging, restoredMod.installedPath)
		if relErr != nil || strings.HasPrefix(relative, "..") {
			return fmt.Errorf("restored mod file escapes the instance directory")
		}
		path := filepath.Join(instance.Directory, relative)
		info, statErr := os.Stat(path)
		if statErr != nil {
			return statErr
		}
		source := "local"
		if restoredMod.entry.Source == domain.SnapshotModSourceModDB {
			source = modDBSource(restoredMod.entry.ModID, restoredMod.entry.ReleaseID)
		}
		record := domain.InstalledMod{
			ID:          newID(),
			InstanceID:  instance.ID,
			Name:        restoredMod.displayName,
			Version:     restoredMod.entry.Version,
			FileName:    filepath.Base(path),
			FilePath:    path,
			Enabled:     restoredMod.entry.Enabled,
			Managed:     restoredMod.entry.Source == domain.SnapshotModSourceModDB,
			Source:      source,
			SizeBytes:   info.Size(),
			InstalledAt: now,
			UpdatedAt:   now,
		}
		if strings.TrimSpace(record.Name) == "" {
			record.Name = restoredMod.entry.Identifier
		}
		if err := s.store.SaveMod(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

// DeleteInstanceSnapshot removes a snapshot of an instance. It only ever
// touches the snapshot directory and never the instance itself.
func (s *Service) DeleteInstanceSnapshot(
	ctx context.Context,
	instanceID string,
	snapshotID string,
) error {
	release, err := s.beginMutation()
	if err != nil {
		return err
	}
	defer release()
	instance, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	s.snapshotMu.Lock()
	busy := s.snapshotBusy[instanceID]
	s.snapshotMu.Unlock()
	if busy != "" {
		return domain.NewError(domain.ErrSnapshotInProgress, "Wait for the running snapshot operation to finish")
	}
	if err := s.snapshots.Remove(instanceID, snapshotID); err != nil {
		return err
	}
	s.ClearLastKnownGoodSnapshotReference(ctx, instanceID, snapshotID)
	slog.Info("instance snapshot deleted", "instance", instance.Name, "snapshot", snapshotID)
	return nil
}

// ensureInstanceSnapshotSafe rejects snapshot operations while the game runs
// or another snapshot operation is in progress for the same instance.
func (s *Service) ensureInstanceSnapshotSafe(instanceID string) error {
	s.snapshotMu.Lock()
	busy := s.snapshotBusy[instanceID]
	s.snapshotMu.Unlock()
	if busy != "" {
		return domain.NewError(domain.ErrSnapshotInProgress, "Wait for the running snapshot operation to finish")
	}
	return s.ensureInstanceNotRunning(instanceID)
}

// ensureInstanceNotRunning rejects operations that must not touch an instance
// while the game is running.
func (s *Service) ensureInstanceNotRunning(instanceID string) error {
	s.runningMu.Lock()
	_, running := s.running[instanceID]
	s.runningMu.Unlock()
	if running {
		return domain.NewError(instances.ErrInstanceRunning, "Stop the game before modifying this instance")
	}
	return nil
}

// ensureNoSnapshotOperation rejects instance lifecycle operations while a
// snapshot operation for the same instance is in progress.
func (s *Service) ensureNoSnapshotOperation(instanceID string) error {
	s.snapshotMu.Lock()
	busy := s.snapshotBusy[instanceID]
	s.snapshotMu.Unlock()
	if busy != "" {
		return domain.NewError(domain.ErrSnapshotInProgress, "Wait for the running snapshot operation to finish")
	}
	return nil
}

func (s *Service) releaseSnapshotBusy(instanceID string, operationID string) {
	s.snapshotMu.Lock()
	if s.snapshotBusy[instanceID] == operationID {
		delete(s.snapshotBusy, instanceID)
	}
	s.snapshotMu.Unlock()
}

// ensureSnapshotSpace rejects a snapshot operation when the free space on the
// data volume cannot hold the estimated data.
func (s *Service) ensureSnapshotSpace(required int64) error {
	available, err := s.diskSpace.Available(s.dataRoot)
	if err != nil {
		return &domain.AppError{
			Code:    domain.ErrFilePermission,
			Message: "Could not check available disk space",
			Cause:   err,
		}
	}
	if available < required {
		return domain.NewError(domain.ErrInsufficientSpace, "Not enough free disk space")
	}
	return nil
}

// instanceGameVersionName resolves the display name of the game version an
// instance runs, falling back to the version ID when it is no longer installed.
func (s *Service) instanceGameVersionName(ctx context.Context, instance instances.Instance) string {
	version, err := s.versions.Get(ctx, instance.GameVersionID)
	if err != nil {
		return instance.GameVersionID
	}
	if strings.TrimSpace(version.Name) != "" {
		return version.Name
	}
	return instance.GameVersionID
}

// finishSnapshotOperation marks a failed snapshot operation and persists it.
func (s *Service) finishSnapshotOperation(operation operations.Operation, cause error, code string) {
	finishedAt := time.Now().UTC()
	operation.FinishedAt = &finishedAt
	operation.Status = operations.StatusFailed
	operation.ErrorCode = &code
	message := cause.Error()
	operation.ErrorMessage = &message
	s.operations.SaveBestEffort(operation, operations.EventFailed)
}

// operationProgress returns a copy callback that throttles persisted operation
// progress updates while a snapshot copy runs.
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

type snapshotStats struct {
	sizeBytes  int64
	worldCount int
}

// copySnapshotData copies an instance data directory into destination. Symbolic
// links are rejected, launcher markers and authentication journals are skipped,
// and clientsettings.json is sanitized so temporary session credentials never
// become part of a snapshot. Files listed in skipPaths are not copied.
func copySnapshotData(
	ctx context.Context,
	source string,
	destination string,
	skipPaths map[string]struct{},
	progress func(int64),
) (snapshotStats, error) {
	stats := snapshotStats{worldCount: countWorlds(filepath.Join(source, "SaveGame"))}
	err := filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == source {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		switch {
		case info.Name() == instanceMarkerFile:
			return nil
		case strings.HasSuffix(info.Name(), ".waxlight-auth-injection"):
			return nil
		}
		if absolute, absErr := filepath.Abs(path); absErr == nil {
			if _, skipped := skipPaths[filepath.Clean(absolute)]; skipped {
				return nil
			}
		}

		target := filepath.Join(destination, relative)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("instance data contains a symbolic link and cannot be snapshotted")
		}
		if !info.Mode().IsRegular() {
			return errors.New("instance data contains a non-regular file and cannot be snapshotted")
		}
		if strings.EqualFold(info.Name(), "clientsettings.json") {
			size, err := sanitizeClientSettingsCopy(path, target, info.Mode().Perm())
			if err != nil {
				return err
			}
			stats.sizeBytes += size
			if progress != nil {
				progress(size)
			}
			return nil
		}
		size, err := copySnapshotFile(path, target, info.Mode().Perm())
		if err != nil {
			return err
		}
		stats.sizeBytes += size
		if progress != nil {
			progress(size)
		}
		return nil
	})
	return stats, err
}

// sanitizeClientSettingsCopy copies clientsettings.json with every temporary
// authentication property and machine specific mod path removed. It returns
// the number of bytes written.
func sanitizeClientSettingsCopy(source string, destination string, mode os.FileMode) (int64, error) {
	file, err := os.Open(source)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, 8<<20))
	if err != nil {
		return 0, err
	}
	sanitized, err := filesystem.SanitizeClientSettings(contents)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return 0, err
	}
	if err := os.WriteFile(destination, sanitized, mode); err != nil {
		return 0, err
	}
	return int64(len(sanitized)), nil
}

func copySnapshotFile(source string, destination string, mode os.FileMode) (int64, error) {
	return copySnapshotFileContext(context.Background(), source, destination, mode)
}

func copySnapshotFileContext(ctx context.Context, source string, destination string, mode os.FileMode) (int64, error) {
	input, err := os.Open(source)
	if err != nil {
		return 0, err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return 0, err
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return 0, err
	}
	size, err := io.Copy(output, &contextReaderMod{ctx: ctx, reader: input})
	if err != nil {
		_ = output.Close()
		return 0, err
	}
	return size, output.Close()
}

// countWorlds counts the save data worlds directly under the SaveGame
// directory.
func countWorlds(saveGameDir string) int {
	entries, err := os.ReadDir(saveGameDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			count++
		}
	}
	return count
}
