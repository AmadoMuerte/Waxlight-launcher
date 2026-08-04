package filesystem

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestScanDiscoversModsAndReadsArchiveMetadata(t *testing.T) {
	root := t.TempDir()
	manager := ModFileManager{}
	if err := manager.EnsureLayout(root); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(root, modsDirectory, "smithingplus_2.4.1.zip")
	writeModArchive(t, archivePath, `{"modid":"smithingplus","name":"Smithing Plus","version":"2.4.1"}`)
	if err := os.WriteFile(filepath.Join(root, disabledModsDirectory, "utility.cs"), []byte("code"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, modsDirectory, "readme.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}

	mods, err := manager.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 2 {
		t.Fatalf("expected two discovered mods, got %#v", mods)
	}
	if mods[0].Name != "Smithing Plus" || mods[0].Version != "2.4.1" || !mods[0].Enabled {
		t.Fatalf("unexpected archive metadata: %#v", mods[0])
	}
	if mods[1].Name != "utility" || mods[1].Version != "unknown" || mods[1].Enabled {
		t.Fatalf("unexpected disabled mod: %#v", mods[1])
	}
}

func writeModArchive(t *testing.T, path, metadata string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create("modinfo.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(metadata)); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

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
