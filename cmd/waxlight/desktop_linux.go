//go:build linux

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

// Wayland compositors ignore window icons set via GTK; they resolve the icon
// from a desktop entry whose StartupWMClass matches the GTK app id (the
// binary's program name, "waxlight"). The portable tarball ships the entry and
// its icon next to the executable, so install them into the user's XDG data
// directory. deb/rpm installs already have these files system-wide and are
// detected by the missing portable desktop file.

const (
	portableDesktopFile = "com.waxlight.launcher.desktop"
	portableIconFile    = "com.waxlight.launcher.png"
)

func installDesktopIntegration(executable string) {
	appDir := filepath.Dir(executable)
	portableDesktopPath := filepath.Join(appDir, portableDesktopFile)
	if _, err := os.Stat(portableDesktopPath); err != nil {
		return
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		dataHome = filepath.Join(home, ".local", "share")
	}

	entry := portableDesktopEntry(executable)
	entryPath := filepath.Join(dataHome, "applications", portableDesktopFile)
	if err := writeFileIfChanged(entryPath, entry, 0o644); err != nil {
		slog.Warn("desktop integration: install desktop entry", "path", entryPath, "error", err)
	} else if err := installPortableIcon(appDir, dataHome); err != nil {
		slog.Warn("desktop integration: install icon", "error", err)
	}
}

func portableDesktopEntry(executable string) []byte {
	return []byte(fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Waxlight Launcher
Comment=Unofficial launcher for Vintage Story
Exec=%s
Icon=com.waxlight.launcher
Terminal=false
Categories=Game;
StartupNotify=true
StartupWMClass=waxlight
`, executable))
}

func installPortableIcon(appDir, dataHome string) error {
	icon, err := os.ReadFile(filepath.Join(appDir, portableIconFile))
	if err != nil {
		return fmt.Errorf("read portable icon: %w", err)
	}
	width, height, err := pngDimensions(icon)
	if err != nil {
		return err
	}

	hicolorDir := filepath.Join(dataHome, "icons", "hicolor")
	iconPath := filepath.Join(hicolorDir, fmt.Sprintf("%dx%d/apps", width, height), portableIconFile)
	if err := writeFileIfChanged(iconPath, icon, 0o644); err != nil {
		return err
	}

	ensureIconThemeIndex(hicolorDir)
	updateIconCache(hicolorDir)
	return nil
}

func pngDimensions(png []byte) (int, int, error) {
	if len(png) < 24 ||
		!bytes.Equal(png[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) ||
		!bytes.Equal(png[12:16], []byte("IHDR")) {
		return 0, 0, fmt.Errorf("portable icon is not a valid PNG")
	}
	width := int(binary.BigEndian.Uint32(png[16:20]))
	height := int(binary.BigEndian.Uint32(png[20:24]))
	if width <= 0 || height <= 0 || width != height {
		return 0, 0, fmt.Errorf("portable icon has unsupported dimensions: %dx%d", width, height)
	}
	return width, height, nil
}

func ensureIconThemeIndex(hicolorDir string) {
	indexPath := filepath.Join(hicolorDir, "index.theme")
	if _, err := os.Stat(indexPath); err == nil {
		return
	}
	systemIndex, err := os.ReadFile("/usr/share/icons/hicolor/index.theme")
	if err != nil {
		return
	}
	_ = writeFileIfChanged(indexPath, systemIndex, 0o644)
}

func updateIconCache(hicolorDir string) {
	if _, err := exec.LookPath("gtk-update-icon-cache"); err != nil {
		return
	}
	command := exec.Command("gtk-update-icon-cache", "-f", "-t", hicolorDir)
	if output, err := command.CombinedOutput(); err != nil {
		slog.Debug("desktop integration: gtk-update-icon-cache", "output", string(output), "error", err)
	}
}

func writeFileIfChanged(path string, content []byte, mode os.FileMode) error {
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, content) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, mode)
}
