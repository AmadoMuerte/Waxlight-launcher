package operations_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/apptest"
	"github.com/waxlight/waxlight-launcher/internal/operations"
)

type memoryRepository struct {
	mu         sync.Mutex
	operations map[string]operations.Operation
	saveErr    error
	onSave     func()
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{operations: make(map[string]operations.Operation)}
}

func (repository *memoryRepository) ListOperations(context.Context, int) ([]operations.Operation, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	result := make([]operations.Operation, 0, len(repository.operations))
	for _, operation := range repository.operations {
		result = append(result, operation)
	}
	return result, nil
}

func (repository *memoryRepository) SaveOperation(_ context.Context, operation operations.Operation) error {
	if repository.onSave != nil {
		repository.onSave()
	}
	if repository.saveErr != nil {
		return repository.saveErr
	}
	repository.mu.Lock()
	repository.operations[operation.ID] = operation
	repository.mu.Unlock()
	return nil
}

func (repository *memoryRepository) ReconcileInterruptedOperations(_ context.Context, finished time.Time, code, message string) (int64, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	var count int64
	for id, operation := range repository.operations {
		if operation.Status != operations.StatusQueued && operation.Status != operations.StatusRunning {
			continue
		}
		operation.Status = operations.StatusFailed
		operation.FinishedAt = &finished
		operation.ErrorCode = &code
		operation.ErrorMessage = &message
		repository.operations[id] = operation
		count++
	}
	return count, nil
}

func (repository *memoryRepository) DeleteFinishedOperation(_ context.Context, id string) error {
	repository.mu.Lock()
	delete(repository.operations, id)
	repository.mu.Unlock()
	return nil
}

func (repository *memoryRepository) ClearFinishedOperations(context.Context) (int64, error) {
	repository.mu.Lock()
	count := int64(len(repository.operations))
	clear(repository.operations)
	repository.mu.Unlock()
	return count, nil
}

func newManager(t *testing.T, publisher func(string, any)) (*operations.Manager, *apptest.Lifecycle) {
	t.Helper()
	lifecycle := apptest.NewLifecycle()
	lifecycle.Startup(context.Background())
	manager := operations.NewManager(newMemoryRepository(), lifecycle, publisherFunc(publisher))
	return manager, lifecycle
}

type publisherFunc func(string, any)

func (publisher publisherFunc) Publish(name string, payload any) {
	if publisher != nil {
		publisher(name, payload)
	}
}

