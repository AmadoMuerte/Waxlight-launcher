package versions

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/operations"
)

type localInstallResult struct {
	operation operations.Operation
	version   GameVersion
}

type LocalInstallService struct {
	repository     Repository
	localInstaller LocalInstaller
	runtime        *InstallRuntime
	platform       string
	architecture   string
}

func NewLocalInstallService(
	repository Repository,
	installer LocalInstaller,
	runtime *InstallRuntime,
	platform string,
	architecture string,
) *LocalInstallService {
	return &LocalInstallService{repository: repository, localInstaller: installer, runtime: runtime, platform: platform, architecture: architecture}
}

func (service *LocalInstallService) InstallLocal(
	ctx context.Context,
	id, name, sourcePath, executableRelativePath, checksum string,
) (operations.Operation, error) {
	if err := service.runtime.gate.Begin(); err != nil {
		return operations.Operation{}, err
	}
	defer service.runtime.gate.End()
	var err error
	id, err = validateID(id)
	if err != nil {
		return operations.Operation{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = id
	}
	if strings.TrimSpace(sourcePath) == "" {
		return operations.Operation{}, domain.NewError(domain.ErrValidation, "Select a game archive or directory")
	}
	if err := ensureMissing(ctx, service.repository, id); err != nil {
		return operations.Operation{}, err
	}

	now := service.runtime.now().UTC()
	resource := id
	operation := operations.Operation{
		ID: service.runtime.newID(), Type: "game_version_install", ResourceID: &resource,
		Title: "Installing Vintage Story " + name, TitleKey: titleInstalling,
		TitleParams: titleParams(name), Status: operations.StatusRunning,
		Progress: 0.05, CreatedAt: now, StartedAt: &now,
	}
	future, err := operations.Start(service.runtime.operations, ctx, operation, operationKey(id), func(workerCtx context.Context) (localInstallResult, error) {
		return service.runLocalInstall(workerCtx, operation, id, name, sourcePath, executableRelativePath, checksum)
	})
	if errors.Is(err, operations.ErrKeyActive) {
		return operations.Operation{}, domain.NewError(domain.ErrVersionExists, "This game version is already being installed")
	}
	if err != nil {
		return operations.Operation{}, err
	}
	result, err := future.Wait(ctx)
	if err != nil {
		if ctx.Err() != nil {
			_ = service.runtime.operations.Cancel(operation.ID)
		}
		if result.operation.ID == "" {
			result.operation = operation
		}
		return result.operation, err
	}
	return result.operation, nil
}

func (service *LocalInstallService) runLocalInstall(
	ctx context.Context,
	operation operations.Operation,
	id, name, sourcePath, executableRelativePath, checksum string,
) (localInstallResult, error) {
	target := service.runtime.filesystem.VersionPath(id)
	lastSaved := service.runtime.now()
	executable, size, err := service.localInstaller.Install(ctx, sourcePath, target, executableRelativePath, checksum, func(copied, total int64) {
		if total > 0 {
			operation.Progress = 0.05 + 0.85*float64(copied)/float64(total)
		}
		operation.CurrentBytes, operation.TotalBytes, operation.BytesPerSecond = copied, total, 0
		service.runtime.operations.Publish(operations.EventProgress, operation)
		if service.runtime.now().Sub(lastSaved) >= 250*time.Millisecond {
			lastSaved = service.runtime.now()
			service.runtime.operations.Persist(operation)
		}
	})
	finished := service.runtime.now().UTC()
	operation.FinishedAt = &finished
	if err != nil {
		if cancelled(err) {
			return localInstallResult{operation: operation}, service.runtime.cancelInstall(operation, "", target, id)
		}
		err = service.runtime.failAndClean(&operation, target, id, err, domain.ErrArchiveInvalid)
		return localInstallResult{operation: operation}, &domain.AppError{Code: errorCode(err, domain.ErrArchiveInvalid), Message: "Failed to install the game version", Cause: err}
	}
	if ctx.Err() != nil {
		return localInstallResult{operation: operation}, service.runtime.cancelInstall(operation, "", target, id)
	}
	if err := service.runtime.filesystem.WriteMarker(target, id); err != nil {
		return localInstallResult{operation: operation}, service.runtime.failAndClean(&operation, target, id, err, domain.ErrFilePermission)
	}
	version := GameVersion{
		ID: id, Name: name, Channel: "unknown", Platform: service.platform, Architecture: service.architecture,
		InstallationDir: target, ExecutablePath: executable, Status: "installed", InstalledAt: finished,
		VerifiedAt: &finished, SizeBytes: size,
	}
	if err := service.repository.SaveVersion(ctx, version); err != nil {
		if cancelled(err) {
			return localInstallResult{operation: operation}, service.runtime.cancelInstall(operation, "", target, id)
		}
		return localInstallResult{operation: operation}, service.runtime.failAndClean(&operation, target, id, err, domain.ErrFilePermission)
	}
	operation.Status, operation.Progress = operations.StatusCompleted, 1
	operation.TotalBytes, operation.CurrentBytes = size, size
	service.runtime.operations.SaveBestEffort(operation, operations.EventCompleted)
	return localInstallResult{operation: operation, version: version}, nil
}

func operationKey(id string) string { return "game-version:" + id }

func ensureMissing(ctx context.Context, repository Repository, id string) error {
	if _, err := repository.GetVersion(ctx, id); err == nil {
		return domain.NewError(domain.ErrVersionExists, "This game version is already installed")
	} else if !isCode(err, domain.ErrVersionNotFound) {
		return err
	}
	return nil
}
