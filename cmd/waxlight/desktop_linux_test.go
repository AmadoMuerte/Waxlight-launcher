//go:build linux

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// 1x1 transparent PNG.
var testPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

func setupPortableInstall(t *testing.T) (appDir, dataHome string) {
	t.Helper()
	root := t.TempDir()
	appDir = filepath.Join(root, "app")
	dataHome = filepath.Join(root, "data")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, portableDesktopFile), []byte("[Desktop Entry]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, portableIconFile), testPNG, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", dataHome)
	return appDir, dataHome
}

func TestInstallDesktopIntegrationPortable(t *testing.T) {
	appDir, dataHome := setupPortableInstall(t)
	executable := filepath.Join(appDir, "waxlight")

	installDesktopIntegration(executable)

	entry, err := os.ReadFile(filepath.Join(dataHome, "applications", portableDesktopFile))
	if err != nil {
		t.Fatalf("desktop entry not installed: %v", err)
	}
	for _, want := range []string{
		"Exec=" + executable,
		"StartupWMClass=waxlight",
		"Icon=com.waxlight.launcher",
	} {
		if !bytes.Contains(entry, []byte(want)) {
			t.Errorf("desktop entry missing %q:\n%s", want, entry)
		}
	}

	icon, err := os.ReadFile(filepath.Join(dataHome, "icons", "hicolor", "1x1", "apps", portableIconFile))
	if err != nil {
		t.Fatalf("icon not installed: %v", err)
	}
	if !bytes.Equal(icon, testPNG) {
		t.Error("installed icon differs from portable icon")
	}
}

func TestInstallDesktopIntegrationIsIdempotent(t *testing.T) {
	appDir, dataHome := setupPortableInstall(t)
	executable := filepath.Join(appDir, "waxlight")
	entryPath := filepath.Join(dataHome, "applications", portableDesktopFile)

	installDesktopIntegration(executable)
	first, err := os.Stat(entryPath)
	if err != nil {
		t.Fatal(err)
	}

	installDesktopIntegration(executable)
	second, err := os.Stat(entryPath)
	if err != nil {
		t.Fatal(err)
	}
	if !first.ModTime().Equal(second.ModTime()) {
		t.Error("second run rewrote the desktop entry")
	}
}

func TestInstallDesktopIntegrationRefreshesMovedInstall(t *testing.T) {
	appDir, dataHome := setupPortableInstall(t)
	executable := filepath.Join(appDir, "waxlight")
	entryPath := filepath.Join(dataHome, "applications", portableDesktopFile)

	installDesktopIntegration(executable)
	moved := filepath.Join(appDir, "moved", "waxlight")
	if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(moved), portableDesktopFile), []byte("[Desktop Entry]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(moved), portableIconFile), testPNG, 0o644); err != nil {
		t.Fatal(err)
	}

	installDesktopIntegration(moved)

	entry, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(entry, []byte("Exec="+moved)) {
		t.Errorf("desktop entry not updated after move:\n%s", entry)
	}
}

func TestInstallDesktopIntegrationSkipsPackagedInstall(t *testing.T) {
	root := t.TempDir()
	dataHome := filepath.Join(root, "data")
	t.Setenv("XDG_DATA_HOME", dataHome)

	installDesktopIntegration(filepath.Join(root, "waxlight"))

	if _, err := os.Stat(filepath.Join(dataHome, "applications")); !os.IsNotExist(err) {
		t.Error("desktop integration ran for a non-portable install")
	}
}

func TestInstallDesktopIntegrationHandlesBrokenIcon(t *testing.T) {
	appDir, dataHome := setupPortableInstall(t)
	if err := os.WriteFile(filepath.Join(appDir, portableIconFile), []byte("not a png"), 0o644); err != nil {
		t.Fatal(err)
	}

	installDesktopIntegration(filepath.Join(appDir, "waxlight"))

	if _, err := os.Stat(filepath.Join(dataHome, "applications", portableDesktopFile)); err != nil {
		t.Error("desktop entry should install even when the icon is broken")
	}
	if _, err := os.Stat(filepath.Join(dataHome, "icons")); !os.IsNotExist(err) {
		t.Error("broken icon must not be installed")
	}
}
