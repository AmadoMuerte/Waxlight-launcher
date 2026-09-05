package presence

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	settingscore "github.com/AmadoMuerte/Waxlight-launcher/internal/settings"
)

type fakeSettingsRepository struct {
	value settingscore.Settings
}

func (repository *fakeSettingsRepository) GetSettings(context.Context) (settingscore.Settings, error) {
	return repository.value, nil
}

func (repository *fakeSettingsRepository) SaveSettings(_ context.Context, value settingscore.Settings) error {
	repository.value = value
	return nil
}

type fakeClient struct {
	mu         sync.Mutex
	connected  bool
	activities []Activity
	cleared    int
	closed     int
	activity   chan struct{}
	closeEvent chan struct{}
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		connected:  true,
		activity:   make(chan struct{}, 8),
		closeEvent: make(chan struct{}, 2),
	}
}

func (client *fakeClient) Connected() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.connected
}

func (client *fakeClient) SetActivity(activity Activity) error {
	client.mu.Lock()
	client.activities = append(client.activities, activity)
	client.mu.Unlock()
	client.activity <- struct{}{}
	return nil
}

func (client *fakeClient) ClearActivity() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.cleared++
	return nil
}

func (client *fakeClient) Close() {
	client.mu.Lock()
	client.connected = false
	client.closed++
	client.mu.Unlock()
	client.closeEvent <- struct{}{}
}

func (client *fakeClient) snapshot() ([]Activity, int, int) {
	client.mu.Lock()
	defer client.mu.Unlock()
	return append([]Activity(nil), client.activities...), client.cleared, client.closed
}

