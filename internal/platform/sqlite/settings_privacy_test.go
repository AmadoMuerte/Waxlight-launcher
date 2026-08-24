package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/sqlite"
)

func TestTelemetryDefaultsToDisabled(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "privacy-default.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	settings, err := store.GetSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if settings.TelemetryEnabled {
		t.Fatal("telemetry must be disabled by default for a fresh installation")
	}
}

func TestLegacySettingsWithoutTelemetryPreferenceRemainDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-settings.db")
	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO app_settings(key, value) VALUES ('settings', '{"language":"ru"}')`); err != nil {
		t.Fatal(err)
	}

	settings, err := store.GetSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if settings.TelemetryEnabled {
		t.Fatal("legacy settings without a telemetry preference must remain opted out")
	}
}
