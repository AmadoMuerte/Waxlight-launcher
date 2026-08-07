package application_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/modstorage"
	"github.com/waxlight/waxlight-launcher/internal/telemetry"
)

// telemetryRecorder implements telemetry.Sender without any network activity.
// It records which allowlisted events and structured errors reach the
// telemetry service from the application operation boundaries.
type telemetryRecorder struct {
	mu     sync.Mutex
	events []string
	errors []telemetry.ErrorEvent
}

func (recorder *telemetryRecorder) SendHeartbeat(context.Context, telemetry.Heartbeat) error {
	return nil
}

func (recorder *telemetryRecorder) SendEvent(_ context.Context, event telemetry.Event) error {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.events = append(recorder.events, event.Event)
	return nil
}

func (recorder *telemetryRecorder) SendError(_ context.Context, event telemetry.ErrorEvent) error {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.errors = append(recorder.errors, event)
	return nil
}

func (recorder *telemetryRecorder) waitForEvent(t *testing.T, name string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		recorder.mu.Lock()
		found := false
		for _, event := range recorder.events {
			if event == name {
				found = true
				break
			}
		}
		recorder.mu.Unlock()
		if found {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("event %q was not recorded at its operation boundary", name)
}

func (recorder *telemetryRecorder) waitForError(t *testing.T, code string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		recorder.mu.Lock()
		found := false
		for _, report := range recorder.errors {
			if report.ErrorCode == code {
				found = true
				break
			}
		}
		recorder.mu.Unlock()
		if found {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("structured error %q was not recorded", code)
}

func (recorder *telemetryRecorder) counts() (events int, errors int) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return len(recorder.events), len(recorder.errors)
}

func newTelemetryFixture(t *testing.T) (testFixture, *telemetryRecorder) {
	t.Helper()
	fixture := newTestFixture(t)
	recorder := &telemetryRecorder{}
	fixture.service.ConfigureTelemetry(telemetry.NewService(recorder, fixture.service))

	settings, err := fixture.service.GetSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	settings.TelemetryEnabled = true
	if _, err := fixture.service.SaveSettings(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	return fixture, recorder
}

func TestTelemetryInstanceEventsAtSuccessBoundaries(t *testing.T) {
	fixture, recorder := newTelemetryFixture(t)
	ctx := context.Background()

	instance, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name:          "Telemetry Test",
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder.waitForEvent(t, telemetry.EventInstanceCreated)

	if err := fixture.service.DeleteInstance(ctx, instance.ID, true); err != nil {
		t.Fatal(err)
	}
	recorder.waitForEvent(t, telemetry.EventInstanceDeleted)
}

func TestTelemetryModEventsAtSuccessBoundaries(t *testing.T) {
	fixture, recorder := newTelemetryFixture(t)
	ctx := context.Background()

	// Catalog download stores the mod in the library: mod_downloaded fires at
	// the authoritative storage boundary. No network is involved; the
	// recording downloader writes local bytes.
	details := domain.ModDetails{ModSummary: domain.ModSummary{ID: "51", Name: "Player Corpse", AuthorName: "Ada", LatestVersion: "2.0.0"}, Versions: []domain.ModVersion{{
		ID: "7", Version: "2.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable",
		FileName: "playercorpse.zip", DownloadURL: "https://cdn.test/playercorpse.zip",
	}}}
	fixture.service.ConfigureVersionDownloads(nil, recordingDownloader{}, nil)
	fixture.service.ConfigureMods(staticModCatalog{details: details}, modstorage.New(fixture.root))
	if _, err := fixture.service.DownloadCatalogMod(ctx, domain.DownloadModRequest{
		ModID: "51", VersionID: "7", DownloadOnly: true,
	}); err != nil {
		t.Fatal(err)
	}
	recorder.waitForEvent(t, telemetry.EventModDownloaded)

	if err := fixture.service.RemoveDownloadedMod(ctx, "51", "7"); err != nil {
		t.Fatal(err)
	}
	recorder.waitForEvent(t, telemetry.EventModRemoved)

	// Instance-level mod removal: install a local file, then remove it.
	instance, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(fixture.root, "sample-mod.zip")
	if err := os.WriteFile(sourcePath, []byte("mod"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.InstallModFile(ctx, instance.ID, sourcePath, "Sample Mod", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	mods, err := fixture.service.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 1 {
		t.Fatalf("expected one installed mod, got %d", len(mods))
	}
	if err := fixture.service.DeleteMod(ctx, mods[0].ID); err != nil {
		t.Fatal(err)
	}
	recorder.waitForEvent(t, telemetry.EventModRemoved)
}

func TestTelemetryGameLaunchEvents(t *testing.T) {
	fixture, recorder := newTelemetryFixture(t)
	ctx := context.Background()

	instance, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.Launch(ctx, instance.ID, nil); err != nil {
		t.Fatal(err)
	}
	recorder.waitForEvent(t, telemetry.EventGameLaunchSucceeded)

	// A separate instance whose process start fails exercises the
	// game_launch_failed boundary (the first instance is still running).
	failing, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.launcher.startErr = errors.New("process start failed")
	if _, err := fixture.service.Launch(ctx, failing.ID, nil); err == nil {
		t.Fatal("expected launch to fail")
	}
	recorder.waitForEvent(t, telemetry.EventGameLaunchFailed)
	recorder.waitForError(t, telemetry.ErrorGameLaunchFailed)
}

func TestTelemetryDisabledEmitsNothing(t *testing.T) {
	fixture := newTestFixture(t)
	recorder := &telemetryRecorder{}
	fixture.service.ConfigureTelemetry(telemetry.NewService(recorder, fixture.service))

	// Explicitly disable telemetry; the default is enabled, so the opt-out
	// must be what prevents transmission.
	settings, err := fixture.service.GetSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	settings.TelemetryEnabled = false
	if _, err := fixture.service.SaveSettings(context.Background(), settings); err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.service.CreateInstance(context.Background(), application.CreateInstanceInput{
		GameVersionID: "1.20",
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if events, reports := recorder.counts(); events != 0 || reports != 0 {
		t.Fatalf("disabled telemetry recorded events=%d errors=%d", events, reports)
	}
}
