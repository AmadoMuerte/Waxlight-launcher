package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/sqlite"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/settings"
)

func TestSettingsPersistGlobalEnvironmentVariables(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	value := settings.Defaults()
	value.GlobalEnvironmentVariables = map[string]string{
		"MESA_GLTHREAD": "true",
		"EMPTY_VALUE":   "",
	}
	if err := store.SaveSettings(ctx, value); err != nil {
		t.Fatal(err)
	}

	stored, err := store.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stored.GlobalEnvironmentVariables["MESA_GLTHREAD"] != "true" || stored.GlobalEnvironmentVariables["EMPTY_VALUE"] != "" {
		t.Fatalf("global environment variables were not persisted: %#v", stored.GlobalEnvironmentVariables)
	}
}

func TestSettingsNormalizeMissingGlobalEnvironmentVariables(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	value := settings.Defaults()
	value.GlobalEnvironmentVariables = nil
	if err := store.SaveSettings(ctx, value); err != nil {
		t.Fatal(err)
	}

	stored, err := store.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stored.GlobalEnvironmentVariables == nil {
		t.Fatal("missing global environment variables were not normalized to an empty map")
	}
}
