package downloader

import (
	"context"
	"errors"
	"sync"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/downloads"
)

// Manager applies one shared concurrency limit to every resource using the
// application Downloader port. The limit can be changed at runtime with
// SetLimit: raising it immediately starts queued downloads, lowering it pauses
// running downloads until a slot frees again. Downloads that resume from a
// .partial file are re-launched, so pausing never loses progress.
type Manager struct {
	downloader downloads.Downloader

	mu           sync.Mutex
	limit        int
	running      int
	wake         chan struct{}
	runningTasks map[*task]context.CancelFunc
}

type task struct {
	request  downloads.Request
	progress chan<- downloads.Progress
	result   chan error
	ctx      context.Context
}

func NewManager(
	downloader downloads.Downloader,
	parallel int,
) *Manager {
	if parallel < 1 {
		parallel = 1
	}
	return &Manager{
		downloader:   downloader,
		limit:        parallel,
		wake:         make(chan struct{}),
		runningTasks: make(map[*task]context.CancelFunc),
	}
}

func (manager *Manager) Download(
	ctx context.Context,
	request downloads.Request,
	progress chan<- downloads.Progress,
) error {
	t := &task{
		request:  request,
		progress: progress,
		result:   make(chan error, 1),
		ctx:      ctx,
	}
	go manager.runDownload(t)
	select {
	case err := <-t.result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (manager *Manager) ContentLength(ctx context.Context, url string) (int64, error) {
	return manager.downloader.ContentLength(ctx, url)
}

// SetLimit updates the shared concurrency limit. Increasing it wakes queued
// downloads; decreasing it cancels the transfer of the excess running
// downloads, which pause and are re-queued until a slot is free again.
func (manager *Manager) SetLimit(parallel int) {
	if parallel < 1 {
		parallel = 1
	}
	manager.mu.Lock()
	manager.limit = parallel
	excess := len(manager.runningTasks) - parallel
	for _, cancel := range manager.runningTasks {
		if excess <= 0 {
			break
		}
		cancel()
		excess--
	}
	manager.notifyLocked()
	manager.mu.Unlock()
}

// runDownload drives one logical download across as many physical transfer
// attempts as the concurrency limit requires, pausing (and later resuming)
// without ever surfacing a pause to the caller.
func (manager *Manager) runDownload(t *task) {
	for {
		runCtx, err := manager.acquireSlot(t)
		if err != nil {
			t.result <- err
			return
		}
		err = manager.downloader.Download(runCtx, t.request, t.progress)

		manager.mu.Lock()
		manager.running--
		delete(manager.runningTasks, t)
		manager.notifyLocked()
		manager.mu.Unlock()

		if err == nil {
			t.result <- nil
			return
		}
		if t.ctx.Err() != nil {
			t.result <- context.Canceled
			return
		}
		if !errors.Is(err, context.Canceled) {
			t.result <- err
			return
		}
	}
}

func (manager *Manager) acquireSlot(t *task) (context.Context, error) {
	for {
		manager.mu.Lock()
		if manager.running < manager.limit {
			manager.running++
			runCtx, cancel := context.WithCancel(t.ctx)
			manager.runningTasks[t] = cancel
			manager.mu.Unlock()
			return runCtx, nil
		}
		wake := manager.wake
		manager.mu.Unlock()

		select {
		case <-wake:
		case <-t.ctx.Done():
			return nil, t.ctx.Err()
		}
	}
}

// notifyLocked broadcasts to every waiter. The wake channel is closed and
// replaced under the lock, so any waiter that observed the old channel is
// released immediately and re-checks the real condition.
func (manager *Manager) notifyLocked() {
	close(manager.wake)
	manager.wake = make(chan struct{})
}
