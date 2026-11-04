//go:build windows

package gameversion

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

func registryTestBase() string {
	return fmt.Sprintf("WaxlightTest%d", time.Now().UnixNano())
}

func registryTestTarget(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "versions", "1.22.7")
}

func createRegistryString(t *testing.T, path string, values map[string]string) {
	t.Helper()
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer key.Close()
	for name, value := range values {
		if err := key.SetStringValue(name, value); err != nil {
			t.Fatalf("set %s\\%s: %v", path, name, err)
		}
	}
	t.Cleanup(func() {
		if err := deleteRegistryTree(registry.CURRENT_USER, path); err != nil {
			t.Fatalf("cleanup %s: %v", path, err)
		}
	})
}

func readRegistryString(t *testing.T, path, name string) (string, bool) {
	t.Helper()
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", false
		}
		t.Fatalf("open %s: %v", path, err)
	}
	defer key.Close()
	value, _, err := key.GetStringValue(name)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", false
		}
		t.Fatalf("read %s\\%s: %v", path, name, err)
	}
	return value, true
}

func writeInstallerState(t *testing.T, base, installLocation string) {
	t.Helper()
	createRegistryString(t, `SOFTWARE\`+base, map[string]string{
		"Inno Setup: App Path": installLocation,
		"Inno Setup: Language": "en",
		"NotAnInnoValue":       "keep me",
	})
	createRegistryString(t, innoUninstallRoot+`\`+base+innoUninstallSuffix, map[string]string{
		"InstallLocation": installLocation,
		"DisplayName":     "Vintage Story",
	})
}

func TestFindInnoAppIDMatchesInstallLocation(t *testing.T) {
	target := registryTestTarget(t)
	base := registryTestBase()
	createRegistryString(t, innoUninstallRoot+`\`+base+innoUninstallSuffix, map[string]string{
		"InstallLocation": target,
	})
	if appID := findInnoAppID(target); appID != base {
		t.Fatalf("findInnoAppID(%q) = %q, want %q", target, appID, base)
	}
}

func TestSnapshotRestoreRemovesKeysCreatedByInstaller(t *testing.T) {
	target := registryTestTarget(t)
	base := registryTestBase()
	state := snapshotInnoState(target)
	if state.appID != "" {
		t.Fatalf("pre-run appID = %q, want empty", state.appID)
	}

	writeInstallerState(t, base, target)

	if appID := findInnoAppID(target); appID != base {
		t.Fatalf("post-run appID = %q, want %q", appID, base)
	}
	if err := state.Restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if _, ok := readRegistryString(t, innoUninstallRoot+`\`+base+innoUninstallSuffix, "InstallLocation"); ok {
		t.Fatalf("uninstall entry survived restore")
	}
	if _, ok := readRegistryString(t, `SOFTWARE\`+base, "Inno Setup: App Path"); ok {
		t.Fatalf("previous directory state survived restore")
	}
}

func TestSnapshotRestoreRestoresPreviousValues(t *testing.T) {
	target := registryTestTarget(t)
	previous := `C:\Program Files\Vintagestory`
	base := registryTestBase()
	writeInstallerState(t, base, previous)

	state := snapshotInnoState(target)
	if state.appID != base {
		t.Fatalf("pre-run appID = %q, want %q", state.appID, base)
	}

	// The silent installer run overwrites the recorded directory and adds
	// values, pointing everything at the launcher's data folder.
	overwrite := innoUninstallRoot + `\` + base + innoUninstallSuffix
	createRegistryString(t, overwrite, map[string]string{
		"InstallLocation": target,
		"DisplayName":     "Vintage Story",
	})
	appState := `SOFTWARE\` + base
	createRegistryString(t, appState, map[string]string{
		"Inno Setup: App Path":       target,
		"Inno Setup: Selected Tasks": "desktopicon",
	})

	if err := state.Restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}

	if value, ok := readRegistryString(t, overwrite, "InstallLocation"); !ok || value != previous {
		t.Fatalf("InstallLocation = %q (present %v), want %q", value, ok, previous)
	}
	if value, ok := readRegistryString(t, appState, "Inno Setup: App Path"); !ok || value != previous {
		t.Fatalf("Inno Setup: App Path = %q (present %v), want %q", value, ok, previous)
	}
	if _, ok := readRegistryString(t, appState, "Inno Setup: Selected Tasks"); ok {
		t.Fatalf("value added by the installer run survived restore")
	}
	if value, ok := readRegistryString(t, appState, "NotAnInnoValue"); !ok || value != "keep me" {
		t.Fatalf("non-Inno value = %q (present %v), want kept", value, ok)
	}
}
