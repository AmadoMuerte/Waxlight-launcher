package instancedirectory_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/platform/filesystem"
	"github.com/waxlight/waxlight-launcher/internal/platform/instancedirectory"
)

func TestCloneStorageCopiesSafeFilesAndPreservesTargetMarker(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	for _, directory := range []string{
		filepath.Join(source, "Config"), filepath.Join(source, "SaveGame"), filepath.Join(source, "Logs"), target,
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(source, ".waxlight-instance"):                     "source-id",
		filepath.Join(source, "Config", "mod.json"):                     "config",
		filepath.Join(source, "SaveGame", "world.db"):                   "save",
		filepath.Join(source, "Logs", "main.log"):                       "log",
		filepath.Join(source, "clientsettings.json"):                    `{"stringSettings":{"sessionkey":"SECRET","useremail":"private@example.com","fov":80}}`,
		filepath.Join(source, "clientsettings.waxlight-auth-injection"): "secret",
		filepath.Join(target, ".waxlight-instance"):                     "clone-id",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := instancedirectory.NewCloneStorage(filesystem.SanitizeClientSettings).Copy(context.Background(), source, target); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(target, "Config", "mod.json"))
	if err != nil || string(content) != "config" {
		t.Fatalf("copied config = %q, %v", content, err)
	}
	marker, err := os.ReadFile(filepath.Join(target, ".waxlight-instance"))
	if err != nil || string(marker) != "clone-id" {
		t.Fatalf("target marker = %q, %v", marker, err)
	}
	settings, err := os.ReadFile(filepath.Join(target, "clientsettings.json"))
	if err != nil || strings.Contains(string(settings), "SECRET") || strings.Contains(string(settings), "private@example.com") || !strings.Contains(string(settings), "fov") {
		t.Fatalf("sanitized client settings = %q, %v", settings, err)
	}
	for _, path := range []string{
		filepath.Join(target, "SaveGame", "world.db"),
		filepath.Join(target, "Logs", "main.log"),
		filepath.Join(target, "clientsettings.waxlight-auth-injection"),
	} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("excluded path %q exists: %v", path, err)
		}
	}
}

func TestCloneStorageRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "link")); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink unavailable: %v", err)
		}
		t.Fatal(err)
	}

	if err := (instancedirectory.CloneStorage{}).Copy(context.Background(), source, target); err == nil {
		t.Fatal("Copy() error = nil")
	}
	if _, err := os.Lstat(filepath.Join(target, "link")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("symlink target was copied: %v", err)
	}
}

func TestCloneStorageHonorsCancellation(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := (instancedirectory.CloneStorage{}).Copy(ctx, source, target); !errors.Is(err, context.Canceled) {
		t.Fatalf("Copy() error = %v, want %v", err, context.Canceled)
	}
}

func TestCloneStorageRejectsOverlappingRoots(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	storage := instancedirectory.CloneStorage{}
	for _, target := range []string{source, filepath.Join(source, "nested"), root} {
		if err := storage.Copy(context.Background(), source, target); err == nil {
			t.Fatalf("Copy(%q, %q) error = nil", source, target)
		}
	}
}

func TestCloneStorageMapsOnlyCopiedInternalPaths(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(filepath.Join(target, "covers"), 0o755); err != nil {
		t.Fatal(err)
	}
	copiedCover := filepath.Join(target, "covers", "cover.png")
	if err := os.WriteFile(copiedCover, []byte("cover"), 0o600); err != nil {
		t.Fatal(err)
	}
	storage := instancedirectory.CloneStorage{}

	if mapped, ok := storage.CopiedPath(source, target, filepath.Join(source, "covers", "cover.png")); !ok || mapped != copiedCover {
		t.Fatalf("CopiedPath() = %q, %v", mapped, ok)
	}
	for _, path := range []string{filepath.Join(root, "outside.png"), filepath.Join(source, "missing.png")} {
		if mapped, ok := storage.CopiedPath(source, target, path); ok || mapped != "" {
			t.Fatalf("CopiedPath(%q) = %q, %v", path, mapped, ok)
		}
	}
}
