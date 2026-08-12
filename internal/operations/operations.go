// Package operations owns persisted operation tracking and asynchronous workers.
package operations

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/events"
)

const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"

	EventCreated   = "operation:created"
	EventUpdated   = "operation:updated"
	EventProgress  = "operation:progress"
	EventCompleted = "operation:completed"
	EventFailed    = "operation:failed"
	EventRemoved   = "operation:removed"
)

var ErrKeyActive = errors.New("operation key is already active")

type Operation struct {
	ID             string            `json:"id"`
	Type           string            `json:"type"`
	ResourceID     *string           `json:"resourceId,omitempty"`
	Title          string            `json:"title"`
	TitleKey       string            `json:"titleKey,omitempty"`
	TitleParams    map[string]string `json:"titleParams,omitempty"`
	Status         string            `json:"status"`
	Progress       float64           `json:"progress"`
	CurrentBytes   int64             `json:"currentBytes"`
	TotalBytes     int64             `json:"totalBytes"`
	BytesPerSecond int64             `json:"bytesPerSecond"`
	ErrorCode      *string           `json:"errorCode,omitempty"`
	ErrorMessage   *string           `json:"errorMessage,omitempty"`
	CreatedAt      time.Time         `json:"createdAt"`
	StartedAt      *time.Time        `json:"startedAt,omitempty"`
	FinishedAt     *time.Time        `json:"finishedAt,omitempty"`
}

type Repository interface {
	ListOperations(context.Context, int) ([]Operation, error)
	SaveOperation(context.Context, Operation) error
	ReconcileInterruptedOperations(context.Context, time.Time, string, string) (int64, error)
	DeleteFinishedOperation(context.Context, string) error
	ClearFinishedOperations(context.Context) (int64, error)
}

// WorkerOwner starts work derived from the application lifecycle context.
type WorkerOwner interface {
	Go(func(context.Context)) bool
}

type workerEntry struct {
	cancel          context.CancelFunc
	done            <-chan struct{}
	err             func() error
	active          bool
	key             string
	cancelRequested bool
}

type Manager struct {
	repository Repository
	owner      WorkerOwner

	mu        sync.Mutex
	publisher events.Publisher
	workers   map[string]*workerEntry
	active    map[string]string
}

func NewManager(repository Repository, owner WorkerOwner, publisher events.Publisher) *Manager {
	return &Manager{
		repository: repository,
		owner:      owner,
		publisher:  publisher,
		workers:    make(map[string]*workerEntry),
		active:     make(map[string]string),
	}
}

func (manager *Manager) Go(worker func(context.Context)) bool {
	return manager.owner.Go(worker)
}

func (manager *Manager) List(ctx context.Context) ([]Operation, error) {
	return manager.repository.ListOperations(ctx, 100)
}

func (manager *Manager) ListLimit(ctx context.Context, limit int) ([]Operation, error) {
	return manager.repository.ListOperations(ctx, limit)
}

// ReconcileInterrupted marks work which cannot survive a process restart as
// failed before the operation list or relocation checks can observe it.
func (manager *Manager) ReconcileInterrupted(ctx context.Context, now time.Time) (int64, error) {
	return manager.repository.ReconcileInterruptedOperations(
		ctx,
		now.UTC(),
		errs.ErrOperationInterrupted,
		"The operation was interrupted when the launcher closed",
	)
}

func (manager *Manager) Save(ctx context.Context, operation Operation, event string) error {
	err := manager.repository.SaveOperation(ctx, operation)
	if event != "" {
		manager.Publish(event, operation)
		manager.logTransition(event, operation)
	}
	return err
}

func (manager *Manager) SaveBestEffort(operation Operation, event string) {
	if err := manager.Save(context.Background(), operation, event); err != nil {
		slog.Warn("could not persist the operation", "operationId", operation.ID, "error", err)
	}
}

func (manager *Manager) Persist(operation Operation) {
	if err := manager.repository.SaveOperation(context.Background(), operation); err != nil {
		slog.Warn("could not persist the operation", "operationId", operation.ID, "error", err)
	}
}

func (manager *Manager) Publish(event string, payload any) {
	if manager.publisher != nil {
		manager.publisher.Publish(event, payload)
	}
}

func (manager *Manager) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errs.NewError(errs.ErrValidation, "Select an operation to delete")
	}
	if err := manager.repository.DeleteFinishedOperation(ctx, id); err != nil {
		return err
	}
	manager.mu.Lock()
	delete(manager.workers, id)
	manager.mu.Unlock()
	return nil
}

