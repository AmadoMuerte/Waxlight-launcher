package app

import (
	"context"
	"testing"
	"time"
)

func TestLifecycleShutdownCancelsAndWaitsForWorkers(t *testing.T) {
	lifecycle := NewLifecycle()
	lifecycle.Startup(context.Background())
	started := make(chan struct{})
	finished := make(chan struct{})

	if !lifecycle.Go(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		time.Sleep(10 * time.Millisecond)
		close(finished)
	}) {
		t.Fatal("worker was not registered")
	}
	<-started

	lifecycle.Shutdown()
	select {
	case <-finished:
	default:
		t.Fatal("shutdown returned before the worker finished")
	}
	if lifecycle.Context().Err() != context.Canceled {
		t.Fatalf("context error = %v, want context.Canceled", lifecycle.Context().Err())
	}
	if lifecycle.Go(func(context.Context) {}) {
		t.Fatal("worker registered after shutdown")
	}
}

func TestLifecycleUsesStartupParent(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	lifecycle := NewLifecycle()
	lifecycle.Startup(parent)
	cancel()

	select {
	case <-lifecycle.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("lifecycle context was not canceled with its parent")
	}
	lifecycle.Shutdown()
}

func TestLifecycleRejectsWorkersBeforeStartup(t *testing.T) {
	lifecycle := NewLifecycle()
	if lifecycle.Go(func(context.Context) {}) {
		t.Fatal("worker registered before startup")
	}
}
