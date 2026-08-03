package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateDirectoryDoesNotRemoveSameDirectory(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "Mods")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	modPath := filepath.Join(directory, "example.zip")
	if err := os.WriteFile(modPath, []byte("mod"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := migrateDirectory(directory, directory); err != nil {
		t.Fatalf("same-directory migration failed: %v", err)
	}
	if contents, err := os.ReadFile(modPath); err != nil || string(contents) != "mod" {
		t.Fatalf("migration changed the existing mod: %q, %v", contents, err)
	}
}

func TestMigrateDirectoryMergesDistinctDirectories(t *testing.T) {
	root := t.TempDir()
	oldDirectory := filepath.Join(root, "mods")
	newDirectory := filepath.Join(root, "Mods")
	if err := os.MkdirAll(oldDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	oldInfo, err := os.Stat(oldDirectory)
	if err != nil {
		t.Fatal(err)
	}
	newInfo, err := os.Stat(newDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(oldInfo, newInfo) {
		t.Skip("filesystem is case-insensitive")
	}
	if err := os.WriteFile(filepath.Join(oldDirectory, "legacy.zip"), []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDirectory, "current.zip"), []byte("current"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := migrateDirectory(oldDirectory, newDirectory); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldDirectory); !os.IsNotExist(err) {
		t.Fatalf("legacy directory still exists: %v", err)
	}
	for _, name := range []string{"legacy.zip", "current.zip"} {
		if _, err := os.Stat(filepath.Join(newDirectory, name)); err != nil {
			t.Fatalf("expected %s after migration: %v", name, err)
		}
	}
}
