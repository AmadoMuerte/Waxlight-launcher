package downloader

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/application"
)

type blockingDownloader struct {
	started chan struct{}
	release chan struct{}
	active  atomic.Int32
	maximum atomic.Int32
}

func (downloader *blockingDownloader) Download(
	context.Context,
	application.DownloadRequest,
	chan<- application.DownloadProgress,
) error {
	active := downloader.active.Add(1)
	for {
		maximum := downloader.maximum.Load()
		if active <= maximum || downloader.maximum.CompareAndSwap(maximum, active) {
			break
		}
	}
	downloader.started <- struct{}{}
	<-downloader.release
	downloader.active.Add(-1)
	return nil
}

func (downloader *blockingDownloader) ContentLength(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func TestManagerLimitsConcurrentDownloads(t *testing.T) {
	underlying := &blockingDownloader{
		started: make(chan struct{}, 3),
		release: make(chan struct{}),
	}
	manager := NewManager(underlying, 2)

	var workers sync.WaitGroup
	workers.Add(3)
	for range 3 {
		go func() {
			defer workers.Done()
			_ = manager.Download(
				context.Background(),
				application.DownloadRequest{},
				nil,
			)
		}()
	}

	<-underlying.started
	<-underlying.started
	select {
	case <-underlying.started:
		t.Fatal("a third download started before a slot was released")
	default:
	}

	close(underlying.release)
	workers.Wait()
	if maximum := underlying.maximum.Load(); maximum != 2 {
		t.Fatalf("expected at most two concurrent downloads, got %d", maximum)
	}
}
