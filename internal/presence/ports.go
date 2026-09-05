// Package presence implements Discord Rich Presence. It is a feature package:
// it owns the Rich Presence service only and reaches Discord through an
// injected port, never through platform, Wails, or transport code.
package presence

import (
	"context"

	settingscore "github.com/AmadoMuerte/Waxlight-launcher/internal/settings"
)

// SettingsReader provides access to launcher settings.
type SettingsReader interface {
	Get(context.Context) (settingscore.Settings, error)
}

// Activity is the launcher-owned Rich Presence state sent to Discord.
type Activity struct {
	State          string
	Details        string
	LargeImageKey  string
	LargeImageText string
	SmallImageKey  string
	SmallImageText string
	StartTimestamp *int64
}

// Client is the Discord IPC port used by the presence service.
type Client interface {
	Connected() bool
	SetActivity(Activity) error
	ClearActivity() error
	Close()
}

// Dialer connects to Discord and returns nil when Discord is unavailable.
type Dialer func(appID string) Client
