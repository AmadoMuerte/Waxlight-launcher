package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

const maximumLauncherUpdateSize = 512 * 1024 * 1024

type LauncherUpdateService struct {
	source         LauncherUpdateSource
	downloader     Downloader
	installer      LauncherUpdateInstaller
	dataRoot       string
	currentVersion string
	mu             sync.Mutex
	installing     bool
}

func NewLauncherUpdateService(
	source LauncherUpdateSource,
	downloader Downloader,
	installer LauncherUpdateInstaller,
	dataRoot string,
	currentVersion string,
) *LauncherUpdateService {
	return &LauncherUpdateService{
		source:         source,
		downloader:     downloader,
		installer:      installer,
		dataRoot:       dataRoot,
		currentVersion: currentVersion,
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
	update, err := service.source.Check(ctx, service.currentVersion, channel)
	if err != nil {
		return domain.LauncherUpdate{}, &domain.AppError{
			Code:      domain.ErrUpdateUnavailable,
			Message:   "Could not check for launcher updates",
			Retryable: true,
			Cause:     err,
		}
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

	update, err := service.Check(ctx, channel)
	if err != nil {
		return err
	}
	if !update.Available {
		return domain.NewError(domain.ErrUpdateUnavailable, "No launcher update is available")
	}
	if update.AssetName == "" || filepath.Base(update.AssetName) != update.AssetName {
		return domain.NewError(domain.ErrUpdateFailed, "The release contains an unsafe update filename")
	}
	if len(update.SHA256) != 64 {
		return domain.NewError(domain.ErrUpdateFailed, "The release checksum is invalid")
	}

	updateRoot := filepath.Join(service.dataRoot, "updates")
	if err := os.MkdirAll(updateRoot, 0o700); err != nil {
		return fmt.Errorf("create update directory: %w", err)
	}
	destination := filepath.Join(updateRoot, update.AssetName)
	if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove previous staged update: %w", err)
	}
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
		return &domain.AppError{
			Code:      domain.ErrUpdateFailed,
			Message:   "Could not download or verify the launcher update",
			Retryable: true,
			Cause:     err,
		}
	}

	publishProgress(publish, domain.LauncherUpdateProgress{Phase: "installing", Progress: 1})
	if err := service.installer.Apply(ctx, destination); err != nil {
		return &domain.AppError{
			Code:    domain.ErrUpdateFailed,
			Message: "Could not install the launcher update; the current installation was preserved",
			Cause:   err,
		}
	}
	publishProgress(publish, domain.LauncherUpdateProgress{Phase: "restarting", Progress: 1})
	return nil
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
