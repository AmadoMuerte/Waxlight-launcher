package application

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/telemetry"
)

const maximumLauncherUpdateSize = 512 * 1024 * 1024

type UpdateStage string

const (
	UpdateStageChecking           UpdateStage = "checking"
	UpdateStageDownloading        UpdateStage = "downloading"
	UpdateStageHashVerification   UpdateStage = "hash_verification"
	UpdateStageSignatureCheck     UpdateStage = "signature_verification"
	UpdateStageStartingInstaller  UpdateStage = "starting_installer"
	UpdateStageClosingApplication UpdateStage = "closing_application"
	UpdateStageRestarting         UpdateStage = "restarting"
)

type LauncherUpdateService struct {
	source            LauncherUpdateSource
	downloader        Downloader
	installer         LauncherUpdateInstaller
	signatureVerifier SignatureVerifier
	dataRoot          string
	currentVersion    string
	telemetry         *telemetry.Service
	mu                sync.Mutex
	installing        bool
}

func NewLauncherUpdateService(
	source LauncherUpdateSource,
	downloader Downloader,
	installer LauncherUpdateInstaller,
	signatureVerifier SignatureVerifier,
	dataRoot string,
	currentVersion string,
) *LauncherUpdateService {
	return &LauncherUpdateService{
		source:            source,
		downloader:        downloader,
		installer:         installer,
		signatureVerifier: signatureVerifier,
		dataRoot:          dataRoot,
		currentVersion:    currentVersion,
	}
}

// ConfigureTelemetry wires the telemetry service into the updater. Update
// events are strictly best-effort and never affect the update outcome.
func (service *LauncherUpdateService) ConfigureTelemetry(t *telemetry.Service) {
	service.telemetry = t
}

func (service *LauncherUpdateService) reportEvent(ctx context.Context, name string) {
	if service.telemetry != nil {
		service.telemetry.Event(ctx, name)
	}
}

func (service *LauncherUpdateService) reportError(ctx context.Context, code, operation string) {
	if service.telemetry != nil {
		service.telemetry.Error(ctx, code, telemetry.ComponentUpdater, operation)
	}
}

func (service *LauncherUpdateService) CurrentVersion() string {
	return service.currentVersion
}

func (service *LauncherUpdateService) Check(
	ctx context.Context,
	channel string,
) (domain.LauncherUpdate, error) {
	channel, err := normalizeUpdateChannel(channel)
	if err != nil {
		return domain.LauncherUpdate{}, err
	}
	slog.Info("checking for launcher updates", "channel", channel)
	update, err := service.source.Check(ctx, service.currentVersion, channel)
	if err != nil {
		slog.Warn("launcher update check failed", "error", err)
		return domain.LauncherUpdate{}, &domain.AppError{
			Code:      domain.ErrUpdateUnavailable,
			Message:   "Could not check for launcher updates",
			Retryable: true,
			Cause:     err,
		}
	}
	if update.Available {
		slog.Info("launcher update available", "version", update.Version)
	}
	return update, nil
}

