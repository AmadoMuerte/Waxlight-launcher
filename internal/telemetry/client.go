package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// DefaultEndpoint is the production Waxlight telemetry backend. It is the
// single place the endpoint is defined for release builds.
const DefaultEndpoint = "https://waxlight.telemetry.amadomuerte.ru"

// endpointEnvironmentVariable overrides DefaultEndpoint for local development
// and test harnesses. It never changes the persisted user configuration.
const endpointEnvironmentVariable = "WAXLIGHT_TELEMETRY_ENDPOINT"

// requestTimeout bounds every telemetry request. Telemetry must never be able
// to block Waxlight for a long time.
const requestTimeout = 4 * time.Second

const (
	pathHeartbeat = "/v1/heartbeat"
	pathEvents    = "/v1/events"
	pathErrors    = "/v1/errors"
)

// Client is the reusable, bounded-timeout telemetry HTTP client. One client is
// shared by all telemetry sends; it performs no retries.
type Client struct {
	endpoint string
	http     *http.Client
}

// NewClient returns a Client posting to endpoint. Tests inject their own
// endpoint; release builds use DefaultEndpoint (or the development override).
func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		http:     &http.Client{Timeout: requestTimeout},
	}
}

// ProductionEndpoint resolves the endpoint for release builds, honoring the
// development-only environment override.
func ProductionEndpoint() string {
	if override := os.Getenv(endpointEnvironmentVariable); override != "" {
		return override
	}
	return DefaultEndpoint
}

func (c *Client) SendHeartbeat(ctx context.Context, payload Heartbeat) error {
	return c.post(ctx, pathHeartbeat, payload)
}

func (c *Client) SendEvent(ctx context.Context, payload Event) error {
	return c.post(ctx, pathEvents, payload)
}

func (c *Client) SendError(ctx context.Context, payload ErrorEvent) error {
	return c.post(ctx, pathErrors, payload)
}

func (c *Client) post(ctx context.Context, path string, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+path, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// The backend confirms ingestion with 204 No Content. Any other status is
	// a delivery failure; no body content is read or logged.
	switch response.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge:
		// Invalid payloads are a client bug; the payload is dropped.
		slog.Debug("telemetry: payload rejected", "path", path, "status", response.StatusCode)
		return nil
	case http.StatusTooManyRequests:
		slog.Debug("telemetry: rate limited; payload dropped", "path", path)
		return errTransient
	case http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		slog.Debug("telemetry: server unavailable; payload dropped", "path", path, "status", response.StatusCode)
		return errTransient
	default:
		slog.Debug("telemetry: unexpected response; payload dropped", "path", path, "status", response.StatusCode)
		return errTransient
	}
}

type transientError string

func (e transientError) Error() string { return string(e) }

var errTransient error = transientError("transient telemetry delivery failure")

// enabledOS maps runtime.GOOS to the normalized values expected by the
// telemetry backend. Unknown platforms are reported as "other" instead of
// forwarding arbitrary GOOS values.
func normalizeOS(goos string) string {
	switch goos {
	case "linux", "windows":
		return goos
	default:
		return "other"
	}
}

// normalizeArch returns the architecture value sent to the telemetry backend.
// Go runtime architectures pass through as-is; the backend's validation
// handles amd64 and arm64, which cover the supported platforms.
func normalizeArch(goarch string) string {
	return goarch
}
