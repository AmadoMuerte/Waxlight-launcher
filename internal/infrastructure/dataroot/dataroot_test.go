package dataroot

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentDefaultsToHome(t *testing.T) {
	manager := NewWithHome(t.TempDir())
	current, err := manager.Current()
	if err != nil {
		t.Fatal(err)
	}
	if current != manager.Home() {
		t.Fatalf("default data root = %q, want home %q", current, manager.Home())
	}
}

func TestCurrentUsesPointer(t *testing.T) {
	manager := NewWithHome(t.TempDir())
	target := filepath.Join(t.TempDir(), "data")
	if err := manager.writeMarker(pointerFile, target); err != nil {
		t.Fatal(err)
	}
	current, err := manager.Current()
	if err != nil {
		t.Fatal(err)
	}
	if current != target {
		t.Fatalf("data root = %q, want %q", current, target)
	}
}

func TestValidateTarget(t *testing.T) {
	current := t.TempDir()

	if err := ValidateTarget(current, ""); err == nil {
		t.Fatal("expected error for empty target")
	}
	if err := ValidateTarget(current, current); err == nil {
		t.Fatal("expected error for identical target")
	}
	if err := ValidateTarget(current, filepath.Join(current, "nested")); err == nil {
		t.Fatal("expected error for nested target")
	}
	parent := filepath.Dir(current)
	if err := ValidateTarget(current, parent); err == nil {
		t.Fatal("expected error for target containing current root")
	}

	missing := filepath.Join(t.TempDir(), "missing")
	if err := ValidateTarget(current, missing); err != nil {
		t.Fatalf("expected missing target to be valid: %v", err)
	}

	empty := t.TempDir()
	if err := ValidateTarget(current, empty); err != nil {
		t.Fatalf("expected empty target to be valid: %v", err)
	}

	nonEmpty := t.TempDir()
	writeFile(t, filepath.Join(nonEmpty, "file.txt"), "x")
	if err := ValidateTarget(current, nonEmpty); err == nil {
		t.Fatal("expected error for non-empty target")
	}
}

func TestCopyDataExcludesReservedFiles(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	writeFile(t, filepath.Join(src, "waxlight.db"), "db")
	writeFile(t, filepath.Join(src, databaseWAL), "wal")
	writeFile(t, filepath.Join(src, pointerFile), "pointer")
	writeFile(t, filepath.Join(src, "versions", "v1", "game.bin"), "game")
	writeFile(t, filepath.Join(src, "instances", "i1", "mods", "mod.jar"), "jar")
	writeFile(t, filepath.Join(src, "instances", "i1", "waxlight.db"), "instance-db")

	total, err := TotalSize(src)
	if err != nil {
		t.Fatal(err)
	}
	if total != int64(len("game")+len("jar")+len("instance-db")) {
		t.Fatalf("TotalSize = %d, want only non-reserved bytes", total)
	}

	if err := CopyData(src, dst, nil); err != nil {
		t.Fatal(err)
	}

	for _, shouldExist := range []string{
		filepath.Join(dst, "versions", "v1", "game.bin"),
		filepath.Join(dst, "instances", "i1", "mods", "mod.jar"),
		filepath.Join(dst, "instances", "i1", "waxlight.db"),
	} {
		if _, err := os.Stat(shouldExist); err != nil {
			t.Fatalf("copied file missing %q: %v", shouldExist, err)
		}
	}
	for _, shouldNotExist := range []string{
		filepath.Join(dst, "waxlight.db"),
		filepath.Join(dst, databaseWAL),
		filepath.Join(dst, pointerFile),
	} {
		if _, err := os.Stat(shouldNotExist); !os.IsNotExist(err) {
			t.Fatalf("reserved file was copied: %q", shouldNotExist)
		}
	}
}

