package app

import (
	"context"
	"sync"
)

// Lifecycle owns the application context and workers derived from it.
type Lifecycle struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	stopping bool
	workers  sync.WaitGroup
}

func NewLifecycle() *Lifecycle {
	return &Lifecycle{ctx: context.Background()}
}

// Startup derives the application context from the framework context.
func (lifecycle *Lifecycle) Startup(parent context.Context) {
	if parent == nil {
		parent = context.Background()
	}

	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if lifecycle.cancel != nil || lifecycle.stopping {
		return
	}
	lifecycle.ctx, lifecycle.cancel = context.WithCancel(parent)
}

// Context returns the context shared by application work.
func (lifecycle *Lifecycle) Context() context.Context {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	return lifecycle.ctx
}

// Go registers and starts work that must finish before Shutdown returns.
// It returns false once shutdown has begun.
func (lifecycle *Lifecycle) Go(worker func(context.Context)) bool {
	if worker == nil {
		return false
	}

	lifecycle.mu.Lock()
	if lifecycle.cancel == nil || lifecycle.stopping {
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

// Shutdown cancels the application context and waits for every registered
// worker. Concurrent calls are safe and wait for the same worker set.
func (lifecycle *Lifecycle) Shutdown() {
	lifecycle.mu.Lock()
	lifecycle.stopping = true
	if lifecycle.cancel != nil {
		lifecycle.cancel()
	}
	lifecycle.mu.Unlock()

	lifecycle.workers.Wait()
}
