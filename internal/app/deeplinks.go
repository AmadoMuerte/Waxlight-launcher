package app

import (
	"log/slog"
	"sync"

	"github.com/waxlight/waxlight-launcher/internal/events"
	"github.com/waxlight/waxlight-launcher/internal/platform/deeplink"
)

// DeepLinks retains cold-start targets until the React router consumes them.
type DeepLinks struct {
	mu      sync.Mutex
	pending []deeplink.Target
	ready   bool
	events  events.Publisher
}

func NewDeepLinks(eventPublisher events.Publisher) *DeepLinks {
	return &DeepLinks{events: eventPublisher}
}

func (links *DeepLinks) ReceiveArgs(args []string) {
	targets, rejected := deeplink.Extract(args)
	if rejected > 0 {
		slog.Warn("deep link rejected", "count", rejected)
	}
	for _, target := range targets {
		slog.Info("deep link received", "type", target.Type)
		links.dispatch(target)
	}
}

func (links *DeepLinks) dispatch(target deeplink.Target) {
	links.mu.Lock()
	if !links.ready {
		links.pending = append(links.pending, target)
		links.mu.Unlock()
		slog.Info("deep link accepted", "type", target.Type)
		return
	}
	links.mu.Unlock()

	links.events.Publish("deeplink:open", target)
	slog.Info("deep link dispatched", "type", target.Type)
}

// Consume atomically makes event dispatch available and returns cold-start links.
func (links *DeepLinks) Consume() []deeplink.Target {
	links.mu.Lock()
	defer links.mu.Unlock()
	links.ready = true
	pending := links.pending
	links.pending = nil
	if pending == nil {
		pending = []deeplink.Target{}
	}
	if len(pending) > 0 {
		slog.Info("deep link dispatched", "count", len(pending))
	}
	return pending
}
