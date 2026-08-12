package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/mods"
	"github.com/waxlight/waxlight-launcher/internal/settings"
)

// testWorkers runs telemetry delivery in plain background goroutines without a
// lifecycle, preserving the asynchronous delivery semantics the service tests
// rely on.
type testWorkers struct{}

func (testWorkers) Go(worker func(context.Context)) bool {
	go worker(context.Background())
	return true
}

// fakeStore implements Store with in-memory state.
type fakeStore struct {
	mu        sync.Mutex
	enabled   bool
	values    map[string]string
	instances []instances.Instance
	mods      map[string][]mods.InstalledMod
	err       error
}

func newFakeStore(t *testing.T, values map[string]string) *fakeStore {
	t.Helper()
	return &fakeStore{values: values, mods: map[string][]mods.InstalledMod{}}
}

func (s *fakeStore) Get(context.Context) (settings.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return settings.Settings{}, s.err
	}
	return settings.Settings{TelemetryEnabled: s.enabled}, nil
}

func (s *fakeStore) GetSettingValue(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return "", s.err
	}
	return s.values[key], nil
}

func (s *fakeStore) SetSettingValue(_ context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.values[key] = value
	return nil
}

func (s *fakeStore) ListInstances(context.Context) ([]instances.Instance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	return append([]instances.Instance(nil), s.instances...), nil
}

func (s *fakeStore) ListMods(_ context.Context, instanceID string) ([]mods.InstalledMod, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	return append([]mods.InstalledMod(nil), s.mods[instanceID]...), nil
}

func (s *fakeStore) setEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
}

func (s *fakeStore) setErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

// fakeSender records deliveries and can be configured to fail.
type fakeSender struct {
	mu             sync.Mutex
	heartbeats     []Heartbeat
	events         []Event
	errors         []ErrorEvent
	heartbeatError error
}

type blockingHeartbeatStore struct {
	*fakeStore
	listed  chan struct{}
	release chan struct{}
}

func (s *blockingHeartbeatStore) ListInstances(ctx context.Context) ([]instances.Instance, error) {
	close(s.listed)
	<-s.release
	return s.fakeStore.ListInstances(ctx)
}

func (s *fakeSender) SendHeartbeat(_ context.Context, payload Heartbeat) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.heartbeats = append(s.heartbeats, payload)
	return s.heartbeatError
}

func (s *fakeSender) SendEvent(_ context.Context, payload Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, payload)
	return nil
}

func (s *fakeSender) SendError(_ context.Context, payload ErrorEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors = append(s.errors, payload)
	return nil
}

func (s *fakeSender) heartbeatCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.heartbeats)
}

func (s *fakeSender) eventCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

func (s *fakeSender) errorCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.errors)
}

func (s *fakeSender) lastHeartbeat() (Heartbeat, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.heartbeats) == 0 {
		return Heartbeat{}, false
	}
	return s.heartbeats[len(s.heartbeats)-1], true
}

// waitFor polls condition until it holds or the timeout expires. Telemetry
// delivery is asynchronous by design, so tests must tolerate the goroutine
// scheduling delay.
func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition did not hold within the timeout")
}

func TestDisabledTelemetrySendsNothing(t *testing.T) {
	store := newFakeStore(t, map[string]string{})
	store.setEnabled(false)
	sender := &fakeSender{}
	service := NewService(sender, store, store, store, testWorkers{})

	service.Event(context.Background(), EventInstanceCreated)
	service.Error(context.Background(), ErrorGameLaunchFailed, ComponentGameLauncher, OperationLaunchGame)
	service.MaybeSendHeartbeat()

	time.Sleep(50 * time.Millisecond)
	if sender.heartbeatCount() != 0 || sender.eventCount() != 0 || sender.errorCount() != 0 {
		t.Fatal("disabled telemetry transmitted data")
	}
}

func TestFirstEligibleHeartbeatIsSent(t *testing.T) {
	store := newFakeStore(t, map[string]string{})
	store.setEnabled(true)
	store.instances = []instances.Instance{{ID: "a"}, {ID: "b"}}
	store.mods["a"] = []mods.InstalledMod{{ID: "m1"}, {ID: "m2"}}
	sender := &fakeSender{}
	service := NewService(sender, store, store, store, testWorkers{})

	service.MaybeSendHeartbeat()
	waitFor(t, func() bool { return sender.heartbeatCount() == 1 })

	heartbeat, ok := sender.lastHeartbeat()
	if !ok {
		t.Fatal("no heartbeat was sent")
	}
	if heartbeat.InstancesCount != 2 {
		t.Fatalf("instances_count = %d, want 2", heartbeat.InstancesCount)
	}
	if heartbeat.ModsCount != 2 {
		t.Fatalf("mods_count = %d, want 2", heartbeat.ModsCount)
	}
}

