// Package apptest provides a self-contained application lifecycle for tests
// that must stay independent of the composition root. internal/app/wire.go
// binds the controllers and features, so importing internal/app from tests of
// packages it imports would create a test-only import cycle.
package apptest

import (
	"context"
	"sync"
)

// Lifecycle mirrors the worker semantics of app.Lifecycle: startup derives the
// application context, Go registers lifecycle-owned workers, and Shutdown
// cancels the context and joins every registered worker.
type Lifecycle struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	stopping bool
	workers  sync.WaitGroup
}

func NewLifecycle() *Lifecycle {
	ctx, cancel := context.WithCancel(context.Background())
	return &Lifecycle{ctx: ctx, cancel: cancel}
}

// Startup derives the application context from the given parent, mirroring
// app.Lifecycle.Startup.
func (lifecycle *Lifecycle) Startup(parent context.Context) {
	if parent == nil {
		return
	}
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if lifecycle.stopping {
		return
	}
	lifecycle.ctx, lifecycle.cancel = context.WithCancel(parent)
}

func (lifecycle *Lifecycle) Context() context.Context {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	return lifecycle.ctx
}

func (lifecycle *Lifecycle) Go(worker func(context.Context)) bool {
	if worker == nil {
		return false
	}
	lifecycle.mu.Lock()
	if lifecycle.stopping {
		lifecycle.mu.Unlock()
		return false
	}
	ctx := lifecycle.ctx
	lifecycle.workers.Add(1)
	lifecycle.mu.Unlock()

	go func() {
		defer lifecycle.workers.Done()
		worker(ctx)
	}()
	return true
}

func (lifecycle *Lifecycle) Shutdown() {
	lifecycle.mu.Lock()
	lifecycle.stopping = true
	lifecycle.cancel()
	lifecycle.mu.Unlock()

	lifecycle.workers.Wait()
}
