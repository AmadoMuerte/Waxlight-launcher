package settings

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/language"
)

// Service is the immutable settings update service.
type Service struct {
	repository Repository
	reader     *Reader
	consent    ConsentSynchronizer
	heartbeat  Heartbeat
	downloads  DownloadLimiter
	mu         sync.Mutex
}

func NewService(
	repository Repository,
	reader *Reader,
	consent ConsentSynchronizer,
	heartbeat Heartbeat,
	downloads DownloadLimiter,
) *Service {
	return &Service{repository: repository, reader: reader, consent: consent, heartbeat: heartbeat, downloads: downloads}
}

func (service *Service) Update(ctx context.Context, value Settings) (Settings, error) {
	service.mu.Lock()
	defer service.mu.Unlock()

	if value.DownloadsParallel < 1 || value.DownloadsParallel > 10 {
		return value, errs.NewError(errs.ErrValidation, "Parallel downloads must be between 1 and 10")
	}
	value.Language = language.NormalizeLanguage(value.Language)
	channel, err := normalizeUpdateChannel(value.UpdateChannel)
	if err != nil {
		return value, err
	}
	value.UpdateChannel = channel
	value.LibrarySort = normalizeLibrarySort(value.LibrarySort)
	value.SkippedUpdateVersion = strings.TrimSpace(value.SkippedUpdateVersion)
	value.OptimumPath = strings.TrimSpace(value.OptimumPath)
	if len(value.SkippedUpdateVersion) > 64 {
		return value, errs.NewError(errs.ErrValidation, "Skipped update version is too long")
	}

	previous, getErr := service.reader.Get(ctx)
	if getErr != nil {
		previous = Settings{}
	}
	save := func() error { return service.repository.SaveSettings(ctx, value) }
	if service.consent != nil {
		err = service.consent.SynchronizeConsent(save)
	} else {
		err = save()
	}
	if err != nil {
		return value, err
	}
	if service.downloads != nil {
		service.downloads.SetLimit(value.DownloadsParallel)
	}
	slog.Info("settings saved", "language", value.Language, "updateChannel", value.UpdateChannel)
	if value.TelemetryEnabled && !previous.TelemetryEnabled && service.heartbeat != nil {
		service.heartbeat.MaybeSendHeartbeat()
	}
	return value, nil
}

func (service *Service) SetLibrarySort(ctx context.Context, librarySort string) (Settings, error) {
	service.mu.Lock()
	defer service.mu.Unlock()

	value, err := service.reader.Get(ctx)
	if err != nil {
		return value, err
	}
	value.LibrarySort = normalizeLibrarySort(librarySort)
	return value, service.repository.SaveSettings(ctx, value)
}

func normalizeLibrarySort(value string) string {
	switch value {
	case LibrarySortName, LibrarySortPlaytime, LibrarySortGameVersion, LibrarySortCreatedAt:
		return value
	default:
		return LibrarySortLastPlayed
	}
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
