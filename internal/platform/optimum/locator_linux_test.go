//go:build linux

package optimum

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	optimumfeature "github.com/AmadoMuerte/Waxlight-launcher/internal/optimum"
)

func TestInspectLinuxInstallation(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"run.sh", "Optimum"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(directory, ".optimum", "donors"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(directory, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "assets", "version-1.22.5.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	installation, err := (Locator{}).Inspect(directory)
	if err != nil {
		t.Fatal(err)
	}
	if installation.Executable != filepath.Join(directory, "run.sh") || installation.WorkingDirectory != directory {
		t.Fatalf("unexpected launch target: %#v", installation)
	}
	if installation.GameVersion != "1.22.5" {
		t.Fatalf("game version = %q", installation.GameVersion)
	}
}

func TestDesktopInstallPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "optimum.desktop")
	if err := os.WriteFile(path, []byte("[Desktop Entry]\nExec=\"/games/Optimum Client/optimum-launch.sh\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := desktopInstallPath(path); got != "/games/Optimum Client" {
		t.Fatalf("desktop install path = %q", got)
	}
}

func TestResolveRejectsNestedVanillaVersionMismatch(t *testing.T) {
	optimumDirectory := writeLinuxInstallation(t, "1.22.5")
	vanillaDirectory := filepath.Join(t.TempDir(), "versions", "1.22.6", "vintagestory")
	if err := os.MkdirAll(filepath.Join(vanillaDirectory, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vanillaDirectory, "assets", "version-1.22.6.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := optimumfeature.NewService(Locator{}).Resolve(optimumDirectory, vanillaDirectory)
	if err == nil || !strings.Contains(err.Error(), "1.22.5") || !strings.Contains(err.Error(), "1.22.6") {
		t.Fatalf("version mismatch error = %v", err)
	}
}

func writeLinuxInstallation(t *testing.T, version string) string {
	t.Helper()
	directory := t.TempDir()
	for _, name := range []string{"run.sh", "Optimum"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(directory, ".optimum", "donors"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(directory, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "assets", "version-"+version+".txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return directory
}
