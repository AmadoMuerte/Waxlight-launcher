package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/mods"
)

// sentinel must never appear in any telemetry payload. The test store injects
// it into every possible user-controlled field (instance names, mod names,
// paths, account data) so an end-to-end leak would be detected.
const sentinel = "SECRET_PATH_OR_TOKEN_SHOULD_NEVER_BE_SENT"

// TestSentinelNeverEntersTelemetryBodies drives the real HTTP client with
// hostile store contents and asserts the sentinel cannot appear in any
// request body sent to any endpoint.
func TestSentinelNeverEntersTelemetryBodies(t *testing.T) {
	store := newFakeStore(t, map[string]string{})
	store.setEnabled(true)
	store.instances = []instances.Instance{
		{ID: sentinel, Name: sentinel},
	}
	store.mods[sentinel] = []mods.InstalledMod{
		{ID: sentinel, Name: sentinel, FilePath: sentinel, FileName: sentinel},
	}

	capture := &bodyCapture{responses: map[string]int{}}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		capture.capture(request)
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	service := NewService(client, store, store, store)

	service.sendHeartbeat(context.Background())
	service.sendEvent(context.Background(), EventInstanceCreated)
	service.sendError(context.Background(), ErrorGameLaunchFailed, ComponentGameLauncher, OperationLaunchGame)

	capture.assertBodies(t, 3)
	for path, body := range capture.bodies {
		if strings.Contains(body, sentinel) {
			t.Fatalf("telemetry payload to %s leaked sensitive data: %s", path, body)
		}
	}
}

// TestForbiddenKeysNeverAppearInTelemetryJSON is a schema-level privacy
// regression: the serialized payloads must contain none of the keys that
// could carry prohibited data.
func TestForbiddenKeysNeverAppearInTelemetryJSON(t *testing.T) {
	for name, encoded := range map[string]string{
		"heartbeat": marshalJSON(t, Heartbeat{InstallationID: "id", AppVersion: "1.2.3"}),
		"event":     marshalJSON(t, Event{InstallationID: "id", AppVersion: "1.2.3", Event: "instance_created"}),
		"error":     marshalJSON(t, ErrorEvent{InstallationID: "id", AppVersion: "1.2.3"}),
	} {
		for _, forbidden := range []string{
			"password",
			"session_key",
			"session_key",
			"session_signature",
			"mp_token",
			"pre_login_token",
			"player_name",
			"username",
			"email",
			"uid",
			"account",
			"path",
			"filename",
			"file_name",
			"stacktrace",
			"message",
			"error_text",
			"hostname",
			"machine_id",
			"mac",
			"token",
			"cookie",
			"authorization",
		} {
			if strings.Contains(strings.ToLower(encoded), forbidden) {
				t.Fatalf("%s payload contains forbidden key %q: %s", name, forbidden, encoded)
			}
		}
	}
}

// TestPayloadKeysAreExplicit verifies the exact JSON key allowlist per
// endpoint, so accidental new fields are caught in review.
func TestPayloadKeysAreExplicit(t *testing.T) {
	heartbeat := map[string]any{}
	event := map[string]any{}
	errorEvent := map[string]any{}
	mustUnmarshal(t, marshalJSON(t, Heartbeat{InstallationID: "a", AppVersion: "b", OS: "c", Arch: "d", InstancesCount: 1, ModsCount: 2}), &heartbeat)
	mustUnmarshal(t, marshalJSON(t, Event{InstallationID: "a", AppVersion: "b", OS: "c", Arch: "d", Event: "e"}), &event)
	mustUnmarshal(t, marshalJSON(t, ErrorEvent{InstallationID: "a", AppVersion: "b", OS: "c", Arch: "d", ErrorCode: "e", Component: "f", Operation: "g"}), &errorEvent)

	expectedHeartbeat := []string{"installation_id", "app_version", "os", "arch", "instances_count", "mods_count"}
	requireKeys(t, heartbeat, expectedHeartbeat)
	requireKeys(t, event, []string{"installation_id", "app_version", "os", "arch", "event"})
	requireKeys(t, errorEvent, []string{"installation_id", "app_version", "os", "arch", "error_code", "component", "operation"})
}

type bodyCapture struct {
	responses map[string]int
	bodies    map[string]string
}

func (c *bodyCapture) capture(request *http.Request) {
	length := request.ContentLength
	raw := make([]byte, length)
	_, _ = request.Body.Read(raw)
	if c.bodies == nil {
		c.bodies = map[string]string{}
	}
	c.bodies[request.URL.Path] = string(raw)
	c.responses[request.URL.Path]++
}

func (c *bodyCapture) assertBodies(t *testing.T, expectedCount int) {
	t.Helper()
	if len(c.bodies) != expectedCount {
		t.Fatalf("captured %d request bodies, want %d: %v", len(c.bodies), expectedCount, c.bodies)
	}
}

func mustUnmarshal(t *testing.T, raw string, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func requireKeys(t *testing.T, payload map[string]any, expected []string) {
	t.Helper()
	if len(payload) != len(expected) {
		t.Fatalf("payload has %d keys, want %v: %v", len(payload), expected, payload)
	}
	for _, key := range expected {
		if _, ok := payload[key]; !ok {
			t.Fatalf("payload missing key %q: %v", key, payload)
		}
	}
}
