//go:build linux || windows

package tray

import (
	"log/slog"
	"runtime"
	"sync"

	"fyne.io/systray"
)

// runOnce keeps the process-global systray singleton from being started twice.
var runOnce sync.Once

// Config configures the system tray.
type Config struct {
	Labels Labels
	OnOpen func() // restore the window
	OnQuit func() // quit the whole app
}

// Run starts the tray at most once per process. systray is a process-global
// singleton that cannot restart in-process: launcher restarts (update install,
// data-root relocation) are full process restarts, and test binaries exercise
// Startup/Shutdown repeatedly.
func Run(cfg Config) {
	runOnce.Do(func() {
		// Must be set before Run: Linux ItemIsMenu property is computed at DBus export time.
		systray.SetOnTapped(cfg.OnOpen)
		go func() {
			runtime.LockOSThread() // Win32: message window + GetMessage loop must share one thread
			systray.Run(onReady(cfg), nil)
		}()
	})
}

func onReady(cfg Config) func() {
	return func() {
		systray.SetIcon(iconBytes)
		systray.SetTooltip("Waxlight Launcher")
		open := systray.AddMenuItem(cfg.Labels.Open, cfg.Labels.Open)
		systray.AddSeparator()
		quit := systray.AddMenuItem(cfg.Labels.Quit, cfg.Labels.Quit)
		if cfg.OnOpen != nil {
			go func() {
				for range open.ClickedCh {
					cfg.OnOpen()
				}
			}()
		}
		if cfg.OnQuit != nil {
			go func() {
				for range quit.ClickedCh {
					cfg.OnQuit()
				}
			}()
		}
		slog.Info("tray: started")
	}
}

// Stop removes the tray icon and stops its loop; safe if Run failed or never ran.
func Stop() { systray.Quit() }
