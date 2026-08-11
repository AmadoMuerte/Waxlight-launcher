package application_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/mods"
	settingscore "github.com/waxlight/waxlight-launcher/internal/settings"
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
	return newTelemetryFixtureWithCatalog(t, nil)
}

func newTelemetryFixtureWithCatalog(t *testing.T, modCatalog mods.Catalog) (testFixture, *telemetryRecorder) {
	t.Helper()
	fixture := newTestFixtureWithMods(t, modCatalog)
	recorder := &telemetryRecorder{}
	telemetryService := telemetry.NewService(recorder, fixture.settings, fixture.store, fixture.store, fixture.lifecycle)
	fixture.modTelemetry.current = telemetryService
	fixture.setCreateTelemetry(telemetryService)
	updates := settingscore.NewService(fixture.store, fixture.settings, telemetryService, telemetryService, nil)

	settings, err := fixture.settings.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	settings.TelemetryEnabled = true
	if _, err := updates.Update(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	return fixture, recorder
}

func TestTelemetryInstanceEventsAtSuccessBoundaries(t *testing.T) {
	fixture, recorder := newTelemetryFixture(t)
	ctx := context.Background()

	instance, err := fixture.service.CreateInstance(ctx, instances.CreateInput{
		Name:          "Telemetry Test",
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder.waitForEvent(t, telemetry.EventInstanceCreated)

	if err := fixture.service.InstanceDeleter().Delete(ctx, instance.ID, true); err != nil {
		t.Fatal(err)
	}
	recorder.waitForEvent(t, telemetry.EventInstanceDeleted)
}

func TestTelemetryModEventsAtSuccessBoundaries(t *testing.T) {
	details := mods.ModDetails{ModSummary: mods.ModSummary{ID: "51", Name: "Player Corpse", AuthorName: "Ada", LatestVersion: "2.0.0"}, Versions: []mods.ModVersion{{
		ID: "7", Version: "2.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable",
		FileName: "playercorpse.zip", DownloadURL: "https://cdn.test/playercorpse.zip",
	}}}
	fixture, recorder := newTelemetryFixtureWithCatalog(t, staticModCatalog{details: details})
	ctx := context.Background()

	// Catalog download stores the mod in the library: mod_downloaded fires at
	// the authoritative storage boundary. No network is involved; the
	// recording downloader writes local bytes.
	fixture.downloader.Set(recordingDownloader{})
	if _, err := fixture.modsCatalog.DownloadCatalogMod(ctx, mods.DownloadModRequest{
		ModID: "51", VersionID: "7", DownloadOnly: true,
	}); err != nil {
		t.Fatal(err)
	}
	recorder.waitForEvent(t, telemetry.EventModDownloaded)

	if err := fixture.modsCatalog.RemoveDownloadedMod(ctx, "51", "7"); err != nil {
		t.Fatal(err)
	}
	recorder.waitForEvent(t, telemetry.EventModRemoved)

	// Instance-level mod removal: install a local file, then remove it.
	instance, err := fixture.service.CreateInstance(ctx, instances.CreateInput{
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(fixture.root, "sample-mod.zip")
	if err := os.WriteFile(sourcePath, []byte("mod"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.mods.InstallModFile(ctx, instance.ID, sourcePath, "Sample Mod", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	mods, err := fixture.mods.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 1 {
		t.Fatalf("expected one installed mod, got %d", len(mods))
	}
	if err := fixture.mods.DeleteMod(ctx, mods[0].ID, false); err != nil {
		t.Fatal(err)
	}
	recorder.waitForEvent(t, telemetry.EventModRemoved)
}

func TestTelemetryGameLaunchEvents(t *testing.T) {
	fixture, recorder := newTelemetryFixture(t)
	ctx := context.Background()

	instance, err := fixture.service.CreateInstance(ctx, instances.CreateInput{
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.launching.Launch(ctx, instance.ID, nil); err != nil {
		t.Fatal(err)
	}
	recorder.waitForEvent(t, telemetry.EventGameLaunchSucceeded)

	// A separate instance whose process start fails exercises the
	// game_launch_failed boundary (the first instance is still running).
	failing, err := fixture.service.CreateInstance(ctx, instances.CreateInput{
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.launcher.startErr = errors.New("process start failed")
	if _, err := fixture.launching.Launch(ctx, failing.ID, nil); err == nil {
		t.Fatal("expected launch to fail")
	}
	recorder.waitForEvent(t, telemetry.EventGameLaunchFailed)
	recorder.waitForError(t, telemetry.ErrorGameLaunchFailed)
}

func TestTelemetryDisabledEmitsNothing(t *testing.T) {
	fixture := newTestFixture(t)
	recorder := &telemetryRecorder{}
	telemetryService := telemetry.NewService(recorder, fixture.settings, fixture.store, fixture.store, fixture.lifecycle)
	fixture.modTelemetry.current = telemetryService
	fixture.setCreateTelemetry(telemetryService)
	updates := settingscore.NewService(fixture.store, fixture.settings, telemetryService, telemetryService, nil)

	// Explicitly disable telemetry; the default is enabled, so the opt-out
	// must be what prevents transmission.
	settings, err := fixture.settings.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	settings.TelemetryEnabled = false
	if _, err := updates.Update(context.Background(), settings); err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.service.CreateInstance(context.Background(), instances.CreateInput{
		GameVersionID: "1.20",
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if events, reports := recorder.counts(); events != 0 || reports != 0 {
		t.Fatalf("disabled telemetry recorded events=%d errors=%d", events, reports)
	}
}
