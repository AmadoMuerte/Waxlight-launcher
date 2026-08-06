//go:build !windows

package main

import "log/slog"

func showFatalError(message string) { slog.Error("Waxlight Launcher: " + message) }
