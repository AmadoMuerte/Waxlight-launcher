package downloader

import (
	"context"

	"github.com/waxlight/waxlight-launcher/internal/application"
)

// Manager applies one shared concurrency limit to every resource using the
// application Downloader port. Mods can use the same manager when remote mod
// installation is added.
type Manager struct {
	downloader application.Downloader
	slots      chan struct{}
}

func NewManager(
	downloader application.Downloader,
	parallel int,
) *Manager {
	if parallel < 1 {
		parallel = 1
	}
	return &Manager{
		downloader: downloader,
		slots:      make(chan struct{}, parallel),
	}
}

func (manager *Manager) Download(
	ctx context.Context,
	request application.DownloadRequest,
	progress chan<- application.DownloadProgress,
) error {
	select {
	case manager.slots <- struct{}{}:
		defer func() { <-manager.slots }()
	case <-ctx.Done():
		return ctx.Err()
	}
	return manager.downloader.Download(ctx, request, progress)
}

func (manager *Manager) ContentLength(ctx context.Context, url string) (int64, error) {
	return manager.downloader.ContentLength(ctx, url)
}
