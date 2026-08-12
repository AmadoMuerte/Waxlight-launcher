// Package nativefs integrates with the host file manager.
package nativefs

import (
	"os"
	"os/exec"
	"runtime"
)

type Opener struct{}

func (Opener) OpenDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return os.ErrNotExist
	}
	var command string
	var args []string
	switch runtime.GOOS {
	case "windows":
		command, args = "explorer.exe", []string{path}
	case "darwin":
		command, args = "open", []string{path}
	default:
		command, args = "xdg-open", []string{path}
	}
	return exec.Command(command, args...).Start()
}
