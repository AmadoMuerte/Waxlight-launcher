package modstorage_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/modstorage"
)

func cachedMod(root string) mods.DownloadedMod {
	return mods.DownloadedMod{
		SchemaVersion:     1,
		ModID:             "42",
		Name:              "Test Mod",
		VersionID:         "7",
		DownloadedVersion: "1.0.0",
		Tags:              []string{},
		FileName:          "testmod_1.0.0.zip",
		FilePath:          filepath.Join(root, "cache", "mods", "42", "7", "testmod_1.0.0.zip"),
		DownloadURL:       "https://mods.example/testmod_1.0.0.zip",
		DownloadedAt:      time.Now().UTC(),
	}
}

func TestSaveStoresCachePathRelativeAndGetResolvesIt(t *testing.T) {
	root := t.TempDir()
	store := modstorage.New(root)
	ctx := context.Background()

	value := cachedMod(root)
	if err := store.Save(ctx, value); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(root, "cache", "mods", "42", "7", "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), root) {
		t.Fatalf("cached metadata must not contain the absolute data root: %s", raw)
	}

	loaded, err := store.Get(ctx, "42", "7")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.FilePath != value.FilePath {
		t.Fatalf("resolved file path = %q, want %q", loaded.FilePath, value.FilePath)
	}
	items, err := store.List(ctx)
	if err != nil || len(items) != 1 || items[0].FilePath != value.FilePath {
		t.Fatalf("unexpected list result: %#v, %v", items, err)
	}
}

func TestSaveKeepsExternalLinkedModPathAbsolute(t *testing.T) {
	root := t.TempDir()
	store := modstorage.New(root)
	ctx := context.Background()

	value := cachedMod(root)
	value.FilePath = filepath.Join(t.TempDir(), "elsewhere", "mod.zip")
	if err := store.Save(ctx, value); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Get(ctx, "42", "7")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.FilePath != value.FilePath {
		t.Fatalf("external linked mod path must stay absolute, got %q", loaded.FilePath)
	}
}

func TestRelocateOldRootHealsStaleAbsoluteCachePaths(t *testing.T) {
	oldRoot := t.TempDir()
	newRoot := t.TempDir()

	stale := cachedMod(oldRoot)
	metadataDir := filepath.Join(newRoot, "cache", "mods", "42", "7")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"schemaVersion":1,"modId":"42","name":"Test Mod","side":"unknown","versionId":"7",` +
		`"downloadedVersion":"1.0.0","gameVersions":[],"tags":[],"fileName":"testmod_1.0.0.zip",` +
		`"filePath":` + strconvQuote(stale.FilePath) + `,"fileSize":10,"downloadUrl":"https://mods.example/testmod_1.0.0.zip",` +
		`"downloadedAt":"2026-01-01T00:00:00Z","updateAvailable":false}`
	if err := os.WriteFile(filepath.Join(metadataDir, "metadata.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	store := modstorage.New(newRoot)
	if err := store.RelocateOldRoot(context.Background(), oldRoot); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Get(context.Background(), "42", "7")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(newRoot, "cache", "mods", "42", "7", "testmod_1.0.0.zip")
	if loaded.FilePath != want {
		t.Fatalf("stale cache path was not healed: %q, want %q", loaded.FilePath, want)
	}
}

func strconvQuote(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	return `"` + escaped + `"`
}
