package errs

import (
	"errors"
	"log/slog"

	"github.com/AmadoMuerte/vintagestory-go/vshttp"
)

// LogFailure logs an operational failure at warn level. When the error chain
// contains a *vshttp.APIError, its structured diagnostics (kind, endpoint,
// status, retryable, retry_after) are attached as attributes so log files
// and support reports keep the real cause of an upstream failure. The full
// error chain is always included as the "error" attribute; the logging
// pipeline redacts sensitive values.
func LogFailure(message string, err error) {
	slog.Warn(message, FailureAttrs(err)...)
}

// FailureAttrs renders the log attributes for an operational failure.
func FailureAttrs(err error) []any {
	attrs := []any{"error", err}
	var apiErr *vshttp.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Kind != "" {
			attrs = append(attrs, "kind", string(apiErr.Kind))
		}
		if apiErr.Endpoint != "" {
			attrs = append(attrs, "endpoint", apiErr.Endpoint)
		}
		if apiErr.StatusCode != 0 {
			attrs = append(attrs, "status", apiErr.StatusCode)
		}
		attrs = append(attrs, "retryable", apiErr.Retryable)
		if apiErr.RetryAfter > 0 {
			attrs = append(attrs, "retry_after", apiErr.RetryAfter.String())
		}
	}
	return attrs
}