func TestSecondHeartbeatWithin24hIsNotSent(t *testing.T) {
	store := newFakeStore(t, map[string]string{
		lastHeartbeatKey: time.Now().UTC().Add(-time.Hour).Format(time.RFC3339),
	})
	store.setEnabled(true)
	sender := &fakeSender{}
	service := NewService(sender, store, store, store, testWorkers{})

	service.MaybeSendHeartbeat()
	time.Sleep(50 * time.Millisecond)
	if sender.heartbeatCount() != 0 {
		t.Fatal("heartbeat was sent before the interval elapsed")
	}
}

func TestEligibleHeartbeatAfterIntervalIsSent(t *testing.T) {
	store := newFakeStore(t, map[string]string{
		lastHeartbeatKey: time.Now().UTC().Add(-25 * time.Hour).Format(time.RFC3339),
	})
	store.setEnabled(true)
	sender := &fakeSender{}
	service := NewService(sender, store, store, store, testWorkers{})

	service.MaybeSendHeartbeat()
	waitFor(t, func() bool { return sender.heartbeatCount() == 1 })
}

func TestHeartbeatEligibilityUsesInjectableClock(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	store := newFakeStore(t, map[string]string{
		lastHeartbeatKey: now.Add(-23 * time.Hour).Format(time.RFC3339),
	})
	store.setEnabled(true)
	sender := &fakeSender{}
	service := NewService(sender, store, store, store, testWorkers{})
	service.now = func() time.Time { return now }

	service.MaybeSendHeartbeat()
	time.Sleep(50 * time.Millisecond)
	if sender.heartbeatCount() != 0 {
		t.Fatal("heartbeat sent before the 24h interval elapsed")
	}

	store.values[lastHeartbeatKey] = now.Add(-25 * time.Hour).Format(time.RFC3339)
	service.MaybeSendHeartbeat()
	waitFor(t, func() bool { return sender.heartbeatCount() == 1 })
}

func TestFailedHeartbeatDoesNotAdvanceLastSuccess(t *testing.T) {
	store := newFakeStore(t, map[string]string{})
	store.setEnabled(true)
	sender := &fakeSender{heartbeatError: errors.New("server down")}
	service := NewService(sender, store, store, store, testWorkers{})

	service.MaybeSendHeartbeat()
	waitFor(t, func() bool { return sender.heartbeatCount() == 1 })

	if value := store.values[lastHeartbeatKey]; value != "" {
		t.Fatalf("failed heartbeat persisted last-success time %q", value)
	}
}

func TestFailedHeartbeatStaysEligible(t *testing.T) {
	store := newFakeStore(t, map[string]string{})
	store.setEnabled(true)
	sender := &fakeSender{}
	service := NewService(sender, store, store, store, testWorkers{})

	service.sendHeartbeat(context.Background())
	service.sendHeartbeat(context.Background())
	if count := sender.heartbeatCount(); count != 2 {
		t.Fatalf("heartbeats sent = %d, want 2", count)
	}
}

func TestConcurrentHeartbeatsSendOnce(t *testing.T) {
	store := newFakeStore(t, map[string]string{})
	store.setEnabled(true)
	sender := &fakeSender{}
	service := NewService(sender, store, store, store, testWorkers{})

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			service.MaybeSendHeartbeat()
		}()
	}
	wg.Wait()
	waitFor(t, func() bool { return sender.heartbeatCount() == 1 })
	time.Sleep(50 * time.Millisecond)
	if count := sender.heartbeatCount(); count != 1 {
		t.Fatalf("concurrent callers sent %d heartbeats, want exactly 1", count)
	}
}

func TestEventDeliveredWhenEnabled(t *testing.T) {
	store := newFakeStore(t, map[string]string{})
	store.setEnabled(true)
	sender := &fakeSender{}
	service := NewService(sender, store, store, store, testWorkers{})

	service.Event(context.Background(), EventInstanceCreated)
	waitFor(t, func() bool { return sender.eventCount() == 1 })

	sender.mu.Lock()
	event := sender.events[0]
	sender.mu.Unlock()
	if event.Event != EventInstanceCreated {
		t.Fatalf("event name mismatch: %q", event.Event)
	}
	if event.InstallationID == "" {
		t.Fatal("event carried no installation ID")
	}
}

func TestUnknownEventNamesAreDropped(t *testing.T) {
	store := newFakeStore(t, map[string]string{})
	store.setEnabled(true)
	sender := &fakeSender{}
	service := NewService(sender, store, store, store, testWorkers{})

	service.Event(context.Background(), "not_in_allowlist")
	service.Event(context.Background(), "clicked_delete_button")
	time.Sleep(50 * time.Millisecond)
	if sender.eventCount() != 0 {
		t.Fatal("non-allowlisted event was transmitted")
	}
}

