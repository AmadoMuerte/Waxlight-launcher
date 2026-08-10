package wails

import (
	"context"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/waxlight/waxlight-launcher/internal/app"
)

type emitFunc func(context.Context, string, ...any)

// EventAdapter publishes semantic application events through Wails.
type EventAdapter struct {
	lifecycle *app.Lifecycle
	emit      emitFunc
}

func NewEventAdapter(lifecycle *app.Lifecycle) *EventAdapter {
	return newEventAdapter(lifecycle, wruntime.EventsEmit)
}

func newEventAdapter(lifecycle *app.Lifecycle, emit emitFunc) *EventAdapter {
	return &EventAdapter{lifecycle: lifecycle, emit: emit}
}

func (adapter *EventAdapter) Publish(name string, payload any) {
	adapter.emit(adapter.lifecycle.Context(), name, payload)
}
