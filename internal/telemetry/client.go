package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
)

// DefaultEndpoint is the production Waxlight telemetry backend. It is the
// single place the endpoint is defined for release builds.
const DefaultEndpoint = "https://waxlight.telemetry.amadomuerte.ru"

// requestTimeout bounds every telemetry request. Telemetry must never be able
// to block Waxlight for a long time.
const requestTimeout = 4 * time.Second

const (
	pathHeartbeat      = "/v1/heartbeat"
	pathEvents         = "/v1/events"
	pathErrors         = "/v1/errors"
	pathSupportReports = "/api/v1/support-reports"
)

// Client is the reusable, bounded-timeout telemetry HTTP client. One client is
// shared by all telemetry sends; it performs no retries.
type Client struct {
	endpoint string
	http     *http.Client
}

// NewClient returns a Client posting to endpoint. Tests inject their own
// endpoint; release builds use DefaultEndpoint.
func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		http:     &http.Client{Timeout: requestTimeout},
	}
}

// ProductionEndpoint returns the sole telemetry destination used by release
// builds. Keeping it fixed makes the privacy policy and runtime behavior match.
func ProductionEndpoint() string {
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

func (c *Client) SendSupportReport(ctx context.Context, payload any) (SupportReportResult, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return SupportReportResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+pathSupportReports, bytes.NewReader(encoded))
	if err != nil {
		return SupportReportResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return SupportReportResult{}, &errs.AppError{Code: errs.ErrSupportReportFailed, Message: "Could not reach the support service", Retryable: true, Cause: err}
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		var result SupportReportResult
		if err := json.NewDecoder(io.LimitReader(response.Body, 16*1024)).Decode(&result); err != nil || !supportReportID.MatchString(result.ReportID) || result.Status != "received" {
			return SupportReportResult{}, &errs.AppError{Code: errs.ErrSupportReportFailed, Message: "The support service returned an invalid response", Cause: err}
		}
		return result, nil
	case http.StatusBadRequest:
		return SupportReportResult{}, errs.NewError(errs.ErrValidation, "The support service rejected the report")
	case http.StatusUnsupportedMediaType:
		return SupportReportResult{}, errs.NewError(errs.ErrSupportReportFailed, "The support service rejected the report format")
	case http.StatusUnprocessableEntity:
		return SupportReportResult{}, errs.NewError(errs.ErrSupportReportFailed, "The support service does not support this report schema")
	case http.StatusRequestEntityTooLarge:
		return SupportReportResult{}, errs.NewError(errs.ErrSupportReportTooLarge, "The support report is too large")
	case http.StatusTooManyRequests:
		return SupportReportResult{}, &errs.AppError{Code: errs.ErrSupportReportRateLimited, Message: "Too many reports were sent; try again later", Retryable: true}
	default:
		return SupportReportResult{}, &errs.AppError{Code: errs.ErrSupportReportFailed, Message: fmt.Sprintf("Support service unavailable (%d)", response.StatusCode), Retryable: response.StatusCode >= 500}
	}
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

var supportReportID = regexp.MustCompile(`^WL-R-[A-F0-9]{6}$`)

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
