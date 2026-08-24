package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/sqlite"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/sessions"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/versions"
)

func TestPlaySessionsPersistFinishAndRecover(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	startedAt := time.Date(2026, time.August, 11, 10, 0, 0, 0, time.UTC)
	if err := store.SaveVersion(ctx, versions.GameVersion{
		ID: "version", Name: "Version", Channel: "stable", Platform: "linux", Architecture: "amd64",
		InstallationDir: "/game", ExecutablePath: "/game/Vintagestory", Status: "installed", InstalledAt: startedAt,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveInstance(ctx, instances.Instance{
		ID: "instance", Name: "Instance", GameVersionID: "version", Directory: "/instance",
		Status: "running", CreatedAt: startedAt, UpdatedAt: startedAt,
	}); err != nil {
		t.Fatal(err)
	}

	processID := 4242
	if err := store.SaveSession(ctx, sessions.PlaySession{
		ID: "finished", InstanceID: "instance", VersionID: "version", ProcessID: &processID, StartedAt: startedAt,
	}); err != nil {
		t.Fatal(err)
	}
	endedAt := startedAt.Add(2 * time.Minute)
	if err := store.FinishSession(ctx, "finished", endedAt, 3, true, 120); err != nil {
		t.Fatal(err)
	}

	playSessions, err := store.ListSessions(ctx, "instance", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(playSessions) != 1 {
		t.Fatalf("unexpected session count: %d", len(playSessions))
	}
	finished := playSessions[0]
	if finished.ProcessID == nil || *finished.ProcessID != processID || finished.EndedAt == nil || !finished.EndedAt.Equal(endedAt) || finished.ExitCode == nil || *finished.ExitCode != 3 || !finished.Crashed || finished.DurationSec != 120 {
		t.Fatalf("unexpected finished session: %+v", finished)
	}

	openStartedAt := startedAt.Add(3 * time.Minute)
	if err := store.SaveSession(ctx, sessions.PlaySession{
		ID: "open", InstanceID: "instance", VersionID: "version", StartedAt: openStartedAt,
	}); err != nil {
		t.Fatal(err)
	}
	recoveredAt := openStartedAt.Add(90 * time.Second)
	if err := store.RecoverOpenSessions(ctx, recoveredAt); err != nil {
		t.Fatal(err)
	}

	playSessions, err = store.ListSessions(ctx, "instance", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(playSessions) != 2 || playSessions[0].ID != "open" || !playSessions[0].Recovered || !playSessions[0].Crashed || playSessions[0].DurationSec != 90 {
		t.Fatalf("unexpected recovered sessions: %+v", playSessions)
	}
	instance, err := store.GetInstance(ctx, "instance")
	if err != nil {
		t.Fatal(err)
	}
	if instance.Status != "ready" {
		t.Fatalf("instance status = %q, want ready", instance.Status)
	}
	totals, err := store.SessionStatistics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if totals.TotalPlaytimeSeconds != 210 || totals.LaunchCount != 2 || totals.MostPlayedInstanceID == nil || *totals.MostPlayedInstanceID != "instance" {
		t.Fatalf("unexpected session statistics: %+v", totals)
	}
	playtime, err := store.InstancePlaytime(ctx, "instance")
	if err != nil || playtime != 210 {
		t.Fatalf("unexpected instance playtime: %d, %v", playtime, err)
	}
}

func TestSessionStatisticsHaveNoMostPlayedInstanceWithoutPlaytime(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "zero-playtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Date(2026, time.August, 11, 10, 0, 0, 0, time.UTC)
	if err := store.SaveVersion(ctx, versions.GameVersion{
		ID: "version", Name: "Version", Channel: "stable", Platform: "linux", Architecture: "amd64",
		InstallationDir: "/game", ExecutablePath: "/game/Vintagestory", Status: "installed", InstalledAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveInstance(ctx, instances.Instance{
		ID: "instance", Name: "Instance", GameVersionID: "version", Directory: "/instance",
		Status: instances.StatusReady, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSession(ctx, sessions.PlaySession{
		ID: "open", InstanceID: "instance", VersionID: "version", StartedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	totals, err := store.SessionStatistics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if totals.LaunchCount != 1 || totals.TotalPlaytimeSeconds != 0 || totals.MostPlayedInstanceID != nil {
		t.Fatalf("unexpected zero-playtime statistics: %+v", totals)
	}
}
