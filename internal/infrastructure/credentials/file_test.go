package credentials

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/application"
)

func TestFileStoreLifecycleAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.json")
	store := NewFileStore(path)
	want := application.Secret{SessionKey: "key", SessionSignature: "signature"}
	if err := store.Set("account", want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("account")
	if err != nil || got != want {
		t.Fatalf("got %#v, %v", got, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("unexpected permissions %o", info.Mode().Perm())
		}
	}
	if err := store.Delete("account"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("account"); !errors.Is(err, application.ErrSecretNotFound) {
		t.Fatalf("expected missing secret, got %v", err)
	}
}

func TestFileStoreBacksUpCorruption(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "secrets.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewFileStore(path)
	if _, err := store.Get("account"); err == nil {
		t.Fatal("expected corruption error")
	}
	matches, err := filepath.Glob(path + ".corrupt-*")
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected one corruption backup, got %v, %v", matches, err)
	}
}
