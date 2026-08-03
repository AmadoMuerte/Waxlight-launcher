//go:build !windows

package main

import "log"

func showFatalError(message string) { log.Printf("Waxlight Launcher: %s", message) }
