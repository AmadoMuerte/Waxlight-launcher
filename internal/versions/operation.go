package versions

import (
	"context"
	"errors"
	"strings"

	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/operations"
)

type Install struct {
	Operation operations.Operation
	future    operations.Future[GameVersion]
}

func (install Install) Wait(ctx context.Context) (GameVersion, error) {
	return install.future.Wait(ctx)
}

func (runtime *InstallRuntime) fail(operation *operations.Operation, err error, defaultCode string) {
	finished := runtime.now().UTC()
	operation.FinishedAt = &finished
	operation.BytesPerSecond = 0
	operation.Status = operations.StatusFailed
	code := errorCode(err, defaultCode)
	operation.ErrorCode = &code
	message := err.Error()
	operation.ErrorMessage = &message
	runtime.operations.SaveBestEffort(*operation, operations.EventFailed)
}

func errorCode(err error, defaultCode string) string {
	if strings.Contains(strings.ToLower(err.Error()), "checksum") {
		return errs.ErrChecksumMismatch
	}
	return defaultCode
}

func (runtime *InstallRuntime) cancel(operation operations.Operation, downloadPath string) error {
	if downloadPath != "" {
		if err := runtime.filesystem.RemoveDownload(downloadPath); err != nil {
			runtime.fail(&operation, err, errs.ErrFilePermission)
			return err
		}
	}
	return runtime.finishCancellation(operation)
}

func (runtime *InstallRuntime) cancelInstall(operation operations.Operation, downloadPath, target, id string) error {
	if err := runtime.filesystem.RemoveInstallTarget(target, id); err != nil {
		runtime.fail(&operation, err, errs.ErrFilePermission)
		return err
	}
	return runtime.cancel(operation, downloadPath)
}

func (runtime *InstallRuntime) finishCancellation(operation operations.Operation) error {
	finished := runtime.now().UTC()
	operation.Status = operations.StatusCancelled
	operation.FinishedAt = &finished
	operation.BytesPerSecond = 0
	if err := runtime.operations.Save(context.Background(), operation, ""); err != nil {
		return err
	}
	if err := runtime.operations.Delete(context.Background(), operation.ID); err != nil {
		return err
	}
	runtime.operations.Publish(operations.EventRemoved, map[string]string{"id": operation.ID})
	return context.Canceled
}

func (runtime *InstallRuntime) failAndClean(operation *operations.Operation, target, id string, err error, defaultCode string) error {
	if cleanupErr := runtime.filesystem.RemoveInstallTarget(target, id); cleanupErr != nil {
		runtime.fail(operation, cleanupErr, errs.ErrFilePermission)
		return cleanupErr
	}
	runtime.fail(operation, err, defaultCode)
	return err
}

func cancelled(err error) bool { return errors.Is(err, context.Canceled) }