func (manager *Manager) Clear(ctx context.Context) (int64, error) {
	removed, err := manager.repository.ClearFinishedOperations(ctx)
	if err != nil {
		return 0, err
	}
	manager.mu.Lock()
	for id, worker := range manager.workers {
		if !worker.active {
			delete(manager.workers, id)
		}
	}
	manager.mu.Unlock()
	return removed, nil
}

func (manager *Manager) Cancel(id string) error {
	manager.mu.Lock()
	worker := manager.workers[id]
	if worker == nil || !worker.active {
		manager.mu.Unlock()
		return errs.NewError(errs.ErrOperationNotFound, "The operation is no longer running")
	}
	worker.cancelRequested = true
	worker.cancel()
	manager.mu.Unlock()

	<-worker.done
	if err := worker.err(); err != nil && !errors.Is(err, context.Canceled) {
		return &errs.AppError{
			Code:    errs.ErrFilePermission,
			Message: "Could not fully clean up the cancelled operation",
			Cause:   err,
		}
	}
	return nil
}

func (manager *Manager) Wait(ctx context.Context, id string) error {
	manager.mu.Lock()
	worker := manager.workers[id]
	manager.mu.Unlock()
	if worker == nil {
		return errs.NewError(errs.ErrOperationNotFound, "The game version install task is no longer running")
	}
	select {
	case <-worker.done:
		return worker.err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (manager *Manager) logTransition(event string, operation Operation) {
	switch event {
	case EventCreated:
		slog.Info("operation started", "type", operation.Type, "title", operation.Title)
	case EventCompleted:
		slog.Info("operation completed", "type", operation.Type, "title", operation.Title)
	case EventFailed:
		message := ""
		if operation.ErrorMessage != nil {
			message = *operation.ErrorMessage
		}
		slog.Error("operation failed", "type", operation.Type, "title", operation.Title, "error", message)
	}
}

type futureState[T any] struct {
	done   chan struct{}
	result T
	err    error
}

type Future[T any] struct {
	state *futureState[T]
}

func (future Future[T]) Wait(ctx context.Context) (T, error) {
	select {
	case <-future.state.done:
		return future.state.result, future.state.err
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

// Start persists and publishes operation before starting a typed lifecycle worker.
// key serializes related work; an empty key does not reserve a namespace entry.
func Start[T any](
	manager *Manager,
	ctx context.Context,
	operation Operation,
	key string,
	worker func(context.Context) (T, error),
) (Future[T], error) {
	state := &futureState[T]{done: make(chan struct{})}
	future := Future[T]{state: state}

	manager.mu.Lock()
	if key != "" {
		if _, exists := manager.active[key]; exists {
			manager.mu.Unlock()
			return Future[T]{}, ErrKeyActive
		}
		manager.active[key] = operation.ID
	}
	manager.mu.Unlock()

	if err := manager.repository.SaveOperation(ctx, operation); err != nil {
		manager.mu.Lock()
		delete(manager.active, key)
		manager.mu.Unlock()
		return Future[T]{}, err
	}
	manager.Publish(EventCreated, operation)
	manager.logTransition(EventCreated, operation)

	manager.mu.Lock()
	manager.workers[operation.ID] = &workerEntry{
		done:   state.done,
		err:    func() error { return state.err },
		active: true,
		key:    key,
		cancel: func() {},
	}
	manager.mu.Unlock()

	started := manager.owner.Go(func(ownerCtx context.Context) {
		workerCtx, cancel := context.WithCancel(ownerCtx)
		manager.mu.Lock()
		entry := manager.workers[operation.ID]
		if entry == nil {
			if key != "" && manager.active[key] == operation.ID {
				delete(manager.active, key)
			}
			manager.mu.Unlock()
			cancel()
			state.err = context.Canceled
			close(state.done)
			return
		}
		entry.cancel = cancel
		if entry.cancelRequested {
			cancel()
		}
		manager.mu.Unlock()

		state.result, state.err = worker(workerCtx)
		cancel()
		manager.mu.Lock()
		if entry != nil {
			entry.active = false
		}
		if key != "" && manager.active[key] == operation.ID {
			delete(manager.active, key)
		}
		close(state.done)
		manager.mu.Unlock()
	})
	if !started {
		manager.mu.Lock()
		delete(manager.active, key)
		delete(manager.workers, operation.ID)
		manager.mu.Unlock()
		return Future[T]{}, errors.New("application lifecycle is not running")
	}
	return future, nil
}
