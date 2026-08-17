//go:build linux

package deeplink

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const desktopFileName = "com.waxlight.launcher-url.desktop"

// RegisterHandler makes a portable Linux install handle waxlight:// links.
//
// The URL handler intentionally uses its own hidden desktop entry instead of
// com.waxlight.launcher.desktop. The latter may be owned by a package manager
// or customised by the user and must never be rewritten at application start.
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

	handlerPath := filepath.Join(applications, desktopFileName)
	if err := writeManagedDesktopFile(handlerPath, desktopEntry(executable)); err != nil {
		return err
	}

	return exec.Command("xdg-mime", "default", desktopFileName, "x-scheme-handler/waxlight").Run()
}

// writeManagedDesktopFile creates or refreshes Waxlight's private URL handler.
// If a user has replaced that file with an unmanaged entry, leave it untouched.
func writeManagedDesktopFile(path, contents string) error {
	existing, err := os.ReadFile(path)
	if err == nil {
		if string(existing) == contents {
			return nil
		}
		if !strings.Contains(string(existing), "X-Waxlight-Managed=true") {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return os.WriteFile(path, []byte(contents), 0o644)
}

func desktopEntry(executable string) string {
	return fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Waxlight Launcher URL Handler
Exec=%s %%u
Terminal=false
NoDisplay=true
MimeType=x-scheme-handler/waxlight;
X-Waxlight-Managed=true
`, strconv.Quote(executable))
}
