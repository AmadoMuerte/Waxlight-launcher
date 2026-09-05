package presence

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// AppID is the Discord Application ID registered for Waxlight Launcher.
const AppID = "1545719884716183582"

// Activity values shown in the Discord client. The app name "Waxlight" is
// already shown by Discord, so the details field is intentionally left empty.
const (
	idleActivityState      = "Idle"
	playingActivityState   = "Playing Vintage Story"
	activityLargeImageKey  = "waxlight_launcher"
	activityLargeImageText = "Waxlight Launcher"
)

// Service is a best-effort Discord Rich Presence service. Discord availability
// errors and settings failures only produce debug log lines and never fail
// launcher operations. All methods are safe for concurrent use.
type Service struct {
	client         Client
	settingsReader SettingsReader
	dialer         Dialer
	appID          string
	mu             sync.Mutex
	connected      bool
	connecting     bool
	disabled       bool
	closed         bool
}

// NewService builds a presence service. It does not connect until requested.
func NewService(settingsReader SettingsReader, dialer Dialer) *Service {
	return &Service{
		appID:          AppID,
		settingsReader: settingsReader,
		dialer:         dialer,
	}
}

// Connect starts a Discord connection in the background when Rich Presence is
// enabled. A failed settings read or unavailable Discord leaves it disconnected.
func (s *Service) Connect(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connectLocked(ctx)
}

// connectLocked reads settings and starts a single background dial. Callers
// must hold s.mu.
func (s *Service) connectLocked(ctx context.Context) {
	if s.connected || s.connecting || s.disabled || s.closed {
		return
	}
	value, err := s.settingsReader.Get(ctx)
	if err != nil {
		slog.Debug("presence: could not read settings", "error", err)
		return
	}
	if !value.RichPresenceEnabled {
		slog.Debug("presence: rich presence disabled in settings")
		return
	}
	if s.dialer == nil {
		return
	}
	s.connecting = true
	go func() {
		client := s.dialer(s.appID)
		s.mu.Lock()
		s.connecting = false
		if s.disabled || s.closed {
			s.mu.Unlock()
			if client != nil {
				client.Close()
			}
			return
		}
		if client != nil {
			s.client = client
			s.connected = true
			slog.Debug("presence: connected to Discord")
			s.setIdleActivityLocked()
		} else {
			slog.Debug("presence: Discord unavailable, staying disconnected")
		}
		s.mu.Unlock()
	}()
}

// GameStarted publishes the playing activity on the live connection.
func (s *Service) GameStarted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.connected && !s.client.Connected() {
		s.connected = false
	}
	if s.closed || !s.connected {
		return
	}
	s.setPlayingActivityLocked()
}

// GameStopped returns the live connection to the idle activity.
func (s *Service) GameStopped() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.connected && !s.client.Connected() {
		s.connected = false
	}
	if s.closed || !s.connected {
		return
	}
	s.setIdleActivityLocked()
}

// SetEnabled connects when enabling and clears and disconnects when disabling.
func (s *Service) SetEnabled(ctx context.Context, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if enabled {
		s.disabled = false
		s.connectLocked(ctx)
		return
	}
	s.disabled = true
	if s.connected {
		_ = s.client.ClearActivity()
	}
	s.closeLocked()
	slog.Debug("presence: disabled")
}

// Close permanently closes the service and its Discord connection.
func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.closeLocked()
}

// closeLocked closes the Discord connection. Callers must hold s.mu.
func (s *Service) closeLocked() {
	if s.client != nil {
		s.client.Close()
	}
	s.client = nil
	s.connected = false
}

// setIdleActivityLocked publishes the idle activity. Callers must hold s.mu
// with a live connection.
func (s *Service) setIdleActivityLocked() {
	_ = s.client.SetActivity(Activity{
		State:          idleActivityState,
		LargeImageKey:  activityLargeImageKey,
		LargeImageText: activityLargeImageText,
	})
	slog.Debug("presence: idle activity set")
}

// setPlayingActivityLocked publishes the playing activity with a start time.
// Callers must hold s.mu with a live connection.
func (s *Service) setPlayingActivityLocked() {
	start := time.Now().Unix()
	_ = s.client.SetActivity(Activity{
		State:          playingActivityState,
		LargeImageKey:  activityLargeImageKey,
		LargeImageText: activityLargeImageText,
		StartTimestamp: &start,
	})
	slog.Debug("presence: playing activity set")
}
