package updates

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

	"github.com/waxlight/waxlight-launcher/internal/downloads"
	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/telemetry"
)

// Service owns launcher update checks, verified downloads, and installation
// orchestration. All dependencies are immutable at construction; telemetry is
// strictly best-effort and never affects the update outcome.
type Service struct {
	source            Source
	downloader        downloads.Downloader
	installer         Installer
	signatureVerifier SignatureVerifier
	mutationGate      MutationGate
	dataRoot          string
	currentVersion    string
	telemetry         Telemetry
	mu                sync.Mutex
	installing        bool
}

func NewService(
	source Source,
	downloader downloads.Downloader,
	installer Installer,
	signatureVerifier SignatureVerifier,
	mutationGate MutationGate,
	dataRoot string,
	currentVersion string,
	telemetry Telemetry,
) *Service {
	return &Service{
		source:            source,
		downloader:        downloader,
		installer:         installer,
		signatureVerifier: signatureVerifier,
		mutationGate:      mutationGate,
		dataRoot:          dataRoot,
		currentVersion:    currentVersion,
		telemetry:         telemetry,
	}
}

func (service *Service) reportEvent(ctx context.Context, name string) {
	if service.telemetry != nil {
		service.telemetry.Event(ctx, name)
	}
}

func (service *Service) reportError(ctx context.Context, code, operation string) {
	if service.telemetry != nil {
		service.telemetry.Error(ctx, code, telemetry.ComponentUpdater, operation)
	}
}

func (service *Service) CurrentVersion() string {
	return service.currentVersion
}

func (service *Service) Check(
	ctx context.Context,
	channel string,
) (Update, error) {
	channel, err := normalizeUpdateChannel(channel)
	if err != nil {
		return Update{}, err
	}
	slog.Info("checking for launcher updates", "channel", channel)
	update, err := service.source.Check(ctx, service.currentVersion, channel)
	if err != nil {
		slog.Warn("launcher update check failed", "error", err)
		return Update{}, &errs.AppError{
			Code:      ErrUpdateUnavailable,
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

func (service *Service) Install(
	ctx context.Context,
	channel string,
	publish func(Progress),
) error {
	if err := service.mutationGate.Begin(); err != nil {
		return err
	}
	defer service.mutationGate.End()
	service.mu.Lock()
	if service.installing {
		service.mu.Unlock()
		return errs.NewError(ErrUpdateInProgress, "A launcher update is already running")
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

	publishProgress(publish, Progress{Phase: string(StageChecking)})

	update, err := service.Check(ctx, channel)
	if err != nil {
		service.cleanupSession(updateRoot)
		return err
	}
	if !update.Available {
		service.cleanupSession(updateRoot)
		return errs.NewError(ErrUpdateUnavailable, "No launcher update is available")
	}
	slog.Info("installing launcher update", "version", update.Version)
	if update.AssetName == "" || filepath.Base(update.AssetName) != update.AssetName {
		service.cleanupSession(updateRoot)
		return errs.NewError(ErrUpdateFailed, "The release contains an unsafe update filename")
	}
	if len(update.SHA256) != 64 {
		service.cleanupSession(updateRoot)
		return errs.NewError(ErrUpdateFailed, "The release checksum is invalid")
	}

	if update.InstallationMode == "portable" && runtime.GOOS == "windows" {
		service.cleanupSession(updateRoot)
		return &errs.AppError{
			Code:    ErrUpdateUnsupported,
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

	progress := make(chan downloads.Progress, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for item := range progress {
			value := float64(0)
			if item.TotalBytes > 0 {
				value = float64(item.DownloadedBytes) / float64(item.TotalBytes)
			}
			publishProgress(publish, Progress{
				Phase:           string(StageDownloading),
				DownloadedBytes: item.DownloadedBytes,
				TotalBytes:      item.TotalBytes,
				Progress:        value,
			})
		}
	}()
	publishProgress(publish, Progress{Phase: string(StageDownloading)})
	err = service.downloader.Download(ctx, downloads.Request{
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
		slog.Error("launcher update download failed", "version", update.Version, "error", err)
		service.reportEvent(ctx, telemetry.EventUpdateFailed)
		service.reportError(ctx, telemetry.ErrorUpdateDownloadFailed, telemetry.OperationDownloadUpdate)
		return &errs.AppError{
			Code:      ErrUpdateDownloadFailed,
			Message:   "Could not download the launcher update",
			Retryable: true,
			Cause:     err,
		}
	}

	publishProgress(publish, Progress{Phase: string(StageSignature), Progress: 1})
	if err := service.signatureVerifier.Verify(ctx, destination); err != nil {
		os.Remove(destination)
		service.cleanupSession(updateRoot)
		slog.Error("launcher update signature verification failed", "version", update.Version, "error", err)
		service.reportEvent(ctx, telemetry.EventUpdateFailed)
		service.reportError(ctx, telemetry.ErrorUpdateSignatureInvalid, telemetry.OperationInstallUpdate)
		return &errs.AppError{
			Code:    ErrUpdateSignatureInvalid,
			Message: fmt.Sprintf("Could not verify update signature: %v", err),
			Cause:   err,
		}
	}

	publishProgress(publish, Progress{Phase: string(StageInstalling), Progress: 1})
	if err := service.installer.Apply(ctx, destination, os.Getpid()); err != nil {
		service.cleanupSession(updateRoot)
		slog.Error("launcher update install failed", "version", update.Version, "error", err)
		service.reportEvent(ctx, telemetry.EventUpdateFailed)
		service.reportError(ctx, telemetry.ErrorUpdateInstallFailed, telemetry.OperationInstallUpdate)
		return &errs.AppError{
			Code:    ErrUpdateInstallerStartFail,
			Message: fmt.Sprintf("Could not start the installer: %v", err),
			Cause:   err,
		}
	}
	publishProgress(publish, Progress{Phase: string(StageRestarting), Progress: 1})
	slog.Info("launcher update applied", "version", update.Version)
	return nil
}

func (service *Service) cleanupSession(sessionDir string) {
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		slog.Warn("could not clean up the update session directory", "dir", sessionDir, "error", err)
		return
	}
	for _, entry := range entries {
		if removeErr := os.Remove(filepath.Join(sessionDir, entry.Name())); removeErr != nil {
			slog.Debug("could not remove a leftover update session file", "dir", sessionDir, "file", entry.Name(), "error", removeErr)
		}
	}
	if removeErr := os.Remove(sessionDir); removeErr != nil {
		slog.Debug("could not remove the update session directory", "dir", sessionDir, "error", removeErr)
	}
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
		return "", errs.NewError(errs.ErrValidation, "Update channel must be stable or prerelease")
	}
}

func publishProgress(publish func(Progress), progress Progress) {
	if publish != nil {
		publish(progress)
	}
}
