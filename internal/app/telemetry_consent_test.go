package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	settingscore "github.com/waxlight/waxlight-launcher/internal/settings"
)

type fakeTelemetryConsentStore struct {
	persisted string
	settings  settingscore.Settings
	saved     bool
}

func (f *fakeTelemetryConsentStore) GetSettingValue(context.Context, string) (string, error) {
	return f.persisted, nil
}
func (f *fakeTelemetryConsentStore) GetSettings(context.Context) (settingscore.Settings, error) {
	return f.settings, nil
}
func (f *fakeTelemetryConsentStore) SaveSettings(_ context.Context, settings settingscore.Settings) error {
	f.settings = settings
	f.saved = true
	return nil
}

func writeTelemetryMarker(t *testing.T, home, value string) string {
	t.Helper()
	path := filepath.Join(home, installerTelemetryOptInFile)
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInstallerTelemetryOptInEnablesTelemetryForFreshSettings(t *testing.T) {
	home := t.TempDir()
	marker := writeTelemetryMarker(t, home, "1")
	store := &fakeTelemetryConsentStore{settings: settingscore.Settings{TelemetryEnabled: false}}
	if err := applyInstallerTelemetryConsent(context.Background(), store, home); err != nil {
		t.Fatal(err)
	}
	if !store.saved || !store.settings.TelemetryEnabled {
		t.Fatal("explicit installer opt-in did not enable telemetry")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("installer marker was not consumed: %v", err)
	}
}

func TestInstallerTelemetryOptInDoesNotOverrideExistingSettings(t *testing.T) {
	home := t.TempDir()
	marker := writeTelemetryMarker(t, home, "1")
	store := &fakeTelemetryConsentStore{persisted: `{"telemetryEnabled":false}`, settings: settingscore.Settings{TelemetryEnabled: false}}
	if err := applyInstallerTelemetryConsent(context.Background(), store, home); err != nil {
		t.Fatal(err)
	}
	if store.saved || store.settings.TelemetryEnabled {
		t.Fatal("installer choice overrode existing user settings")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("installer marker was not consumed: %v", err)
	}
}

func TestInstallerTelemetryChoiceFailsClosed(t *testing.T) {
	home := t.TempDir()
	writeTelemetryMarker(t, home, "unexpected")
	store := &fakeTelemetryConsentStore{settings: settingscore.Settings{TelemetryEnabled: false}}
	if err := applyInstallerTelemetryConsent(context.Background(), store, home); err != nil {
		t.Fatal(err)
	}
	if store.saved || store.settings.TelemetryEnabled {
		t.Fatal("invalid installer marker enabled telemetry")
	}
}