func waitForSignal(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func testService(richPresenceEnabled bool) (*Service, *fakeSettingsRepository, *fakeClient, *atomic.Int32) {
	repository := &fakeSettingsRepository{
		value: settingscore.Settings{RichPresenceEnabled: richPresenceEnabled},
	}
	client := newFakeClient()
	var dialCalls atomic.Int32
	service := NewService(settingscore.NewReader(repository), func(string) Client {
		dialCalls.Add(1)
		return client
	})
	return service, repository, client, &dialCalls
}

func TestNewServiceDoesNotConnect(t *testing.T) {
	service, _, _, dialCalls := testService(true)
	defer service.Close()
	if service.client != nil || dialCalls.Load() != 0 {
		t.Fatal("NewService() must not connect to Discord")
	}
	if AppID == "" {
		t.Fatal("the Discord application ID must not be empty")
	}
}

func TestGameStartedAndStoppedNoopWithoutConnection(t *testing.T) {
	service, _, client, dialCalls := testService(false)
	defer service.Close()

	service.GameStarted()
	service.GameStopped()

	activities, _, _ := client.snapshot()
	if len(activities) != 0 || dialCalls.Load() != 0 {
		t.Fatal("game activity methods must not connect or publish while disconnected")
	}
}

func TestSetEnabledFalseNoop(t *testing.T) {
	service, _, _, dialCalls := testService(false)
	defer service.Close()
	service.SetEnabled(t.Context(), false)
	if dialCalls.Load() != 0 {
		t.Fatal("SetEnabled(false) must not connect")
	}
}

func TestConnectAndGameActivity(t *testing.T) {
	service, _, client, dialCalls := testService(true)
	defer service.Close()

	service.Connect(t.Context())
	waitForSignal(t, client.activity, "initial idle activity")
	service.GameStarted()
	service.GameStopped()

	activities, _, _ := client.snapshot()
	if dialCalls.Load() != 1 {
		t.Fatalf("Dial calls = %d, want 1", dialCalls.Load())
	}
	if len(activities) != 3 {
		t.Fatalf("activities = %d, want idle, playing, idle", len(activities))
	}
	if activities[0].State != idleActivityState || activities[1].State != playingActivityState || activities[1].StartTimestamp == nil || activities[2].State != idleActivityState {
		t.Fatalf("activities = %+v", activities)
	}
	if delta := time.Now().Unix() - *activities[1].StartTimestamp; delta < 0 || delta > 1 {
		t.Fatalf("playing start timestamp = %d, want current Unix seconds", *activities[1].StartTimestamp)
	}
}

func TestGameActivityDetectsConnectionLoss(t *testing.T) {
	service, _, client, _ := testService(true)
	defer service.Close()
	service.Connect(t.Context())
	waitForSignal(t, client.activity, "initial idle activity")

	client.mu.Lock()
	client.connected = false
	client.mu.Unlock()
	service.GameStarted()
	service.GameStopped()

	activities, _, _ := client.snapshot()
	if service.connected || len(activities) != 1 {
		t.Fatalf("lost connection: service.connected=%v activities=%+v", service.connected, activities)
	}
}

func TestFastGameStopLeavesIdle(t *testing.T) {
	service, _, client, _ := testService(true)
	defer service.Close()
	service.Connect(t.Context())
	waitForSignal(t, client.activity, "initial idle activity")

	service.GameStarted()
	service.GameStopped()

	activities, _, _ := client.snapshot()
	if got := activities[len(activities)-1].State; got != idleActivityState {
		t.Fatalf("final state = %q, want idle (activities = %+v)", got, activities)
	}
}

func TestSetEnabledUsesPersistedSetting(t *testing.T) {
	service, repository, client, dialCalls := testService(false)
	defer service.Close()
	repository.value.RichPresenceEnabled = true

	service.SetEnabled(t.Context(), true)
	waitForSignal(t, client.activity, "initial idle activity")

	activities, _, _ := client.snapshot()
	if dialCalls.Load() != 1 {
		t.Fatalf("SetEnabled(true): dial calls = %d, want 1", dialCalls.Load())
	}
	if len(activities) != 1 || activities[0].State != idleActivityState {
		t.Fatalf("activities = %+v, want idle", activities)
	}
}

func TestSetEnabledFalseClearsAndCloses(t *testing.T) {
	service, _, client, _ := testService(true)
	service.Connect(t.Context())
	waitForSignal(t, client.activity, "initial idle activity")

	service.SetEnabled(t.Context(), false)
	service.Close()

	_, cleared, closed := client.snapshot()
	if cleared != 1 || closed != 1 {
		t.Fatalf("disabled client: cleared=%d closed=%d", cleared, closed)
	}
}

func TestCloseDuringDialClosesLateClient(t *testing.T) {
	repository := &fakeSettingsRepository{value: settingscore.Settings{RichPresenceEnabled: true}}
	client := newFakeClient()
	dialStarted := make(chan struct{})
	releaseDial := make(chan struct{})
	service := NewService(settingscore.NewReader(repository), func(string) Client {
		close(dialStarted)
		<-releaseDial
		return client
	})

	service.Connect(t.Context())
	waitForSignal(t, dialStarted, "dial start")
	service.Close()
	close(releaseDial)
	waitForSignal(t, client.closeEvent, "late client close")

	activities, _, closed := client.snapshot()
	if closed != 1 || len(activities) != 0 {
		t.Fatalf("late client: closed=%d activities=%+v", closed, activities)
	}
}

func TestDisableDuringDialClosesLateClient(t *testing.T) {
	repository := &fakeSettingsRepository{value: settingscore.Settings{RichPresenceEnabled: true}}
	client := newFakeClient()
	dialStarted := make(chan struct{})
	releaseDial := make(chan struct{})
	service := NewService(settingscore.NewReader(repository), func(string) Client {
		close(dialStarted)
		<-releaseDial
		return client
	})
	defer service.Close()

	service.Connect(t.Context())
	waitForSignal(t, dialStarted, "dial start")
	service.SetEnabled(t.Context(), false)
	close(releaseDial)
	waitForSignal(t, client.closeEvent, "late client close")

	activities, _, closed := client.snapshot()
	if service.connected || closed != 1 || len(activities) != 0 {
		t.Fatalf("late client after disable: connected=%v closed=%d activities=%+v", service.connected, closed, activities)
	}
}

func TestOperationsAfterCloseAreNoops(t *testing.T) {
	service, _, client, dialCalls := testService(true)
	service.Connect(t.Context())
	waitForSignal(t, client.activity, "initial idle activity")
	service.Close()

	service.GameStarted()
	service.GameStopped()
	service.Connect(t.Context())
	service.SetEnabled(t.Context(), true)

	activities, _, closed := client.snapshot()
	if dialCalls.Load() != 1 || closed != 1 || len(activities) != 1 {
		t.Fatalf("post-close state: dialCalls=%d closed=%d activities=%+v", dialCalls.Load(), closed, activities)
	}
}
