package versions

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/downloads"
	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/events"
	"github.com/waxlight/waxlight-launcher/internal/operations"
)

type CatalogInstallService struct {
	repository       Repository
	query            *QueryService
	downloader       Downloader
	packageInstaller PackageInstaller
	diskSpace        DiskSpace
	runtime          *InstallRuntime
	events           events.Publisher
	dataRoot         string
}

func NewCatalogInstallService(
	repository Repository,
	query *QueryService,
	downloader Downloader,
	installer PackageInstaller,
	diskSpace DiskSpace,
	runtime *InstallRuntime,
	publisher events.Publisher,
	dataRoot string,
) *CatalogInstallService {
	return &CatalogInstallService{
		repository: repository, query: query, downloader: downloader, packageInstaller: installer,
		diskSpace: diskSpace, runtime: runtime, events: publisher, dataRoot: dataRoot,
	}
}

func (service *CatalogInstallService) InstallCatalog(ctx context.Context, versionID string) (Install, error) {
	if err := service.runtime.gate.Begin(); err != nil {
		return Install{}, err
	}
	releaseOnReturn := true
	defer func() {
		if releaseOnReturn {
			service.runtime.gate.End()
		}
	}()
	var err error
	versionID, err = validateID(versionID)
	if err != nil {
		return Install{}, err
	}
	if service.downloader == nil || service.packageInstaller == nil {
		return Install{}, errs.NewError(errs.ErrVersionCatalog, "Game version downloads are not configured")
	}
	if err := ensureMissing(ctx, service.repository, versionID); err != nil {
		return Install{}, err
	}
	available, err := service.query.ListAvailable(ctx)
	if err != nil {
		return Install{}, err
	}
	var selected *AvailableGameVersion
	for index := range available {
		if available[index].ID == versionID {
			selected = &available[index]
			break
		}
	}
	if selected == nil {
		return Install{}, errs.NewError(errs.ErrVersionNotFound, "The selected version is not available for this platform")
	}
	if _, err := validateID(selected.ID); err != nil {
		return Install{}, errs.NewError(errs.ErrVersionCatalog, "The game version catalog contains an invalid version ID")
	}
	if err := validateCatalogFilename(selected.Filename); err != nil {
		return Install{}, err
	}
	if err := service.checkDiskSpace(*selected); err != nil {
		return Install{}, err
	}

	now := service.runtime.now().UTC()
	resource := selected.ID
	operation := operations.Operation{
		ID: service.runtime.newID(), Type: "game_version_download", ResourceID: &resource,
		Title: "Downloading Vintage Story " + selected.Name, TitleKey: titleDownloading,
		TitleParams: titleParams(selected.Name), Status: operations.StatusQueued,
		TotalBytes: selected.DownloadSize, CreatedAt: now,
	}
	release := *selected
	gateOwnership := make(chan struct{})
	future, err := operations.Start(service.runtime.operations, ctx, operation, operationKey(selected.ID), func(workerCtx context.Context) (GameVersion, error) {
		<-gateOwnership
		defer service.runtime.gate.End()
		return service.runCatalogInstall(workerCtx, release, operation)
	})
	if errors.Is(err, operations.ErrKeyActive) {
		return Install{}, errs.NewError(errs.ErrVersionExists, "This game version is already being installed")
	}
	if err != nil {
		return Install{}, err
	}
	releaseOnReturn = false
	close(gateOwnership)
	return Install{Operation: operation, future: future}, nil
}

func (service *CatalogInstallService) InstallCatalogAndWait(ctx context.Context, versionID string) (GameVersion, error) {
	install, err := service.InstallCatalog(ctx, versionID)
	if err != nil {
		return GameVersion{}, err
	}
	version, err := install.Wait(ctx)
	if err != nil {
		return GameVersion{}, &errs.AppError{Code: errs.ErrVersionInstall, Message: "Could not install the required game version", Cause: err}
	}
	if version.ID == "" {
		return GameVersion{}, errs.NewError(errs.ErrVersionInstall, "Could not install the required game version")
	}
	return version, nil
}

func (service *CatalogInstallService) checkDiskSpace(release AvailableGameVersion) error {
	if service.diskSpace == nil || release.DownloadSize <= 0 {
		return nil
	}
	available, err := service.diskSpace.Available(service.dataRoot)
	if err != nil {
		return &errs.AppError{Code: errs.ErrFilePermission, Message: "Could not check available disk space", Cause: err}
	}
	if available < release.DownloadSize*2 {
		return errs.NewError(errs.ErrInsufficientSpace, "Not enough disk space to download and install this version")
	}
	return nil
}

