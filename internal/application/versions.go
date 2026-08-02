package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

func (s *Service) ListAvailableVersions(
	ctx context.Context,
) ([]domain.AvailableGameVersion, error) {
	if s.versionCatalog == nil {
		return nil, domain.NewError(
			domain.ErrVersionCatalog,
			"The game version catalog is not configured",
		)
	}

	available, err := s.versionCatalog.List(ctx)
	if err != nil {
		return nil, &domain.AppError{
			Code:      domain.ErrVersionCatalog,
			Message:   "Could not load the official game version catalog",
			Retryable: true,
			Cause:     err,
		}
	}

	installed, err := s.store.ListVersions(ctx)
	if err != nil {
		return nil, err
	}
	installedByID := make(map[string]domain.GameVersion, len(installed))
	for _, version := range installed {
		installedByID[version.ID] = version
	}

	for index := range available {
		version, ok := installedByID[available[index].ID]
		if !ok {
			continue
		}
		available[index].Installed = true
		status := version.Status
		available[index].InstallStatus = &status
	}

	return available, nil
}

// InstallAvailableVersion starts a service-owned, cancellable operation. The
// returned operation is persisted before background work begins, so the UI can
// immediately track it through the operations API.
func (s *Service) InstallAvailableVersion(
	ctx context.Context,
	versionID string,
) (domain.Operation, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return domain.Operation{}, domain.NewError(
			domain.ErrValidation,
			"Select a game version",
		)
	}
	if s.downloader == nil || s.packageInstaller == nil {
		return domain.Operation{}, domain.NewError(
			domain.ErrVersionCatalog,
			"Game version downloads are not configured",
		)
	}

	if _, err := s.store.GetVersion(ctx, versionID); err == nil {
		return domain.Operation{}, domain.NewError(
			domain.ErrVersionExists,
			"This game version is already installed",
		)
	} else if !isAppErrorCode(err, domain.ErrVersionNotFound) {
		return domain.Operation{}, err
	}

	versions, err := s.ListAvailableVersions(ctx)
	if err != nil {
		return domain.Operation{}, err
	}
	var selected *domain.AvailableGameVersion
	for index := range versions {
		if versions[index].ID == versionID {
			selected = &versions[index]
			break
		}
	}
	if selected == nil {
		return domain.Operation{}, domain.NewError(
			domain.ErrVersionNotFound,
			"The selected version is not available for this platform",
		)
	}
	if s.diskSpace != nil && selected.DownloadSize > 0 {
		availableBytes, err := s.diskSpace.Available(s.dataRoot)
		if err != nil {
			return domain.Operation{}, &domain.AppError{
				Code:    domain.ErrFilePermission,
				Message: "Could not check available disk space",
				Cause:   err,
			}
		}
		// Keep room for both the downloaded package and its extracted files.
		requiredBytes := selected.DownloadSize * 2
		if availableBytes < requiredBytes {
			return domain.Operation{}, domain.NewError(
				domain.ErrInsufficientSpace,
				"Not enough disk space to download and install this version",
			)
		}
	}

	now := time.Now().UTC()
	resourceID := selected.ID
	operation := domain.Operation{
		ID:         newID(),
		Type:       "game_version_download",
		ResourceID: &resourceID,
		Title:      "Downloading Vintage Story " + selected.Name,
		Status:     "queued",
		TotalBytes: selected.DownloadSize,
		CreatedAt:  now,
	}
	s.operationsMu.Lock()
	if _, installing := s.versionOperations[selected.ID]; installing {
		s.operationsMu.Unlock()
		return domain.Operation{}, domain.NewError(
			domain.ErrVersionExists,
			"This game version is already being installed",
		)
	}
	s.versionOperations[selected.ID] = operation.ID
	s.operationsMu.Unlock()
	if err := s.store.SaveOperation(ctx, operation); err != nil {
		s.operationsMu.Lock()
		delete(s.versionOperations, selected.ID)
		s.operationsMu.Unlock()
		return domain.Operation{}, err
	}
	s.emit("operation:created", operation)

	operationContext, cancel := context.WithCancel(s.shutdownCtx)
	done := make(chan error, 1)
	s.operationsMu.Lock()
	s.operationCancels[operation.ID] = cancel
	s.operationDone[operation.ID] = done
	s.operationsMu.Unlock()

	s.operationWG.Add(1)
	go func(release domain.AvailableGameVersion, operation domain.Operation) {
		defer s.operationWG.Done()
		defer cancel()
		operationErr := s.runAvailableVersionInstall(
			operationContext,
			release,
			operation,
		)
		s.operationsMu.Lock()
		delete(s.operationCancels, operation.ID)
		delete(s.operationDone, operation.ID)
		delete(s.versionOperations, release.ID)
		s.operationsMu.Unlock()
		done <- operationErr
		close(done)
	}(*selected, operation)

	return operation, nil
}

func (s *Service) CancelOperation(operationID string) error {
	s.operationsMu.Lock()
	cancel, ok := s.operationCancels[operationID]
	done := s.operationDone[operationID]
	s.operationsMu.Unlock()
	if !ok {
		return domain.NewError(
			domain.ErrOperationNotFound,
			"The operation is no longer running",
		)
	}
	cancel()
	if cleanupErr := <-done; cleanupErr != nil {
		return &domain.AppError{
			Code:    domain.ErrFilePermission,
			Message: "Could not fully clean up the cancelled operation",
			Cause:   cleanupErr,
		}
	}
	return nil
}

