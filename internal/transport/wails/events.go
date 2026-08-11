package wails

import (
	"context"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type emitFunc func(context.Context, string, ...any)

// EventAdapter publishes semantic application events through Wails.
type EventAdapter struct {
	lifecycle lifecycle
	emit      emitFunc
}

func NewEventAdapter(lifecycle lifecycle) *EventAdapter {
	return newEventAdapter(lifecycle, wruntime.EventsEmit)
}

func newEventAdapter(lifecycle lifecycle, emit emitFunc) *EventAdapter {
	return &EventAdapter{lifecycle: lifecycle, emit: emit}
}

func (adapter *EventAdapter) Publish(name string, payload any) {
	adapter.emit(adapter.lifecycle.Context(), name, payload)
}
