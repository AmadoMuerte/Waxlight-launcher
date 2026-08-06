package logging

import (
	"context"
	"log/slog"
	"strings"
	"sync"
)

// handler is a slog.Handler that feeds every record into the shared in-memory
// buffer, optionally notifies an external emitter, and mirrors the line to
// stderr. Attribute and group support is kept deliberately simple: attributes
// are appended to the message so nothing is silently dropped.
type handler struct {
	mu    sync.Mutex
	attrs []slog.Attr
}

var _ slog.Handler = (*handler)(nil)

func (h *handler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *handler) Handle(_ context.Context, record slog.Record) error {
	message := record.Message
	h.mu.Lock()
	if len(h.attrs) > 0 {
		var builder strings.Builder
		builder.WriteString(message)
		builder.WriteString(" (")
		for index, attr := range h.attrs {
			if index > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(attr.Key)
			builder.WriteString("=")
			builder.WriteString(attr.Value.String())
		}
		builder.WriteString(")")
		message = builder.String()
	}
	h.mu.Unlock()

	var attrs []slog.Attr
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})
	if len(attrs) > 0 {
		var builder strings.Builder
		builder.WriteString(message)
		for _, attr := range attrs {
			builder.WriteString(" ")
			builder.WriteString(attr.Key)
			builder.WriteString("=")
			builder.WriteString(attr.Value.String())
		}
		message = builder.String()
	}

	entry := Entry{
		Time:    record.Time,
		Level:   levelString(record.Level),
		Message: Redact(message),
	}
	appendEntry(entry)
	return nil
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.mu.Lock()
	defer h.mu.Unlock()
	merged := append([]slog.Attr(nil), h.attrs...)
	merged = append(merged, attrs...)
	return &handler{attrs: merged}
}

func (h *handler) WithGroup(string) slog.Handler {
	return h
}

func levelString(level slog.Level) Level {
	switch {
	case level <= slog.LevelDebug:
		return LevelDebug
	case level <= slog.LevelInfo:
		return LevelInfo
	case level <= slog.LevelWarn:
		return LevelWarn
	default:
		return LevelError
	}
}
