package downloader

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

// ctxAwareDownloader honours context cancellation so a paused transfer stops,
// and signals every (re)start through the started channel. It records the
// maximum number of simultaneously active transfers.
type ctxAwareDownloader struct {
	started chan struct{}
	paused  chan struct{}
	release chan struct{}
	active  atomic.Int32
	maximum atomic.Int32
}

func (downloader *ctxAwareDownloader) Download(
	ctx context.Context,
	_ application.DownloadRequest,
	_ chan<- application.DownloadProgress,
) error {
	active := downloader.active.Add(1)
	for {
		maximum := downloader.maximum.Load()
		if active <= maximum || downloader.maximum.CompareAndSwap(maximum, active) {
			break
		}
	}
	downloader.started <- struct{}{}
	select {
	case <-ctx.Done():
		downloader.paused <- struct{}{}
		downloader.active.Add(-1)
		return ctx.Err()
	case <-downloader.release:
		downloader.active.Add(-1)
		return nil
	}
}

func (downloader *ctxAwareDownloader) ContentLength(_ context.Context, _ string) (int64, error) {
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

func TestManagerSetLimitStartsQueuedDownloads(t *testing.T) {
	underlying := &blockingDownloader{
		started: make(chan struct{}, 3),
		release: make(chan struct{}),
	}
	manager := NewManager(underlying, 1)

	var workers sync.WaitGroup
	workers.Add(2)
	for range 2 {
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
	select {
	case <-underlying.started:
		t.Fatal("a second download started while the limit was one")
	default:
	}

	manager.SetLimit(2)
	<-underlying.started

	close(underlying.release)
	workers.Wait()
	if maximum := underlying.maximum.Load(); maximum != 2 {
		t.Fatalf("expected two concurrent downloads after raising the limit, got %d", maximum)
	}
}

func TestManagerSetLimitStartsAllQueuedDownloads(t *testing.T) {
	underlying := &blockingDownloader{
		started: make(chan struct{}, 4),
		release: make(chan struct{}),
	}
	manager := NewManager(underlying, 1)

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
	manager.SetLimit(3)
	<-underlying.started
	<-underlying.started

	close(underlying.release)
	workers.Wait()
	if maximum := underlying.maximum.Load(); maximum != 3 {
		t.Fatalf("expected three concurrent downloads after raising the limit, got %d", maximum)
	}
}

func TestManagerSetLimitPausesRunningDownloads(t *testing.T) {
	underlying := &ctxAwareDownloader{
		started: make(chan struct{}, 4),
		paused:  make(chan struct{}, 2),
		release: make(chan struct{}),
	}
	manager := NewManager(underlying, 2)

	var workers sync.WaitGroup
	workers.Add(2)
	for range 2 {
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

	manager.SetLimit(1)
	<-underlying.paused
	select {
	case <-underlying.started:
		t.Fatal("a paused download restarted while the limit was one")
	default:
	}

	close(underlying.release)
	<-underlying.started
	workers.Wait()
	if maximum := underlying.maximum.Load(); maximum != 2 {
		t.Fatalf("expected at most two concurrent downloads, got %d", maximum)
	}
}

func TestManagerPausedDownloadResumesWhenSlotFrees(t *testing.T) {
	underlying := &ctxAwareDownloader{
		started: make(chan struct{}, 4),
		paused:  make(chan struct{}, 2),
		release: make(chan struct{}),
	}
	manager := NewManager(underlying, 2)

	var workers sync.WaitGroup
	workers.Add(2)
	for range 2 {
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

	manager.SetLimit(1)
	<-underlying.paused

	manager.SetLimit(2)
	<-underlying.started

	close(underlying.release)
	workers.Wait()
	if maximum := underlying.maximum.Load(); maximum != 2 {
		t.Fatalf("expected two concurrent downloads, got %d", maximum)
	}
}

func TestManagerPauseKeepsCallersBlockedUntilResume(t *testing.T) {
	underlying := &ctxAwareDownloader{
		started: make(chan struct{}, 4),
		paused:  make(chan struct{}, 2),
		release: make(chan struct{}),
	}
	manager := NewManager(underlying, 2)

	results := make([]chan error, 2)
	for index := range results {
		results[index] = make(chan error, 1)
		go func(result chan error) {
			result <- manager.Download(
				context.Background(),
				application.DownloadRequest{},
				nil,
			)
		}(results[index])
	}

	<-underlying.started
	<-underlying.started

	manager.SetLimit(1)
	<-underlying.paused

	select {
	case err := <-results[0]:
		t.Fatalf("a paused download returned to its caller: %v", err)
	case err := <-results[1]:
		t.Fatalf("a paused download returned to its caller: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	manager.SetLimit(2)
	<-underlying.started

	close(underlying.release)
	for _, result := range results {
		select {
		case err := <-result:
			if err != nil {
				t.Fatalf("download failed after resuming: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("a download never returned after the slot freed")
		}
	}
}
