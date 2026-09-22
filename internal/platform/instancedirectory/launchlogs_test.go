package instancedirectory_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/instancedirectory"
)

func TestHardenLogsDoesNotCreateMissingInstanceRoot(t *testing.T) {
	instanceRoot := filepath.Join(t.TempDir(), "missing")
	if err := instancedirectory.HardenLogs(filepath.Join(instanceRoot, "Logs")); err == nil {
		t.Fatal("expected missing instance root error")
	}
	if _, err := os.Lstat(instanceRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing instance root was created: %v", err)
	}
}

func TestHardenLogsRejectsSymlinkInstanceRoot(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	instanceRoot := filepath.Join(root, "instance")
	if err := os.Symlink(target, instanceRoot); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlinks unavailable: %v", err)
		}
		t.Fatal(err)
	}
	if err := instancedirectory.HardenLogs(filepath.Join(instanceRoot, "Logs")); err == nil {
		t.Fatal("expected symlink instance root error")
	}
	if _, err := os.Lstat(filepath.Join(target, "Logs")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("logs directory was created through symlink root: %v", err)
	}
}
