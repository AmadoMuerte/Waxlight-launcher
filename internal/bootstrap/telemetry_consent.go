package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

const installerTelemetryOptInFile = "installer-telemetry-opt-in"

type telemetryConsentStore interface {
	GetSettingValue(context.Context, string) (string, error)
	GetSettings(context.Context) (domain.Settings, error)
	SaveSettings(context.Context, domain.Settings) error
}

// applyInstallerTelemetryConsent consumes the one-time marker written by the
// interactive Windows installer. Telemetry defaults to disabled in the normal
// settings path, so only an explicit installer opt-in may enable it here.
//
// Existing settings always win. This is important for silent updates: an
// installer run must never reset a user's in-app privacy choice.
func applyInstallerTelemetryConsent(ctx context.Context, store telemetryConsentStore, home string) error {
	markerPath := filepath.Join(home, installerTelemetryOptInFile)
	marker, err := os.ReadFile(markerPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read installer telemetry choice: %w", err)
	}

	defer func() {
		if err := os.Remove(markerPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("bootstrap: could not remove installer telemetry choice", "error", err)
		}
	}()

	persisted, err := store.GetSettingValue(ctx, "settings")
	if err != nil {
		return fmt.Errorf("check persisted settings before installer telemetry choice: %w", err)
	}
	if strings.TrimSpace(persisted) != "" {
		return nil
	}

	// Fail closed. Only the exact marker emitted by the installer can opt in.
	if strings.TrimSpace(string(marker)) != "1" {
		return nil
	}

	settings, err := store.GetSettings(ctx)
	if err != nil {
		return fmt.Errorf("load default settings for installer telemetry choice: %w", err)
	}
	settings.TelemetryEnabled = true
	if err := store.SaveSettings(ctx, settings); err != nil {
		return fmt.Errorf("save installer telemetry choice: %w", err)
	}
	return nil
}
