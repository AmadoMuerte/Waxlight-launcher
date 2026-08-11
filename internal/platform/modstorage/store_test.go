package modstorage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/mods"
)

func TestStorePersistsMetadataAndRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	store := New(root)
	filePath, err := store.FilePath("51", "7", "playercorpse.zip")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(filePath) != ".zip" {
		t.Fatalf("unexpected file path %s", filePath)
	}
	value := mods.DownloadedMod{
		SchemaVersion: 1, ModID: "51", VersionID: "7", Name: "Player Corpse",
		DownloadedVersion: "2.0.0", FileName: "playercorpse.zip",
		FilePath: filePath, DownloadedAt: time.Now().UTC(),
	}
	if err := store.Save(context.Background(), value); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(filepath.Dir(filePath), metadataFile))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("metadata permissions are %o", info.Mode().Perm())
	}
	items, err := store.List(context.Background())
	if err != nil || len(items) != 1 || items[0].ModID != "51" {
		t.Fatalf("unexpected list: %#v, %v", items, err)
	}
	if _, err := store.FilePath("../escape", "7", "mod.zip"); err == nil {
		t.Fatal("path traversal was accepted")
	}
	if _, err := store.FilePath("51", "7", "not-a-mod.exe"); err == nil {
		t.Fatal("unsafe extension was accepted")
	}
}