func TestDisablingPreventsFurtherEvents(t *testing.T) {
	store := newFakeStore(t, map[string]string{})
	store.setEnabled(true)
	sender := &fakeSender{}
	service := NewService(sender, store, store, store, testWorkers{})

	service.Event(context.Background(), EventInstanceCreated)
	waitFor(t, func() bool { return sender.eventCount() == 1 })

	store.setEnabled(false)
	service.Event(context.Background(), EventInstanceDeleted)
	service.Error(context.Background(), ErrorGameLaunchFailed, ComponentGameLauncher, OperationLaunchGame)
	service.MaybeSendHeartbeat()

	time.Sleep(50 * time.Millisecond)
	if sender.eventCount() != 1 || sender.errorCount() != 0 {
		t.Fatal("events or errors were transmitted after telemetry was disabled")
	}
}

func TestDisablingBeforeHeartbeatDeliveryPreventsTransmission(t *testing.T) {
	store := &blockingHeartbeatStore{
		fakeStore: newFakeStore(t, map[string]string{}),
		listed:    make(chan struct{}),
		release:   make(chan struct{}),
	}
	store.setEnabled(true)
	sender := &fakeSender{}
	service := NewService(sender, store, store, store, testWorkers{})

	done := make(chan struct{})
	go func() {
		service.sendHeartbeat(context.Background())
		close(done)
	}()
	<-store.listed
	store.setEnabled(false)
	close(store.release)
	<-done

	if sender.heartbeatCount() != 0 {
		t.Fatal("heartbeat was transmitted after telemetry was disabled")
	}
}

func TestErrorTaxonomyRejectsUnknownValues(t *testing.T) {
	store := newFakeStore(t, map[string]string{})
	store.setEnabled(true)
	sender := &fakeSender{}
	service := NewService(sender, store, store, store, testWorkers{})

	service.Error(context.Background(), "UNKNOWN_ERROR", ComponentLauncher, OperationLaunchGame)
	service.Error(context.Background(), ErrorGameLaunchFailed, "not_a_component", OperationLaunchGame)
	service.Error(context.Background(), ErrorGameLaunchFailed, ComponentGameLauncher, "not_an_operation")
	time.Sleep(50 * time.Millisecond)
	if sender.errorCount() != 0 {
		t.Fatal("out-of-taxonomy error values were transmitted")
	}
}

func TestStructuredErrorPayloadHasNoRawText(t *testing.T) {
	store := newFakeStore(t, map[string]string{})
	store.setEnabled(true)
	sender := &fakeSender{}
	service := NewService(sender, store, store, store, testWorkers{})

	service.Error(context.Background(), ErrorModDownloadHTTP404, ComponentModDownloader, OperationDownloadMod)
	waitFor(t, func() bool { return sender.errorCount() == 1 })

	sender.mu.Lock()
	report := sender.errors[0]
	sender.mu.Unlock()
	if report.ErrorCode != "MOD_DOWNLOAD_HTTP_404" {
		t.Fatalf("error code mismatch: %q", report.ErrorCode)
	}
}

func TestHeartbeatPayloadContainsOnlyApprovedFields(t *testing.T) {
	store := newFakeStore(t, map[string]string{})
	store.setEnabled(true)
	store.instances = []instances.Instance{{ID: "a", Name: "secret-instance-name"}}
	store.mods["a"] = []mods.InstalledMod{{ID: "m1", Name: "secret-mod-name", FilePath: "/home/user/.waxlight/instances/a/Mods/secret.zip"}}
	sender := &fakeSender{}
	service := NewService(sender, store, store, store, testWorkers{})

	service.sendHeartbeat(context.Background())
	heartbeat, ok := sender.lastHeartbeat()
	if !ok {
		t.Fatal("no heartbeat recorded")
	}
	if heartbeat.InstancesCount != 1 || heartbeat.ModsCount != 1 {
		t.Fatalf("counts mismatch: %+v", heartbeat)
	}
	encoded := marshalJSON(t, heartbeat)
	for _, forbidden := range []string{"secret-instance-name", "secret-mod-name", "/home/user", ".zip"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("heartbeat payload leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestSettingsErrorFailsClosed(t *testing.T) {
	store := newFakeStore(t, map[string]string{})
	store.setEnabled(true)
	store.setErr(errors.New("database locked"))
	sender := &fakeSender{}
	service := NewService(sender, store, store, store, testWorkers{})

	service.Event(context.Background(), EventInstanceCreated)
	service.MaybeSendHeartbeat()
	time.Sleep(50 * time.Millisecond)
	if sender.eventCount() != 0 || sender.heartbeatCount() != 0 {
		t.Fatal("telemetry transmitted while settings were unavailable")
	}
}

func marshalJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(encoded)
}
