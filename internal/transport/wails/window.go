package wails

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/settings"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// settingsReader is the settings access the window adapter needs to decide
// whether a window close must be turned into a tray hide. *settings.Reader
// satisfies it.
type settingsReader interface {
	Get(context.Context) (settings.Settings, error)
}

// WindowAdapter owns the window lifecycle decisions that must stay consistent
// across the tray, the post-update restart, and the data-root relocation
// restart.
type WindowAdapter struct {
	lifecycle lifecycle
	reader    settingsReader
	quitting  atomic.Bool
	ready     atomic.Bool
}

func NewWindowAdapter(lifecycle lifecycle, reader settingsReader) *WindowAdapter {
	return &WindowAdapter{lifecycle: lifecycle, reader: reader}
}

// CloseRequested is the Wails OnBeforeClose hook. It reports whether the close
// was handled by the launcher: with close-to-tray enabled the window is hidden
// instead of closed, so Wails must cancel the close. A settings read failure
// fails open (returns false) so the user is never trapped in an unclosable
// window.
func (adapter *WindowAdapter) CloseRequested(ctx context.Context) bool {
	hide, err := adapter.shouldHideWindow(ctx)
	if err != nil {
		slog.Warn("wails: could not read the close-to-tray setting; closing the window", "error", err)
		return false
	}
	if !hide {
		return false
	}
	wruntime.WindowHide(ctx)
	return true
}

// shouldHideWindow is the pure close-to-tray decision. It is separated from
// CloseRequested because the Wails runtime calls terminate the process when no
// live application context is present, which makes them untestable.
func (adapter *WindowAdapter) shouldHideWindow(ctx context.Context) (bool, error) {
	if adapter.quitting.Load() {
		return false, nil
	}
	current, err := adapter.reader.Get(ctx)
	if err != nil {
		return false, err
	}
	return current.CloseToTray, nil
}

// Show restores and focuses the window from the tray icon. It mirrors the
// second-instance restore sequence in cmd/waxlight/main.go.
func (adapter *WindowAdapter) Show() {
	ctx := adapter.lifecycle.Context()
	wruntime.WindowUnminimise(ctx)
	wruntime.Show(ctx)
}

// QuitFromTray quits the launcher from the tray menu.
func (adapter *WindowAdapter) QuitFromTray() {
	adapter.Quit(adapter.lifecycle.Context())
}

// Quit marks the launcher as quitting before asking Wails to quit. The flag
// stops CloseRequested from hiding the window: runtime.Quit invokes
// OnBeforeClose and is canceled when it returns true.
func (adapter *WindowAdapter) Quit(ctx context.Context) {
	adapter.quitting.Store(true)
	wruntime.Quit(ctx)
}

// MarkReady records that the lifecycle context carries the Wails application
// values. Before that point wruntime.Quit would abort the process with
// log.Fatalf, so ForceQuit skips the runtime call and relies on Wails' own
// signal handler, which the quitting flag lets through OnBeforeClose.
func (adapter *WindowAdapter) MarkReady() {
	adapter.ready.Store(true)
}

// ForceQuit turns a termination signal into a real quit: a termination signal
// must exit the process even when close-to-tray would turn a window close
// into a hide. Wails' own signal handler also quits through OnBeforeClose, so
// whichever handler runs last finishes the exit once quitting is set.
func (adapter *WindowAdapter) ForceQuit() {
	adapter.quitting.Store(true)
	if adapter.ready.Load() {
		wruntime.Quit(adapter.lifecycle.Context())
	}
}
