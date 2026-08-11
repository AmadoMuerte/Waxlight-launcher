package snapshots

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/operations"
)

// Restore replaces the current user data of an instance with the captured
// state of a snapshot. The restored copy is prepared in a temporary directory
// and swapped with the live directory only after it is complete, so a failed
// restore never leaves the instance destroyed.
func (s *Service) Restore(ctx context.Context, instanceID, snapshotID string) error {
	release, err := s.beginMutation()
	if err != nil {
		return err
	}
	defer release()
	instance, err := s.instances.GetInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	reservationRelease, err := s.lock.Guard(instanceID, ReservationMarker, "Stop the game before modifying this instance")
	if err != nil {
		return err
	}
	defer reservationRelease()

	snapshotDir, err := s.storage.SnapshotDir(instanceID, snapshotID)
	if err != nil {
		return err
	}
	manifest, err := s.storage.ReadManifest(snapshotDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.NewError(ErrSnapshotNotFound, "Snapshot not found")
		}
		slog.Warn("snapshot manifest could not be read", "instanceId", instanceID, "snapshotId", snapshotID, "error", err)
		return domain.NewError(ErrSnapshotInvalid, "Snapshot is corrupted or incomplete")
	}
	if manifest.InstanceID != instance.ID {
		return domain.NewError(ErrSnapshotInvalid, "This snapshot does not belong to the selected instance")
	}

	if manifest.FormatVersion == FormatVersion1 {
		return s.restoreV1(ctx, instance, snapshotDir, manifest)
	}
	return s.restoreV2(ctx, instance, snapshotDir, manifest)
}

// restoreV1 restores a legacy snapshot whose data directory physically
// contains the instance's Mods. Every file is copied back verbatim.
func (s *Service) restoreV1(
	ctx context.Context,
	instance InstanceRef,
	snapshotDir string,
	manifest Manifest,
) error {
	size, err := s.storage.Size(snapshotDir)
	if err != nil {
		return &domain.AppError{
			Code:    ErrSnapshotInvalid,
			Message: "Snapshot is corrupted or incomplete",
			Cause:   err,
		}
	}
	if err := s.ensureSpace(size); err != nil {
		return err
	}

	operation := s.beginRestore(instance, size)
	defer s.slot.Set(instance.ID, operation.ID)()

	fail := func(cause error, code string) error {
		s.finishOperation(operation, cause, code)
		return &domain.AppError{
			Code:    code,
			Message: "Could not restore snapshot",
			Cause:   cause,
		}
	}

	staging, err := s.prepareRestoreStaging(ctx, instance, snapshotDir, nil)
	if err != nil {
		if errors.Is(err, errRestoreStaging) {
			return fail(restoreStagingCause(err), ErrSnapshotInvalid)
		}
		return fail(err, domain.ErrFilePermission)
	}

	if err := s.swapRestoredInstance(ctx, instance, staging); err != nil {
		return fail(err, domain.ErrFilePermission)
	}
	s.finalizeRestore(instance, operation, size)
	return nil
}

// restoreV2 restores a lightweight snapshot: instance data is copied first,
// then every managed ModDB release is downloaded into the staging directory,
// and only a complete staging directory replaces the live instance. Manual
// mods without a downloadable source are rejected before anything is touched.
func (s *Service) restoreV2(
	ctx context.Context,
	instance InstanceRef,
	snapshotDir string,
	manifest Manifest,
) error {
	if err := ValidateMods(manifest.Mods); err != nil {
		return err
	}
	size, err := s.storage.Size(snapshotDir)
	if err != nil {
		return &domain.AppError{
			Code:    ErrSnapshotInvalid,
			Message: "Snapshot is corrupted or incomplete",
			Cause:   err,
		}
	}
	if err := s.ensureSpace(size); err != nil {
		return err
	}

	operation := s.beginRestore(instance, size)
	defer s.slot.Set(instance.ID, operation.ID)()

	fail := func(cause error, code string) error {
		s.finishOperation(operation, cause, code)
		return &domain.AppError{
			Code:    code,
			Message: "Could not restore snapshot",
			Cause:   cause,
		}
	}

	operation.Title = "Restoring files..."
	operation.TitleKey = TitleRestoringFiles
	operation.TitleParams = nil
	s.operations.SaveBestEffort(operation, operations.EventProgress)
	staging, err := s.prepareRestoreStaging(
		ctx,
		instance,
		snapshotDir,
		operationScaledProgress(s, &operation, func(fraction float64) float64 {
			return restoreDownloadPhaseStart * fraction
		}),
	)
	if err != nil {
		if errors.Is(err, errRestoreStaging) {
			return fail(restoreStagingCause(err), ErrSnapshotInvalid)
		}
		return fail(err, domain.ErrFilePermission)
	}

	restored := []restoredMod{}
	if len(manifest.Mods) > 0 {
		operation.Title = "Downloading mods..."
		operation.TitleKey = TitleDownloadingMods
		operation.TitleParams = nil
		s.operations.SaveBestEffort(operation, operations.EventProgress)
		var restoreErr error
		restored, restoreErr = s.restoreSnapshotMods(ctx, staging, manifest.Mods, &operation)
		if restoreErr != nil {
			_ = os.RemoveAll(staging)
			message := modRestoreMessage(restoreErr)
			s.finishOperation(operation, restoreErr, ErrSnapshotInvalid)
			return &domain.AppError{
				Code:    ErrSnapshotInvalid,
				Message: message,
				Cause:   restoreErr,
			}
		}
		if err := s.validateRestoredMods(staging, restored); err != nil {
			_ = os.RemoveAll(staging)
			s.finishOperation(operation, err, ErrSnapshotInvalid)
			return &domain.AppError{
				Code:    ErrSnapshotInvalid,
				Message: "Could not restore snapshot",
				Cause:   err,
			}
		}
	}

	operation.Title = "Finishing restore..."
	operation.TitleKey = TitleFinishingRestore
	operation.TitleParams = nil
	operation.Progress = 0.95
	s.operations.SaveBestEffort(operation, operations.EventProgress)

	if err := s.swapRestoredInstance(ctx, instance, staging); err != nil {
		return fail(err, domain.ErrFilePermission)
	}
	if err := s.rebuildInstanceMods(ctx, instance, staging, restored); err != nil {
		slog.Warn("could not rebuild the restored instance mod records", "instance", instance.Name, "error", err)
	}

	s.finalizeRestore(instance, operation, size)
	return nil
}

