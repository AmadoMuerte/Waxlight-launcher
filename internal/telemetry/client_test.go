package telemetry

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSendSupportReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/support-reports" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"reportId":"WL-R-A7F31C","status":"received"}`))
	}))
	defer server.Close()
	result, err := NewClient(server.URL).SendSupportReport(context.Background(), map[string]any{"schemaVersion": 1})
	if err != nil || result.ReportID != "WL-R-A7F31C" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestSendSupportReportStatusErrors(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
			defer server.Close()
			if _, err := NewClient(server.URL).SendSupportReport(context.Background(), struct{}{}); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestClientSendsHeartbeatToHeartbeatPath(t *testing.T) {
	server := newCapturingServer(t, http.StatusNoContent)
	defer server.server.Close()

	client := NewClient(server.server.URL)
	err := client.SendHeartbeat(context.Background(), Heartbeat{
		InstallationID: "550e8400-e29b-41d4-a716-446655440000",
		AppVersion:     "0.2.1-beta.5",
		OS:             "linux",
		Arch:           "amd64",
		InstancesCount: 4,
		ModsCount:      73,
	})
	if err != nil {
		t.Fatalf("SendHeartbeat returned error: %v", err)
	}
	server.requirePath(t, "/v1/heartbeat")
	var payload map[string]any
	server.decodeBody(t, &payload)
	if payload["installation_id"] != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("installation_id mismatch: %v", payload)
	}
	if payload["app_version"] != "0.2.1-beta.5" {
		t.Fatalf("prerelease app_version was not preserved: %v", payload)
	}
	if payload["os"] != "linux" || payload["arch"] != "amd64" {
		t.Fatalf("os/arch mismatch: %v", payload)
	}
	if payload["instances_count"] != float64(4) || payload["mods_count"] != float64(73) {
		t.Fatalf("counts mismatch: %v", payload)
	}
}

func TestClientSendsEventToEventsPath(t *testing.T) {
	server := newCapturingServer(t, http.StatusNoContent)
	defer server.server.Close()

	client := NewClient(server.server.URL)
	err := client.SendEvent(context.Background(), Event{
		InstallationID: "550e8400-e29b-41d4-a716-446655440000",
		AppVersion:     "0.2.2",
		OS:             "windows",
		Arch:           "arm64",
		Event:          "instance_created",
	})
	if err != nil {
		t.Fatalf("SendEvent returned error: %v", err)
	}
	server.requirePath(t, "/v1/events")
	var payload map[string]any
	server.decodeBody(t, &payload)
	if payload["event"] != "instance_created" {
		t.Fatalf("event mismatch: %v", payload)
	}
}

func TestClientSendsErrorToErrorsPath(t *testing.T) {
	server := newCapturingServer(t, http.StatusNoContent)
	defer server.server.Close()

	client := NewClient(server.server.URL)
	err := client.SendError(context.Background(), ErrorEvent{
		InstallationID: "550e8400-e29b-41d4-a716-446655440000",
		AppVersion:     "0.2.2",
		OS:             "linux",
		Arch:           "amd64",
		ErrorCode:      "MOD_DOWNLOAD_HTTP_404",
		Component:      "mod_downloader",
		Operation:      "download_mod",
	})
	if err != nil {
		t.Fatalf("SendError returned error: %v", err)
	}
	server.requirePath(t, "/v1/errors")
	var payload map[string]any
	server.decodeBody(t, &payload)
	if payload["error_code"] != "MOD_DOWNLOAD_HTTP_404" ||
		payload["component"] != "mod_downloader" ||
		payload["operation"] != "download_mod" {
		t.Fatalf("error payload mismatch: %v", payload)
	}
}

func TestClientTreatsOnly204AsSuccess(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusCreated, http.StatusAccepted} {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(status)
		}))
		client := NewClient(server.URL)
		err := client.SendHeartbeat(context.Background(), Heartbeat{})
		server.Close()
		if err == nil {
			t.Fatalf("status %d was not treated as a delivery failure", status)
		}
	}
}

func TestClientDropsInvalidPayloadStatuses(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusRequestEntityTooLarge} {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(status)
		}))
		client := NewClient(server.URL)
		err := client.SendEvent(context.Background(), Event{})
		server.Close()
		if err != nil {
			t.Fatalf("status %d should drop the payload without an error: %v", status, err)
		}
	}
}

func TestClientReportsTransientFailures(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable} {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(status)
		}))
		client := NewClient(server.URL)
		err := client.SendHeartbeat(context.Background(), Heartbeat{})
		server.Close()
		if err == nil {
			t.Fatalf("status %d was not treated as a transient failure", status)
		}
	}
}

func TestClientTimeoutsSlowServer(t *testing.T) {
	// The client-side timeout must fire even though the server never responds.
	// The handler only returns after the test asserts the timeout, so server
	// shutdown stays deterministic regardless of keep-alive behavior.
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		<-release
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	started := time.Now()
	err := client.SendHeartbeat(context.Background(), Heartbeat{})
	elapsed := time.Since(started)
	if err == nil {
		t.Fatal("slow server did not time out")
	}
	if elapsed > 6*time.Second {
		t.Fatalf("timeout took too long: %v", elapsed)
	}
	close(release)
}

func TestClientFailsGracefullyWhenUnreachable(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	err := client.SendHeartbeat(context.Background(), Heartbeat{})
	if err == nil {
		t.Fatal("unreachable server did not produce an error")
	}
}

func TestClientIgnoresMalformedServerResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
		_, _ = writer.Write([]byte("not json at all"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if err := client.SendHeartbeat(context.Background(), Heartbeat{}); err != nil {
		t.Fatalf("malformed success body caused a failure: %v", err)
	}
}

type capturingServer struct {
	t      *testing.T
	server *httptest.Server
	mu     sync.Mutex
	path   string
	body   string
}

func newCapturingServer(t *testing.T, status int) *capturingServer {
	t.Helper()
	capture := &capturingServer{t: t}
	capture.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		raw, _ := io.ReadAll(request.Body)
		capture.mu.Lock()
		capture.path = request.URL.Path
		capture.body = string(raw)
		capture.mu.Unlock()
		writer.WriteHeader(status)
	}))
	return capture
}

func (c *capturingServer) requirePath(t *testing.T, expected string) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.path != expected {
		t.Fatalf("request went to %q, want %q", c.path, expected)
	}
}

func (c *capturingServer) decodeBody(t *testing.T, target any) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if strings.TrimSpace(c.body) == "" {
		t.Fatal("no request body captured")
	}
	if err := json.Unmarshal([]byte(c.body), target); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
}