func (service *CatalogInstallService) runCatalogInstall(ctx context.Context, release AvailableGameVersion, operation operations.Operation) (GameVersion, error) {
	now := service.runtime.now().UTC()
	operation.StartedAt, operation.Status = &now, operations.StatusRunning
	service.runtime.operations.SaveBestEffort(operation, operations.EventUpdated)
	downloadPath := service.runtime.filesystem.DownloadPath(release.Filename)
	progress := make(chan downloads.Progress, 8)
	result := make(chan error, 1)
	go func() {
		result <- service.downloader.Download(ctx, downloads.Request{
			URL: release.DownloadURL, DestinationPath: downloadPath,
			ExpectedChecksum: release.Checksum, ChecksumAlgorithm: release.ChecksumAlgorithm, Resume: true,
		}, progress)
	}()

	lastSaved := time.Time{}
	for {
		select {
		case update := <-progress:
			operation.CurrentBytes, operation.TotalBytes = update.DownloadedBytes, update.TotalBytes
			operation.BytesPerSecond = update.BytesPerSecond
			if update.TotalBytes > 0 {
				operation.Progress = 0.85 * float64(update.DownloadedBytes) / float64(update.TotalBytes)
			}
			service.runtime.operations.Publish(operations.EventProgress, operation)
			if service.runtime.now().Sub(lastSaved) >= 250*time.Millisecond {
				service.runtime.operations.Persist(operation)
				lastSaved = service.runtime.now()
			}
		case err := <-result:
			if err != nil {
				if cancelled(err) {
					return GameVersion{}, service.runtime.cancel(operation, downloadPath)
				}
				service.runtime.fail(&operation, err, errs.ErrDownloadFailed)
				return GameVersion{}, err
			}
			return service.installDownloaded(ctx, release, downloadPath, operation, lastSaved)
		}
	}
}

func (service *CatalogInstallService) installDownloaded(ctx context.Context, release AvailableGameVersion, downloadPath string, operation operations.Operation, lastSaved time.Time) (GameVersion, error) {
	if ctx.Err() != nil {
		return GameVersion{}, service.runtime.cancel(operation, downloadPath)
	}
	operation.Type = "game_version_install"
	operation.Title, operation.TitleKey = "Installing Vintage Story "+release.Name, titleInstalling
	if release.Platform == "windows" {
		operation.Title = "Installing Vintage Story " + release.Name + " - choose No if Setup asks to uninstall the old version"
		operation.TitleKey = titleInstallingWindows
	}
	operation.TitleParams, operation.Progress, operation.BytesPerSecond = titleParams(release.Name), 0.9, 0
	service.runtime.operations.SaveBestEffort(operation, operations.EventUpdated)
	target := service.runtime.filesystem.VersionPath(release.ID)
	executable, size, err := service.packageInstaller.Install(ctx, downloadPath, target, func(copied, total int64) {
		if total > 0 {
			operation.Progress = 0.9 + 0.1*float64(copied)/float64(total)
		}
		operation.CurrentBytes, operation.TotalBytes, operation.BytesPerSecond = copied, total, 0
		service.runtime.operations.Publish(operations.EventProgress, operation)
		if service.runtime.now().Sub(lastSaved) >= 250*time.Millisecond {
			service.runtime.operations.Persist(operation)
			lastSaved = service.runtime.now()
		}
	})
	if err != nil {
		if cancelled(err) {
			return GameVersion{}, service.runtime.cancelInstall(operation, downloadPath, target, release.ID)
		}
		return GameVersion{}, service.runtime.failAndClean(&operation, target, release.ID, err, errs.ErrArchiveInvalid)
	}
	if ctx.Err() != nil {
		return GameVersion{}, service.runtime.cancelInstall(operation, downloadPath, target, release.ID)
	}
	if err := service.runtime.filesystem.WriteMarker(target, release.ID); err != nil {
		return GameVersion{}, service.runtime.failAndClean(&operation, target, release.ID, err, errs.ErrFilePermission)
	}
	installedAt := service.runtime.now().UTC()
	version := GameVersion{
		ID: release.ID, Name: release.Name, Channel: release.Channel, Platform: release.Platform,
		Architecture: release.Architecture, InstallationDir: target, ExecutablePath: executable,
		Status: "installed", InstalledAt: installedAt, VerifiedAt: &installedAt, SizeBytes: size,
	}
	if err := service.repository.SaveVersion(ctx, version); err != nil {
		if cancelled(err) {
			return GameVersion{}, service.runtime.cancelInstall(operation, downloadPath, target, release.ID)
		}
		return GameVersion{}, service.runtime.failAndClean(&operation, target, release.ID, err, errs.ErrFilePermission)
	}
	operation.Status, operation.Progress = operations.StatusCompleted, 1
	operation.CurrentBytes, operation.FinishedAt = operation.TotalBytes, &installedAt
	service.runtime.operations.SaveBestEffort(operation, operations.EventCompleted)
	service.publish("version:installed", version)
	if err := service.runtime.filesystem.RemoveDownload(downloadPath); err != nil {
		slog.Debug("could not remove the downloaded archive", "path", downloadPath, "error", err)
	}
	return version, nil
}

func (service *CatalogInstallService) publish(name string, payload any) {
	if service.events != nil {
		service.events.Publish(name, payload)
	}
}