// beginRestore creates the persisted operation record for a restore.
func (s *Service) beginRestore(instance InstanceRef, size int64) operations.Operation {
	now := s.now().UTC()
	resource := instance.ID
	operation := operations.Operation{
		ID:         s.newID(),
		Type:       "snapshot_restore",
		ResourceID: &resource,
		Title:      "Restoring snapshot",
		TitleKey:   TitleRestoringSnapshot,
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
func (s *Service) prepareRestoreStaging(
	ctx context.Context,
	instance InstanceRef,
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
		s.storage.DataDir(snapshotDir),
		staging,
		nil,
		progress,
		s.sanitizeSettings,
	); err != nil {
		_ = os.RemoveAll(staging)
		return "", fmt.Errorf("%w: %v", errRestoreStaging, err)
	}
	if err := os.WriteFile(filepath.Join(staging, instanceMarkerFile), []byte(instance.ID), 0o600); err != nil {
		_ = os.RemoveAll(staging)
		return "", err
	}
	if err := s.hardenLogs(filepath.Join(staging, "Logs")); err != nil {
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
func (s *Service) swapRestoredInstance(ctx context.Context, instance InstanceRef, staging string) error {
	parent := filepath.Dir(instance.Directory)
	previous := filepath.Join(parent, ".waxlight-restore-old-"+s.newID()[:12])
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
	if err := s.removeDirectory(previous); err != nil {
		slog.Warn("could not remove the replaced instance directory", "instance", instance.Name, "error", err)
	}
	return nil
}

// finalizeRestore applies the post-swap hardening and completes the operation.
func (s *Service) finalizeRestore(instance InstanceRef, operation operations.Operation, size int64) {
	if err := s.clearClientSettings(filepath.Join(instance.Directory, "clientsettings.json")); err != nil {
		slog.Warn("could not clear authentication from the restored instance", "instance", instance.Name, "error", err)
	}
	if err := s.hardenLogs(filepath.Join(instance.Directory, "Logs")); err != nil {
		slog.Warn("could not harden the restored instance logs", "instance", instance.Name, "error", err)
	}
	finished := s.now().UTC()
	operation.FinishedAt = &finished
	operation.Status = operations.StatusCompleted
	operation.Progress = 1
	operation.CurrentBytes = size
	operation.Title = "Restoring snapshot"
	operation.TitleKey = TitleRestoringSnapshot
	operation.TitleParams = nil
	s.operations.SaveBestEffort(operation, operations.EventCompleted)
	slog.Info("instance restored from snapshot", "instance", instance.Name)
}

type restoredMod struct {
	entry         Mod
	displayName   string
	installedPath string
}

// snapshotModsDirectoryFor maps an enabled flag to the standard instance mod
// directory name.
func snapshotModsDirectoryFor(enabled bool) string {
	if enabled {
		return "Mods"
	}
	return "ModsDisabled"
}

// modDownloadError carries the mod that failed during restore.
type modDownloadError struct {
	mod   Mod
	cause error
}

func (e *modDownloadError) Error() string {
	return fmt.Sprintf("could not restore the mod %s %s: %v", ModDisplayName(e.mod), e.mod.Version, e.cause)
}

func (e *modDownloadError) Unwrap() error { return e.cause }

// modRestoreMessage renders the user-facing failure message for a mod that
// could not be recovered.
func modRestoreMessage(err error) string {
	var downloadError *modDownloadError
	if !errors.As(err, &downloadError) {
		return "Snapshot could not be restored: " + err.Error()
	}
	mod := downloadError.mod
	if isAppErrorCode(downloadError.cause, domain.ErrModVersionNotFound) {
		return fmt.Sprintf(
			"Snapshot could not be restored. The following mod release is no longer available: %s %s",
			ModDisplayName(mod),
			mod.Version,
		)
	}
	detail := downloadError.cause.Error()
	if appError, ok := downloadError.cause.(*domain.AppError); ok {
		detail = appError.Message
	}
	return fmt.Sprintf(
		"Snapshot could not be restored. Could not download the mod %s %s: %s",
		ModDisplayName(mod),
		mod.Version,
		detail,
	)
}

// isAppErrorCode reports whether the error chain contains an AppError with
// the given code.
func isAppErrorCode(err error, code string) bool {
	var appError *domain.AppError
	return errors.As(err, &appError) && appError.Code == code
}

// restoreSnapshotMods downloads and installs every managed release of a
// snapshot into the staging directory. Downloads run with a bounded worker
// count; the first failure cancels the remaining work.
func (s *Service) restoreSnapshotMods(
	ctx context.Context,
	staging string,
	mods []Mod,
	operation *operations.Operation,
) ([]restoredMod, error) {
	downloadCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan restoredMod, len(mods))
	failures := make(chan modDownloadError, len(mods))
	semaphore := make(chan struct{}, snapshotModDownloadLimit)
	var completed atomic.Int64
	var progressMu sync.Mutex

	for index := range mods {
		semaphore <- struct{}{}
		go func(mod Mod) {
			defer func() { <-semaphore }()
			installed, err := s.restoreSnapshotMod(downloadCtx, staging, mod)
			done := completed.Add(1)
			if err != nil {
				failures <- modDownloadError{mod: mod, cause: err}
				return
			}
			progressMu.Lock()
			operation.Title = fmt.Sprintf("Restoring mods %d / %d", done, len(mods))
			operation.TitleKey = TitleRestoringModsProgress
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

	restored := make([]restoredMod, 0, len(mods))
	var firstFailure *modDownloadError
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
	mod Mod,
) (restoredMod, error) {
	downloaded, err := s.catalog.DownloadRelease(ctx, mod.ModID, mod.ReleaseID)
	if err != nil {
		return restoredMod{}, err
	}
	if mod.SHA256 != "" && downloaded.Checksum != "" && !strings.EqualFold(mod.SHA256, downloaded.Checksum) {
		return restoredMod{}, fmt.Errorf(
			"the downloaded release does not match the snapshot checksum",
		)
	}
	if info, infoErr := s.archiveInfo.ReadArchiveInfo(downloaded.FilePath); infoErr == nil {
		if strings.TrimSpace(info.ModID) != "" && strings.TrimSpace(mod.Identifier) != "" &&
			!strings.EqualFold(info.ModID, mod.Identifier) {
			slog.Warn("snapshot mod identifier mismatch", "snapshot", mod.Identifier, "archive", info.ModID, "modId", mod.ModID)
		}
		if strings.TrimSpace(info.Version) != "" && strings.TrimSpace(mod.Version) != "" &&
			!strings.EqualFold(info.Version, mod.Version) {
			slog.Warn("snapshot mod version mismatch", "snapshot", mod.Version, "archive", info.Version, "modId", mod.ModID)
		}
	}

	targetDir := filepath.Join(staging, snapshotModsDirectoryFor(mod.Enabled))
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return restoredMod{}, err
	}
	fileName := filepath.Base(downloaded.FilePath)
	target := filepath.Join(targetDir, fileName)
	if _, err := copySnapshotFileContext(ctx, downloaded.FilePath, target, 0o644); err != nil {
		return restoredMod{}, err
	}
	displayName := downloaded.Name
	if strings.TrimSpace(displayName) == "" {
		displayName = mod.Identifier
	}
	return restoredMod{
		entry:         mod,
		displayName:   displayName,
		installedPath: target,
	}, nil
}

// validateRestoredMods verifies that every expected mod file exists in the
// staging directory after the download phase.
func (s *Service) validateRestoredMods(staging string, restored []restoredMod) error {
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
	instance InstanceRef,
	staging string,
	restored []restoredMod,
) error {
	existing, err := s.mods.ListMods(ctx, instance.ID)
	if err != nil {
		return err
	}
	for _, stale := range existing {
		if err := s.mods.DeleteMod(ctx, stale.ID); err != nil {
			return err
		}
	}
	now := s.now().UTC()
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
		managed := restoredMod.entry.Source == ModSourceModDB
		if managed {
			source = ModDBSource(restoredMod.entry.ModID, restoredMod.entry.ReleaseID)
		}
		record := InstalledMod{
			ID:          s.newID(),
			InstanceID:  instance.ID,
			Name:        restoredMod.displayName,
			Version:     restoredMod.entry.Version,
			FileName:    filepath.Base(path),
			FilePath:    path,
			Enabled:     restoredMod.entry.Enabled,
			Managed:     managed,
			Source:      source,
			SizeBytes:   info.Size(),
			InstalledAt: now,
			UpdatedAt:   now,
		}
		if strings.TrimSpace(record.Name) == "" {
			record.Name = restoredMod.entry.Identifier
		}
		if err := s.mods.SaveMod(ctx, record); err != nil {
			return err
		}
	}
	return nil
}
