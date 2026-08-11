package wails

import "context"

// testLifecycle is a minimal lifecycle stub for transport tests. It keeps the
// transport package free of the composition-root import, which would create a
// test-only import cycle because internal/app/wire.go binds these controllers.
type testLifecycle struct {
	ctx context.Context
}

func newTestLifecycle() *testLifecycle {
	return &testLifecycle{ctx: context.Background()}
}

func (stub *testLifecycle) Context() context.Context {
	return stub.ctx
}

func (stub *testLifecycle) Go(func(context.Context)) bool {
	return true
}
