//go:build !linux && !windows

package tray

// Config configures the system tray.
type Config struct {
	Labels Labels
	OnOpen func() // restore the window
	OnQuit func() // quit the whole app
}

// Run is a no-op on unsupported platforms (macOS).
func Run(cfg Config) {}

// Stop is a no-op on unsupported platforms (macOS).
func Stop() {}
