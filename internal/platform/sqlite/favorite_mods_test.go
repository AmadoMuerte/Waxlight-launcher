package sqlite_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/sqlite"
)

func TestFavoriteModsCRUDIsDeterministicAndIdempotent(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "favorites.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	ids, err := store.ListFavoriteModIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ids == nil || len(ids) != 0 {
		t.Fatalf("initial favorites = %#v, want non-nil empty slice", ids)
	}

	for _, id := range []string{"zeta/mod?release=1", "alpha mod", "zeta/mod?release=1"} {
		if err := store.SetModFavorite(ctx, id, true); err != nil {
			t.Fatal(err)
		}
	}
	ids, err = store.ListFavoriteModIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"alpha mod", "zeta/mod?release=1"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("favorites = %#v, want %#v", ids, want)
	}

	for range 2 {
		if err := store.SetModFavorite(ctx, "alpha mod", false); err != nil {
			t.Fatal(err)
		}
		if err := store.SetModFavorite(ctx, "missing'; DROP TABLE favorite_mods; --", false); err != nil {
			t.Fatal(err)
		}
	}
	ids, err = store.ListFavoriteModIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"zeta/mod?release=1"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("favorites = %#v, want %#v", ids, want)
	}
}

func TestFavoriteModsPersistAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "favorites.db")
	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetModFavorite(context.Background(), "persisted", true); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ids, err := store.ListFavoriteModIDs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"persisted"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("favorites after reopen = %#v, want %#v", ids, want)
	}
}
