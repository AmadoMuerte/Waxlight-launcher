package wails

import (
	"context"
	"reflect"
	"testing"
)

func TestEventAdapterUsesLifecycleContextAndPreservesEvent(t *testing.T) {
	type contextKey string
	parent := context.WithValue(context.Background(), contextKey("key"), "value")
	lifecycle := newTestLifecycle()
	lifecycle.ctx = parent

	var gotContext context.Context
	var gotName string
	var gotPayload []any
	adapter := newEventAdapter(lifecycle, func(ctx context.Context, name string, payload ...any) {
		gotContext = ctx
		gotName = name
		gotPayload = payload
	})
	wantPayload := map[string]int{"progress": 50}
	adapter.Publish("operation:progress", wantPayload)

	if gotContext.Value(contextKey("key")) != "value" {
		t.Fatal("adapter did not use the lifecycle context")
	}
	if gotName != "operation:progress" {
		t.Fatalf("event name = %q", gotName)
	}
	if len(gotPayload) != 1 || !reflect.DeepEqual(gotPayload[0], wantPayload) {
		t.Fatalf("event payload = %#v", gotPayload)
	}
}
