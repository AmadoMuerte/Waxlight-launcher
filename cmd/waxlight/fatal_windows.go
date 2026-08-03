//go:build windows

package main

import "golang.org/x/sys/windows"

const (
	messageBoxOK    = 0x00000000
	messageBoxError = 0x00000010
)

func showFatalError(message string) {
	text, textErr := windows.UTF16PtrFromString(message)
	title, titleErr := windows.UTF16PtrFromString("Waxlight Launcher")
	if textErr != nil || titleErr != nil {
		return
	}
	_, _ = windows.MessageBox(0, text, title, messageBoxOK|messageBoxError)
}