func TestPrepareStartupFinalizesCompletedCopy(t *testing.T) {
	manager := NewWithHome(t.TempDir())
	from := manager.Home()
	target := filepath.Join(t.TempDir(), "data")

	writeFile(t, filepath.Join(from, "versions", "v1", "game.bin"), "game")
	writeFile(t, filepath.Join(from, "waxlight.db"), "database-bytes")

	if err := CopyData(from, target, nil); err != nil {
		t.Fatal(err)
	}
	if err := manager.writePending(Marker{From: from, To: target, Phase: PhaseFinalize}); err != nil {
		t.Fatal(err)
	}

	root, err := manager.PrepareStartup()
	if err != nil {
		t.Fatal(err)
	}
	if root != target {
		t.Fatalf("PrepareStartup returned %q, want %q", root, target)
	}
	if data, err := os.ReadFile(filepath.Join(target, "waxlight.db")); err != nil || string(data) != "database-bytes" {
		t.Fatalf("database not handed over: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(target, "versions", "v1", "game.bin")); err != nil {
		t.Fatalf("copied data missing at target: %v", err)
	}
	if marker, err := manager.Pending(); err != nil || marker != nil {
		t.Fatalf("pending marker should be cleared: %v, %v", marker, err)
	}
	if current, err := manager.Current(); err != nil || current != target {
		t.Fatalf("pointer should point at target: %q, %v", current, err)
	}
}

func TestPrepareStartupDiscardsInterruptedCopy(t *testing.T) {
	manager := NewWithHome(t.TempDir())
	target := filepath.Join(t.TempDir(), "partial")
	writeFile(t, filepath.Join(target, "half-file.bin"), "partial")
	if err := manager.writePending(Marker{
		From:  manager.Home(),
		To:    target,
		Phase: PhaseCopy,
	}); err != nil {
		t.Fatal(err)
	}

	root, err := manager.PrepareStartup()
	if err != nil {
		t.Fatal(err)
	}
	if root != manager.Home() {
		t.Fatalf("interrupted copy should keep old root, got %q", root)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("partial target should be removed: %v", err)
	}
	if marker, err := manager.Pending(); err != nil || marker != nil {
		t.Fatalf("pending marker should be cleared: %v, %v", marker, err)
	}
}

func TestPrepareStartupFailedFinalizeKeepsOldRoot(t *testing.T) {
	manager := NewWithHome(t.TempDir())
	missingFrom := filepath.Join(t.TempDir(), "missing-from")
	target := filepath.Join(t.TempDir(), "data")
	if err := manager.writePending(Marker{From: missingFrom, To: target, Phase: PhaseFinalize}); err != nil {
		t.Fatal(err)
	}

	root, err := manager.PrepareStartup()
	if err != nil {
		t.Fatal(err)
	}
	if root != manager.Home() {
		t.Fatalf("failed finalize should keep old root, got %q", root)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("failed target should be removed: %v", err)
	}
	if message, err := manager.ReadError(); err != nil || message == "" {
		t.Fatalf("expected recorded error, got %q, %v", message, err)
	}
}

func TestFinalizePreviousRewritesAndRemovesOld(t *testing.T) {
	home := t.TempDir()
	manager := NewWithHome(home)
	oldRoot := filepath.Join(t.TempDir(), "old")
	newRoot := filepath.Join(t.TempDir(), "new")

	for _, root := range []string{oldRoot, newRoot} {
		writeFile(t, filepath.Join(root, "versions", "v1", "game.bin"), "game")
	}
	if err := manager.writeMarker(pointerFile, newRoot); err != nil {
		t.Fatal(err)
	}
	if err := manager.writeMarker(previousFile, oldRoot); err != nil {
		t.Fatal(err)
	}

	var rewrittenOld, rewrittenNew string
	err := manager.FinalizePrevious(func(old, current string) error {
		rewrittenOld, rewrittenNew = old, current
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if rewrittenOld != oldRoot || rewrittenNew != newRoot {
		t.Fatalf("relocate callback = (%q, %q), want (%q, %q)", rewrittenOld, rewrittenNew, oldRoot, newRoot)
	}
	if _, err := os.Stat(oldRoot); !os.IsNotExist(err) {
		t.Fatalf("old data root should be removed: %v", err)
	}
	if previous, err := manager.readMarkerFile(previousFile); err != nil || previous != "" {
		t.Fatalf("previous marker should be cleared: %q, %v", previous, err)
	}
}

func TestFinalizePreviousKeepsHomeDirectory(t *testing.T) {
	manager := NewWithHome(t.TempDir())
	home := manager.Home()
	newRoot := filepath.Join(t.TempDir(), "new")
	writeFile(t, filepath.Join(home, "versions", "v1", "game.bin"), "game")
	writeFile(t, filepath.Join(home, "waxlight.db"), "db")
	if err := manager.writeMarker(pointerFile, newRoot); err != nil {
		t.Fatal(err)
	}
	if err := manager.writeMarker(previousFile, home); err != nil {
		t.Fatal(err)
	}

	if err := manager.FinalizePrevious(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatalf("home directory must survive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "versions")); !os.IsNotExist(err) {
		t.Fatalf("home data subdirectories should be removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, pointerFile)); err != nil {
		t.Fatalf("pointer file should survive: %v", err)
	}
	if current, err := manager.Current(); err != nil || current != newRoot {
		t.Fatalf("pointer should keep pointing at new root: %q, %v", current, err)
	}
}
