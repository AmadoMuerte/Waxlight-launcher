package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/apptest"
	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/mods"
	"github.com/waxlight/waxlight-launcher/internal/operations"
)

type namespaceOperationRepository struct{}

func (namespaceOperationRepository) ListOperations(context.Context, int) ([]operations.Operation, error) {
	return nil, nil
}
func (namespaceOperationRepository) SaveOperation(context.Context, operations.Operation) error {
	return nil
}
func (namespaceOperationRepository) ReconcileInterruptedOperations(context.Context, time.Time, string, string) (int64, error) {
	return 0, nil
}
func (namespaceOperationRepository) DeleteFinishedOperation(context.Context, string) error {
	return nil
}
func (namespaceOperationRepository) ClearFinishedOperations(context.Context) (int64, error) {
	return 0, nil
}

func TestPersistentAndModTaskCancellationNamespacesDoNotCross(t *testing.T) {
	lifecycle := apptest.NewLifecycle()
	lifecycle.Startup(context.Background())
	t.Cleanup(lifecycle.Shutdown)
	manager := operations.NewManager(namespaceOperationRepository{}, lifecycle, nil)
	taskManager := mods.NewModTaskManager(nil)
	catalogService := mods.NewCatalogService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		taskManager, time.Now, func() string { return "mod-task-id" },
	)

	operationStarted := make(chan struct{})
	_, err := operations.Start(manager, context.Background(), operations.Operation{ID: "persistent"}, "", func(ctx context.Context) (struct{}, error) {
		close(operationStarted)
		<-ctx.Done()
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	<-operationStarted
	modCtx, _, err := taskManager.Begin(context.Background(), "mod-task", "some-mod", "some-version")
	if err != nil {
		t.Fatal(err)
	}

	if err := catalogService.CancelModTask("persistent"); !isAppErrorCode(err, errs.ErrOperationNotFound) {
		t.Fatalf("mod cancellation accepted persistent operation ID: %v", err)
	}
	if err := manager.Cancel("mod-task"); !isAppErrorCode(err, errs.ErrOperationNotFound) {
		t.Fatalf("operation cancellation accepted ModDB task ID: %v", err)
	}
	select {
	case <-modCtx.Done():
		t.Fatal("operation cancellation cancelled the ModDB task")
	default:
	}

	if err := manager.Cancel("persistent"); err != nil {
		t.Fatal(err)
	}
	if err := catalogService.CancelModTask("mod-task"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(modCtx.Err(), context.Canceled) {
		t.Fatalf("mod task context = %v, want cancellation", modCtx.Err())
	}
}
