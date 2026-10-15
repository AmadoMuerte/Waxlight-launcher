package wails

import (
	"context"
	"errors"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/settings"
)

// fakeSettingsReader returns a fixed settings document or error.
type fakeSettingsReader struct {
	value settings.Settings
	err   error
}

func (reader fakeSettingsReader) Get(context.Context) (settings.Settings, error) {
	return reader.value, reader.err
}

// TestShouldHideWindow covers the close-to-tray decision. It targets the pure
// decision rather than CloseRequested/Quit because the Wails runtime calls
// (WindowHide and Quit) terminate the process without a live application
// context, so they cannot run in a unit test.
func TestShouldHideWindow(t *testing.T) {
	tests := []struct {
		name     string
		reader   fakeSettingsReader
		quitting bool
		want     bool
		wantErr  bool
	}{
		{
			name:   "close to tray hides the window",
			reader: fakeSettingsReader{value: settings.Settings{CloseToTray: true}},
			want:   true,
		},
		{
			name:   "close to tray disabled closes the window",
			reader: fakeSettingsReader{value: settings.Settings{CloseToTray: false}},
			want:   false,
		},
		{
			name:    "settings error fails open to closing",
			reader:  fakeSettingsReader{err: errors.New("settings unavailable")},
			want:    false,
			wantErr: true,
		},
		{
			// Mirrors the state Quit sets before Wails is asked to quit.
			name:     "quitting overrides close to tray",
			reader:   fakeSettingsReader{value: settings.Settings{CloseToTray: true}},
			quitting: true,
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewWindowAdapter(newTestLifecycle(), tt.reader)
			if tt.quitting {
				adapter.quitting.Store(true)
			}
			got, err := adapter.shouldHideWindow(context.Background())
			if (err != nil) != tt.wantErr {
				t.Fatalf("shouldHideWindow() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("shouldHideWindow() = %v, want %v", got, tt.want)
			}
		})
	}
}
