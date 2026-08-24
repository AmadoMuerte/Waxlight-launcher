package snapshots

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/snapshots"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return New(t.TempDir())
}

func writeTestManifest(t *testing.T, store *Store, instanceID, snapshotID string, manifest snapshots.Manifest) {
	t.Helper()
	dir, err := store.SnapshotDir(instanceID, snapshotID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteManifest(dir, manifest); err != nil {
		t.Fatal(err)
	}
}

func TestStoreManifestRoundTrip(t *testing.T) {
	store := newTestStore(t)
	created := time.Now().UTC().Truncate(time.Second)
	manifest := snapshots.Manifest{
		FormatVersion: snapshots.FormatVersion,
		ID:            "snap-1",
		InstanceID:    "instance-1",
		InstanceName:  "Survival",
		CreatedAt:     created,
		Type:          snapshots.TypeManual,
		GameVersion:   "1.20",
		SizeBytes:     1234,
		ModCount:      2,
		WorldCount:    1,
		Mods: []snapshots.Mod{
			{Source: snapshots.ModSourceModDB, ModID: "100", ReleaseID: "1000", Identifier: "fancy", Version: "1.0.0", Enabled: true},
			{Source: snapshots.ModSourceUnknown, Identifier: "local", Version: "2.0.0", FileName: "local.zip", Enabled: false},
		},
	}
	dir, err := store.SnapshotDir("instance-1", "snap-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(store.DataDir(dir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.DataDir(dir), "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteManifest(dir, manifest); err != nil {
		t.Fatal(err)
	}

	read, err := store.ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if read.ID != manifest.ID || read.InstanceID != manifest.InstanceID || read.Type != manifest.Type ||
		read.GameVersion != manifest.GameVersion || read.SizeBytes != manifest.SizeBytes ||
		read.ModCount != manifest.ModCount || read.WorldCount != manifest.WorldCount {
		t.Fatalf("manifest round trip lost data: %#v", read)
	}
	if !read.CreatedAt.Equal(created) {
		t.Fatalf("manifest creation time changed: %s != %s", read.CreatedAt, created)
	}
	if len(read.Mods) != 2 || read.Mods[0].Source != snapshots.ModSourceModDB || read.Mods[1].FileName != "local.zip" {
		t.Fatalf("manifest mods round trip lost data: %#v", read.Mods)
	}
}

func TestStoreListNewestFirstAndSkipsCorruptedDirectories(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()
	writeTestManifest(t, store, "instance-1", "older", snapshots.Manifest{
		FormatVersion: snapshots.FormatVersion,
		ID:            "older",
		InstanceID:    "instance-1",
		CreatedAt:     now.Add(-time.Hour),
		Type:          snapshots.TypeAutomatic,
		Reason:        snapshots.ReasonBeforeModUpdate,
		GameVersion:   "1.20",
	})
	writeTestManifest(t, store, "instance-1", "newer", snapshots.Manifest{
		FormatVersion: snapshots.FormatVersion,
		ID:            "newer",
		InstanceID:    "instance-1",
		CreatedAt:     now,
		Type:          snapshots.TypeManual,
		GameVersion:   "1.20",
	})
	// A corrupted directory and a dot-prefixed staging directory are skipped.
	if err := os.MkdirAll(filepath.Join(store.root, "instance-1", "broken"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.root, "instance-1", "broken", "manifest.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(store.root, "instance-1", ".tmp-x"), 0o700); err != nil {
		t.Fatal(err)
	}

	listed, err := store.List(context.Background(), "instance-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 readable snapshots, got %d", len(listed))
	}
	if listed[0].ID != "newer" || listed[1].ID != "older" {
		t.Fatalf("snapshots are not sorted newest first: %s, %s", listed[0].ID, listed[1].ID)
	}
}

func TestStoreListRejectsManifestWithoutMatchingDirectoryName(t *testing.T) {
	store := newTestStore(t)
	writeTestManifest(t, store, "instance-1", "snap-1", snapshots.Manifest{
		FormatVersion: snapshots.FormatVersion,
		ID:            "different-id",
		InstanceID:    "instance-1",
		CreatedAt:     time.Now().UTC(),
		Type:          snapshots.TypeManual,
		GameVersion:   "1.20",
	})
	listed, err := store.List(context.Background(), "instance-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("a mismatched manifest must be skipped, got %d snapshots", len(listed))
	}
}

func TestStoreListForUnknownInstanceReturnsEmpty(t *testing.T) {
	store := newTestStore(t)
	listed, err := store.List(context.Background(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected no snapshots, got %d", len(listed))
	}
}

func TestStoreRejectsUnsafeIdentifiers(t *testing.T) {
	store := newTestStore(t)
	for _, value := range []string{"", ".", "..", "a/b", `a\b`, "a/b/c"} {
		if _, err := store.SnapshotDir("instance", value); err == nil {
			t.Fatalf("unsafe snapshot id %q was accepted", value)
		}
		if _, err := store.InstancePath(value); err == nil {
			t.Fatalf("unsafe instance id %q was accepted", value)
		}
	}
}

func TestStoreRemove(t *testing.T) {
	store := newTestStore(t)
	writeTestManifest(t, store, "instance-1", "snap-1", snapshots.Manifest{
		FormatVersion: snapshots.FormatVersion,
		ID:            "snap-1",
		InstanceID:    "instance-1",
		CreatedAt:     time.Now().UTC(),
		Type:          snapshots.TypeManual,
		GameVersion:   "1.20",
	})
	if err := store.Remove("instance-1", "snap-1"); err != nil {
		t.Fatal(err)
	}
	if err := store.Remove("instance-1", "snap-1"); err == nil {
		t.Fatal("removing a missing snapshot must fail")
	} else {
		var appErr *errs.AppError
		if !errors.As(err, &appErr) || appErr.Code != snapshots.ErrSnapshotNotFound {
			t.Fatalf("expected SNAPSHOT_NOT_FOUND, got %v", err)
		}
	}
}

func TestStoreSizeAndDataDir(t *testing.T) {
	store := newTestStore(t)
	dir, err := store.SnapshotDir("instance-1", "snap-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(store.DataDir(dir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.DataDir(dir), "a"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	size, err := store.Size(dir)
	if err != nil {
		t.Fatal(err)
	}
	if size != 5 {
		t.Fatalf("expected 5 bytes, got %d", size)
	}
	if !strings.HasSuffix(store.DataDir(dir), string(filepath.Separator)+"data") {
		t.Fatalf("unexpected data directory %q", store.DataDir(dir))
	}
}

func TestStoreTempDirIsSiblingAndDotPrefixed(t *testing.T) {
	store := newTestStore(t)
	instanceDir, err := store.InstanceDir("instance-1")
	if err != nil {
		t.Fatal(err)
	}
	temp, err := store.TempDir("instance-1")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(temp) != instanceDir {
		t.Fatalf("staging directory %q is not a sibling of the instance directory", temp)
	}
	if !strings.HasPrefix(filepath.Base(temp), ".tmp-") {
		t.Fatalf("staging directory %q is not dot-prefixed", temp)
	}
	listed, err := store.List(context.Background(), "instance-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("staging directory must never be listed, got %d entries", len(listed))
	}
}