func (s *Service) runAvailableVersionInstall(
	ctx context.Context,
	release domain.AvailableGameVersion,
	operation domain.Operation,
) error {
	now := time.Now().UTC()
	operation.StartedAt = &now
	operation.Status = "running"
	s.saveOperation(operation, "operation:updated")

	downloadPath := filepath.Join(
		s.dataRoot,
		"downloads",
		release.Filename,
	)
	progress := make(chan DownloadProgress, 8)
	downloadResult := make(chan error, 1)
	go func() {
		downloadResult <- s.downloader.Download(
			ctx,
			DownloadRequest{
				URL:               release.DownloadURL,
				DestinationPath:   downloadPath,
				ExpectedChecksum:  release.Checksum,
				ChecksumAlgorithm: release.ChecksumAlgorithm,
				Resume:            true,
			},
			progress,
		)
	}()

	lastSaved := time.Time{}
	downloadFinished := false
	for !downloadFinished {
		select {
		case update := <-progress:
			operation.CurrentBytes = update.DownloadedBytes
			operation.TotalBytes = update.TotalBytes
			operation.BytesPerSecond = update.BytesPerSecond
			if update.TotalBytes > 0 {
				operation.Progress = 0.85 * float64(update.DownloadedBytes) /
					float64(update.TotalBytes)
			}
			if time.Since(lastSaved) >= 250*time.Millisecond {
				s.saveOperation(operation, "operation:progress")
				lastSaved = time.Now()
			}
		case err := <-downloadResult:
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return s.removeCancelledVersionOperation(operation, downloadPath)
				}
				s.finishVersionOperation(operation, err, domain.ErrDownloadFailed)
				return nil
			}
			downloadFinished = true
		}
	}

	if err := ctx.Err(); err != nil {
		return s.removeCancelledVersionOperation(operation, downloadPath)
	}

	operation.Type = "game_version_install"
	operation.Title = "Installing Vintage Story " + release.Name
	operation.Progress = 0.9
	operation.BytesPerSecond = 0
	s.saveOperation(operation, "operation:updated")

	targetPath := filepath.Join(s.dataRoot, "versions", safeSegment(release.ID))
	executablePath, size, err := s.packageInstaller.Install(
		ctx,
		downloadPath,
		targetPath,
	)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return s.removeCancelledVersionOperation(operation, downloadPath)
		}
		s.finishVersionOperation(operation, err, domain.ErrArchiveInvalid)
		return nil
	}
	if err := ctx.Err(); err != nil {
		return s.removeCancelledVersionOperation(operation, downloadPath)
	}
	if err := os.WriteFile(
		filepath.Join(targetPath, ".waxlight-version"),
		[]byte(release.ID),
		0o600,
	); err != nil {
		s.finishVersionOperation(operation, err, domain.ErrFilePermission)
		return nil
	}

	installedAt := time.Now().UTC()
	version := domain.GameVersion{
		ID:              release.ID,
		Name:            release.Name,
		Channel:         release.Channel,
		Platform:        release.Platform,
		Architecture:    release.Architecture,
		InstallationDir: targetPath,
		ExecutablePath:  executablePath,
		Status:          "installed",
		InstalledAt:     installedAt,
		VerifiedAt:      &installedAt,
		SizeBytes:       size,
	}
	if err := s.store.SaveVersion(ctx, version); err != nil {
		if errors.Is(err, context.Canceled) {
			return s.removeCancelledVersionOperation(operation, downloadPath)
		}
		s.finishVersionOperation(operation, err, domain.ErrFilePermission)
		return nil
	}

	operation.Status = "completed"
	operation.Progress = 1
	operation.CurrentBytes = operation.TotalBytes
	operation.FinishedAt = &installedAt
	s.saveOperation(operation, "operation:completed")
	s.emit("version:installed", version)
	_ = os.Remove(downloadPath)
	return nil
}

func (s *Service) removeCancelledVersionOperation(
	operation domain.Operation,
	downloadPath string,
) error {
	for _, path := range []string{downloadPath + ".partial", downloadPath} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.finishVersionOperation(operation, err, domain.ErrFilePermission)
			return err
		}
	}
	// The guarded store method only deletes terminal operations, so transition
	// without publishing a transient cancelled row to the UI and remove it next.
	finishedAt := time.Now().UTC()
	operation.Status = "cancelled"
	operation.FinishedAt = &finishedAt
	operation.BytesPerSecond = 0
	if err := s.store.SaveOperation(context.Background(), operation); err != nil {
		return err
	}
	if err := s.store.DeleteFinishedOperation(
		context.Background(),
		operation.ID,
	); err != nil {
		return err
	}
	s.emit("operation:removed", map[string]string{"id": operation.ID})
	return nil
}

func (s *Service) finishVersionOperation(
	operation domain.Operation,
	err error,
	defaultCode string,
) {
	finishedAt := time.Now().UTC()
	operation.FinishedAt = &finishedAt
	operation.BytesPerSecond = 0
	operation.Status = "failed"
	code := defaultCode
	if strings.Contains(strings.ToLower(err.Error()), "checksum") {
		code = domain.ErrChecksumMismatch
	}
	operation.ErrorCode = &code
	message := err.Error()
	operation.ErrorMessage = &message
	s.saveOperation(operation, "operation:failed")
}

func (s *Service) saveOperation(operation domain.Operation, event string) {
	_ = s.store.SaveOperation(context.Background(), operation)
	s.emit(event, operation)
}

func isAppErrorCode(err error, code string) bool {
	var appError *domain.AppError
	return errors.As(err, &appError) && appError.Code == code
}
