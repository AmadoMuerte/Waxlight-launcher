package updater

import (
	"strings"
	"testing"
)

func TestWindowsUpdateHelperWaitsInstallsAndRestarts(t *testing.T) {
	requiredFragments := []string{
		"Wait-Process",
		"Start-Process",
		"-ArgumentList @(\"/S\")",
		"-Wait",
		"Waxlight Launcher\\waxlight.exe",
		"Shell.Application",
		"Removed legacy installation directory",
		"update.log",
	}

	for _, fragment := range requiredFragments {
		if !strings.Contains(windowsUpdateHelperScript, fragment) {
			t.Fatalf("Windows update helper is missing %q", fragment)
		}
	}
}

func TestWindowsUpdateHelperTargetsCleanInstallDirectory(t *testing.T) {
	if !strings.Contains(windowsUpdateHelperScript, `Waxlight Launcher\waxlight.exe`) {
		t.Fatal("Windows update helper does not target the clean Waxlight Launcher directory")
	}
}
