//go:build !windows

package discord

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestPipePathsIncludeSandboxLocationsAndAllSlots(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("TMPDIR", "")
	tmpDir := t.TempDir()
	tempDir := t.TempDir()
	t.Setenv("TMP", tmpDir)
	t.Setenv("TEMP", tempDir)
	paths := pipePaths()

	for _, path := range []string{
		filepath.Join(runtimeDir, "discord-ipc-0"),
		filepath.Join(runtimeDir, "app", "com.discordapp.Discord", "discord-ipc-9"),
		filepath.Join(runtimeDir, "snap.discord", "discord-ipc-9"),
		filepath.Join(tmpDir, "discord-ipc-0"),
		filepath.Join(tempDir, "discord-ipc-9"),
	} {
		if !slices.Contains(paths, path) {
			t.Fatalf("pipePaths() does not contain %q", path)
		}
	}
}
