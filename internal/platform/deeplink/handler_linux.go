//go:build linux

package deeplink

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

const desktopFileName = "com.waxlight.launcher.desktop"

// RegisterHandler makes a portable Linux install handle waxlight:// links.
func RegisterHandler() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}

	applications := filepath.Join(dataHome, "applications")
	if err := os.MkdirAll(applications, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(applications, desktopFileName), []byte(desktopEntry(executable)), 0o644); err != nil {
		return err
	}
	return exec.Command("xdg-mime", "default", desktopFileName, "x-scheme-handler/waxlight").Run()
}

func desktopEntry(executable string) string {
	return fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Waxlight Launcher
Comment=Unofficial launcher for Vintage Story
Exec=%s %%u
Terminal=false
Categories=Game;
StartupNotify=true
StartupWMClass=waxlight
MimeType=x-scheme-handler/waxlight;
`, strconv.Quote(executable))
}
