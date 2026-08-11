package versionfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVersionPathUnsafeIDsDoNotCollide(t *testing.T) {
	filesystem := New(t.TempDir())
	left := filesystem.VersionPath("a/b")
	right := filesystem.VersionPath("a?b")
	if left == right {
		t.Fatalf("unsafe IDs collided at %q", left)
	}
	if filepath.Dir(left) != filepath.Join(filesystem.root, "versions") || filepath.Dir(right) != filepath.Join(filesystem.root, "versions") {
		t.Fatalf("encoded path escaped versions root: %q, %q", left, right)
	}
	if got := filepath.Base(filesystem.VersionPath("1.22.0-rc.1")); got != "1.22.0-rc.1" {
		t.Fatalf("ordinary ID changed to %q", got)
	}
}

func TestRemoveVersionRequiresExactMarkerID(t *testing.T) {
	filesystem := New(t.TempDir())
	directory := filesystem.VersionPath("a/b")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := filesystem.WriteMarker(directory, "a?b"); err != nil {
		t.Fatal(err)
	}
	if err := filesystem.RemoveVersion(directory, "a/b"); err == nil {
		t.Fatal("version with mismatched ownership marker was removed")
	}
	if _, err := os.Stat(directory); err != nil {
		t.Fatalf("mismatched directory did not survive: %v", err)
	}
	if err := filesystem.WriteMarker(directory, "a/b"); err != nil {
		t.Fatal(err)
	}
	if err := filesystem.RemoveVersion(directory, "a/b"); err != nil {
		t.Fatal(err)
	}
}