func (service *LauncherUpdateService) Install(
	ctx context.Context,
	channel string,
	publish func(domain.LauncherUpdateProgress),
) error {
	service.mu.Lock()
	if service.installing {
		service.mu.Unlock()
		return domain.NewError(domain.ErrUpdateInProgress, "A launcher update is already running")
	}
	service.installing = true
	service.mu.Unlock()
	defer func() {
		service.mu.Lock()
		service.installing = false
		service.mu.Unlock()
	}()

	sessionID := fmt.Sprintf("%d", time.Now().UnixNano())
	updateRoot := filepath.Join(service.dataRoot, "updates", sessionID)
	if err := os.MkdirAll(updateRoot, 0o700); err != nil {
		return fmt.Errorf("create update session directory: %w", err)
	}

	publishProgress(publish, domain.LauncherUpdateProgress{Phase: "checking"})

	update, err := service.Check(ctx, channel)
	if err != nil {
		service.cleanupSession(updateRoot)
		return err
	}
	if !update.Available {
		service.cleanupSession(updateRoot)
		return domain.NewError(domain.ErrUpdateUnavailable, "No launcher update is available")
	}
	slog.Info("installing launcher update", "version", update.Version)
	if update.AssetName == "" || filepath.Base(update.AssetName) != update.AssetName {
		service.cleanupSession(updateRoot)
		return domain.NewError(domain.ErrUpdateFailed, "The release contains an unsafe update filename")
	}
	if len(update.SHA256) != 64 {
		service.cleanupSession(updateRoot)
		return domain.NewError(domain.ErrUpdateFailed, "The release checksum is invalid")
	}

	if update.InstallationMode == "portable" && runtime.GOOS == "windows" {
		service.cleanupSession(updateRoot)
		return &domain.AppError{
			Code:    domain.ErrUpdateUnsupported,
			Message: "Automatic replacement is unavailable for portable installations. Download the new portable package and replace the current version manually.",
			Cause:   nil,
		}
	}

	// update_started marks the moment the launcher update operation actually
	// begins: the update was detected, validated, and is about to be
	// downloaded. update_succeeded is deliberately NOT emitted here: the
	// platform installer applies the package asynchronously after this process
	// quits, so a successful completion cannot be reliably observed in
	// process. It is deferred to a future startup confirmation mechanism.
	service.reportEvent(ctx, telemetry.EventUpdateStarted)

	destination := filepath.Join(updateRoot, update.AssetName)

	progress := make(chan DownloadProgress, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for item := range progress {
			value := float64(0)
			if item.TotalBytes > 0 {
				value = float64(item.DownloadedBytes) / float64(item.TotalBytes)
			}
			publishProgress(publish, domain.LauncherUpdateProgress{
				Phase:           "downloading",
				DownloadedBytes: item.DownloadedBytes,
				TotalBytes:      item.TotalBytes,
				Progress:        value,
			})
		}
	}()
	publishProgress(publish, domain.LauncherUpdateProgress{Phase: "downloading"})
	err = service.downloader.Download(ctx, DownloadRequest{
		URL:               update.DownloadURL,
		DestinationPath:   destination,
		ExpectedChecksum:  strings.ToLower(update.SHA256),
		ChecksumAlgorithm: "sha256",
		Resume:            true,
		MaxBytes:          maximumLauncherUpdateSize,
	}, progress)
	close(progress)
	<-done
	if err != nil {
		service.cleanupSession(updateRoot)
		service.reportEvent(ctx, telemetry.EventUpdateFailed)
		service.reportError(ctx, telemetry.ErrorUpdateDownloadFailed, telemetry.OperationDownloadUpdate)
		return &domain.AppError{
			Code:      domain.ErrUpdateDownloadFailed,
			Message:   "Could not download the launcher update",
			Retryable: true,
			Cause:     err,
		}
	}

	publishProgress(publish, domain.LauncherUpdateProgress{Phase: "signature", Progress: 1})
	if err := service.signatureVerifier.Verify(ctx, destination); err != nil {
		os.Remove(destination)
		service.cleanupSession(updateRoot)
		service.reportEvent(ctx, telemetry.EventUpdateFailed)
		service.reportError(ctx, telemetry.ErrorUpdateSignatureInvalid, telemetry.OperationInstallUpdate)
		return &domain.AppError{
			Code:    domain.ErrUpdateSignatureInvalid,
			Message: fmt.Sprintf("Could not verify update signature: %v", err),
			Cause:   err,
		}
	}

	publishProgress(publish, domain.LauncherUpdateProgress{Phase: "installing", Progress: 1})
	if err := service.installer.Apply(ctx, destination, os.Getpid()); err != nil {
		service.cleanupSession(updateRoot)
		service.reportEvent(ctx, telemetry.EventUpdateFailed)
		service.reportError(ctx, telemetry.ErrorUpdateInstallFailed, telemetry.OperationInstallUpdate)
		return &domain.AppError{
			Code:    domain.ErrUpdateInstallerStartFail,
			Message: fmt.Sprintf("Could not start the installer: %v", err),
			Cause:   err,
		}
	}
	publishProgress(publish, domain.LauncherUpdateProgress{Phase: "restarting", Progress: 1})
	slog.Info("launcher update applied", "version", update.Version)
	return nil
}

func (service *LauncherUpdateService) cleanupSession(sessionDir string) {
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		os.Remove(filepath.Join(sessionDir, entry.Name()))
	}
	os.Remove(sessionDir)
}

// PurgeStaleUpdateSessions removes every leftover update session directory under
// dataRoot/updates. A successful launcher update intentionally leaves its
// session directory behind so the platform installer can consume the package
// asynchronously; without cleanup these directories accumulate on disk. At
// startup no session belongs to the current process, so all leftovers are stale
// and safe to remove. A missing updates directory is not an error.
func PurgeStaleUpdateSessions(dataRoot string) error {
	return os.RemoveAll(filepath.Join(dataRoot, "updates"))
}

func normalizeUpdateChannel(channel string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "", "stable":
		return "stable", nil
	case "prerelease":
		return "prerelease", nil
	default:
		return "", domain.NewError(domain.ErrValidation, "Update channel must be stable or prerelease")
	}
}

func publishProgress(publish func(domain.LauncherUpdateProgress), progress domain.LauncherUpdateProgress) {
	if publish != nil {
		publish(progress)
	}
}