func TestFutureRemainsWaitableAfterFastCompletion(t *testing.T) {
	manager, lifecycle := newManager(t, nil)
	defer lifecycle.Shutdown()

	future, err := operations.Start(manager, context.Background(), operations.Operation{ID: "fast"}, "", func(context.Context) (int, error) {
		return 42, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Wait through the registry first so the typed future is exercised only
	// after the worker has already completed.
	if err := manager.Wait(context.Background(), "fast"); err != nil {
		t.Fatal(err)
	}
	result, err := future.Wait(context.Background())
	if err != nil || result != 42 {
		t.Fatalf("future returned (%d, %v), want (42, nil)", result, err)
	}
}

func TestFuturePropagatesWorkerFailure(t *testing.T) {
	manager, lifecycle := newManager(t, nil)
	defer lifecycle.Shutdown()
	want := errors.New("worker failed")

	future, err := operations.Start(manager, context.Background(), operations.Operation{ID: "failure"}, "", func(context.Context) (struct{}, error) {
		return struct{}{}, want
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := future.Wait(context.Background()); !errors.Is(err, want) {
		t.Fatalf("future error = %v, want %v", err, want)
	}
	if err := manager.Wait(context.Background(), "failure"); !errors.Is(err, want) {
		t.Fatalf("manager wait error = %v, want %v", err, want)
	}
}

func TestLifecycleShutdownCancelsWorker(t *testing.T) {
	manager, lifecycle := newManager(t, nil)
	started := make(chan struct{})
	future, err := operations.Start(manager, context.Background(), operations.Operation{ID: "shutdown"}, "", func(ctx context.Context) (struct{}, error) {
		close(started)
		<-ctx.Done()
		return struct{}{}, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	lifecycle.Shutdown()
	if _, err := future.Wait(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("future error = %v, want context cancellation", err)
	}
}

func TestWorkerCanRemoveItsCancelledOperation(t *testing.T) {
	manager, lifecycle := newManager(t, nil)
	defer lifecycle.Shutdown()
	operation := operations.Operation{ID: "cancelled", Status: operations.StatusRunning}

	future, err := operations.Start(manager, context.Background(), operation, "cancelled-key", func(context.Context) (struct{}, error) {
		operation.Status = operations.StatusCancelled
		if err := manager.Save(context.Background(), operation, ""); err != nil {
			return struct{}{}, err
		}
		if err := manager.Delete(context.Background(), operation.ID); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, context.Canceled
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := future.Wait(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("future error = %v, want context cancellation", err)
	}
	if _, err := manager.List(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestOperationEventNameAndPayloadJSON(t *testing.T) {
	var eventName string
	var payload []byte
	manager, lifecycle := newManager(t, func(name string, value any) {
		eventName = name
		payload, _ = json.Marshal(value)
	})
	defer lifecycle.Shutdown()

	created := time.Date(2026, time.August, 11, 12, 30, 0, 0, time.UTC)
	operation := operations.Operation{ID: "event-id", Type: "snapshot_create", Title: "Creating snapshot", Status: operations.StatusRunning, CreatedAt: created}
	if err := manager.Save(context.Background(), operation, operations.EventCreated); err != nil {
		t.Fatal(err)
	}
	if eventName != "operation:created" {
		t.Fatalf("event name = %q", eventName)
	}
	want := `{"id":"event-id","type":"snapshot_create","title":"Creating snapshot","status":"running","progress":0,"currentBytes":0,"totalBytes":0,"bytesPerSecond":0,"createdAt":"2026-08-11T12:30:00Z"}`
	if string(payload) != want {
		t.Fatalf("payload JSON = %s\nwant         = %s", payload, want)
	}
}

func TestSaveDoesNotPublishOrLogWhenPersistenceFails(t *testing.T) {
	var calls []string
	want := errors.New("database busy")
	repository := newMemoryRepository()
	repository.saveErr = want
	repository.onSave = func() { calls = append(calls, "save") }
	manager := operations.NewManager(repository, nil, publisherFunc(func(string, any) {
		calls = append(calls, "publish")
	}))
	logs := captureLogs(t)

	err := manager.Save(context.Background(), operations.Operation{ID: "failed-save"}, operations.EventCompleted)
	if !errors.Is(err, want) {
		t.Fatalf("Save() error = %v, want %v", err, want)
	}
	if !reflect.DeepEqual(calls, []string{"save"}) {
		t.Fatalf("calls = %v", calls)
	}
	if strings.Contains(logs.String(), "operation completed") {
		t.Fatalf("successful transition was logged: %s", logs.String())
	}
}

func TestSavePersistsBeforePublishingAndLogging(t *testing.T) {
	var calls []string
	repository := newMemoryRepository()
	repository.onSave = func() { calls = append(calls, "save") }
	manager := operations.NewManager(repository, nil, publisherFunc(func(string, any) {
		calls = append(calls, "publish")
	}))
	logs := captureLogs(t)

	if err := manager.Save(context.Background(), operations.Operation{ID: "completed"}, operations.EventCompleted); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"save", "publish"}) {
		t.Fatalf("calls = %v", calls)
	}
	if !strings.Contains(logs.String(), "operation completed") {
		t.Fatalf("transition was not logged: %s", logs.String())
	}
}

func TestSaveWithEmptyEventOnlyPersists(t *testing.T) {
	var calls []string
	repository := newMemoryRepository()
	repository.onSave = func() { calls = append(calls, "save") }
	manager := operations.NewManager(repository, nil, publisherFunc(func(string, any) {
		calls = append(calls, "publish")
	}))
	logs := captureLogs(t)

	if err := manager.Save(context.Background(), operations.Operation{ID: "silent"}, ""); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"save"}) {
		t.Fatalf("calls = %v", calls)
	}
	if logs.Len() != 0 {
		t.Fatalf("unexpected transition log: %s", logs.String())
	}
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &output
}
