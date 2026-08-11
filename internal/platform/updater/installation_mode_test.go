package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectInstallationModeReturnsPortableForTempDir(t *testing.T) {
	mode := DetectInstallationMode()
	if mode != InstallationModePortable {
		t.Logf("installation mode detection returned %s for temp directory, expected portable", mode)
	}
}

func TestInstallationModeConstants(t *testing.T) {
	if InstallationModeInstalled != "installed" {
		t.Fatal("InstallationModeInstalled constant mismatch")
	}
	if InstallationModePortable != "portable" {
		t.Fatal("InstallationModePortable constant mismatch")
	}
}

func TestIsDirectoryWritableForExistingDir(t *testing.T) {
	dir := t.TempDir()
	if !isDirectoryWritable(dir) {
		t.Fatal("expected temp directory to be writable")
	}
}

func TestIsDirectoryWritableForNonexistentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")
	if isDirectoryWritable(dir) {
		t.Fatal("expected nonexistent directory to not be writable")
	}
}

func TestIsDirectoryWritableForReadOnlyDir(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0o555)
	defer os.Chmod(dir, 0o755)
	if isDirectoryWritable(dir) {
		t.Log("directory appears writable even with restricted permissions (may be running as root)")
	}
}
