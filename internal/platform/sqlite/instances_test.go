package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/platform/sqlite"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

func TestInstancesPersistListAndDelete(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "instances.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Date(2026, time.August, 11, 8, 0, 0, 0, time.UTC)
	if err := store.SaveVersion(ctx, versions.GameVersion{
		ID: "version", Name: "Version", Channel: "stable", Platform: "linux", Architecture: "amd64",
		InstallationDir: "/game", ExecutablePath: "/game/Vintagestory", Status: "installed", InstalledAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	coverPath := "/instances/primary/cover.png"
	lastPlayedAt := now.Add(2 * time.Hour)
	primary := instances.Instance{
		ID: "primary", Name: "Primary", Description: "Description", GameVersionID: "version",
		Directory: "/instances/primary", CoverPath: &coverPath,
		Status: instances.StatusReady, LaunchArguments: []string{"--foo", "bar baz"},
		LastPlayedAt: &lastPlayedAt, CreatedAt: now, UpdatedAt: now.Add(time.Hour),
	}
	if err := store.SaveInstance(ctx, primary); err != nil {
		t.Fatal(err)
	}
	secondary := instances.Instance{
		ID: "secondary", Name: "Secondary", GameVersionID: "version", Directory: "/instances/secondary",
		Status: instances.StatusReady, CreatedAt: now.Add(time.Hour), UpdatedAt: now.Add(time.Hour),
	}
	if err := store.SaveInstance(ctx, secondary); err != nil {
		t.Fatal(err)
	}

	stored, err := store.GetInstance(ctx, primary.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Name != primary.Name || stored.DefaultAccountID != nil || stored.CoverPath == nil || *stored.CoverPath != coverPath || stored.LastPlayedAt == nil || !stored.LastPlayedAt.Equal(lastPlayedAt) || len(stored.LaunchArguments) != 2 || stored.LaunchArguments[1] != "bar baz" {
		t.Fatalf("unexpected stored instance: %+v", stored)
	}
	listed, err := store.ListInstances(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].ID != primary.ID {
		t.Fatalf("unexpected instance ordering: %+v", listed)
	}

	secondary.Directory = primary.Directory
	if err := store.SaveInstance(ctx, secondary); appErrorCode(err) != instances.ErrDirectoryConflict {
		t.Fatalf("directory conflict error = %v", err)
	}
	if err := store.DeleteInstance(ctx, primary.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetInstance(ctx, primary.ID); appErrorCode(err) != instances.ErrInstanceNotFound {
		t.Fatalf("missing instance error = %v", err)
	}
	if err := store.DeleteInstance(ctx, primary.ID); appErrorCode(err) != instances.ErrInstanceNotFound {
		t.Fatalf("missing delete error = %v", err)
	}
}

func appErrorCode(err error) string {
	var appError *errs.AppError
	if errors.As(err, &appError) {
		return appError.Code
	}
	return ""
}
