package mods_test

import (
	"context"
	"errors"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
)

type countingCatalog struct {
	staticModCatalog
	listCalls int
	getCalls  int
	listErr   error
}

func (catalog *countingCatalog) List(ctx context.Context) ([]mods.ModSummary, error) {
	catalog.listCalls++
	if catalog.listErr != nil {
		return nil, catalog.listErr
	}
	return catalog.staticModCatalog.List(ctx)
}

func (catalog *countingCatalog) Get(ctx context.Context, id string) (mods.ModDetails, error) {
	catalog.getCalls++
	return catalog.staticModCatalog.Get(ctx, id)
}

func TestFavoriteModsEmptyAvoidsCatalog(t *testing.T) {
	catalog := &countingCatalog{staticModCatalog: corpseCatalog()}
	fixture := newTestFixtureWithDeps(t, catalog, recordingDownloader{})

	items, err := fixture.catalogService.ListFavoriteMods(context.Background())
	if err != nil || len(items) != 0 || catalog.listCalls != 0 {
		t.Fatalf("ListFavoriteMods() = %#v, %v; list calls = %d", items, err, catalog.listCalls)
	}
}

func TestFavoriteModsListsOnceAndRetainsMissingIDs(t *testing.T) {
	catalog := &countingCatalog{staticModCatalog: corpseCatalog()}
	fixture := newTestFixtureWithDeps(t, catalog, recordingDownloader{})
	fixture.favorites.ids = []string{"51", "missing"}

	items, err := fixture.catalogService.ListFavoriteMods(context.Background())
	if err != nil || len(items) != 1 || items[0].ID != "51" || catalog.listCalls != 1 {
		t.Fatalf("ListFavoriteMods() = %#v, %v; list calls = %d", items, err, catalog.listCalls)
	}
	ids, err := fixture.catalogService.ListFavoriteModIDs(context.Background())
	if err != nil || len(ids) != 2 || ids[1] != "missing" {
		t.Fatalf("ListFavoriteModIDs() = %#v, %v", ids, err)
	}
}

func TestSetModFavoriteCanonicalAndOfflineRemoval(t *testing.T) {
	details := corpseCatalog().details
	details.ID = "opaque/canonical"
	catalog := &countingCatalog{staticModCatalog: staticModCatalog{details: details}}
	fixture := newTestFixtureWithDeps(t, catalog, recordingDownloader{})

	if err := fixture.catalogService.SetModFavorite(context.Background(), " playercorpse ", true); err != nil {
		t.Fatal(err)
	}
	if len(fixture.favorites.ids) != 1 || fixture.favorites.ids[0] != "opaque/canonical" || catalog.getCalls != 1 {
		t.Fatalf("stored IDs = %#v; get calls = %d", fixture.favorites.ids, catalog.getCalls)
	}
	catalog.staticModCatalog = staticModCatalog{}
	if err := fixture.catalogService.SetModFavorite(context.Background(), " opaque/canonical ", false); err != nil {
		t.Fatal(err)
	}
	if len(fixture.favorites.ids) != 0 || catalog.getCalls != 1 {
		t.Fatalf("stored IDs = %#v; get calls = %d", fixture.favorites.ids, catalog.getCalls)
	}
}

func TestSetModFavoriteValidatesAndGatesOnlyWrite(t *testing.T) {
	catalog := &countingCatalog{staticModCatalog: corpseCatalog()}
	fixture := newTestFixtureWithDeps(t, catalog, recordingDownloader{})
	if err := fixture.catalogService.SetModFavorite(context.Background(), " \t", true); err == nil || catalog.getCalls != 0 {
		t.Fatalf("blank favorite error = %v; get calls = %d", err, catalog.getCalls)
	}
	if err := fixture.gate.BeginRelocation(); err != nil {
		t.Fatal(err)
	}
	defer fixture.gate.EndRelocation()
	if err := fixture.catalogService.SetModFavorite(context.Background(), "51", true); err == nil || catalog.getCalls != 1 {
		t.Fatalf("gated add error = %v; get calls = %d", err, catalog.getCalls)
	}
	fixture.gate.EndRelocation()
	catalog.staticModCatalog.details.ID = ""
	if err := fixture.catalogService.SetModFavorite(context.Background(), "51", true); err == nil {
		t.Fatal("empty catalog ID was accepted")
	}
}

func TestFavoriteModsPropagatesStorageAndCatalogFailures(t *testing.T) {
	catalogErr := errors.New("catalog unavailable")
	catalog := &countingCatalog{staticModCatalog: corpseCatalog(), listErr: catalogErr}
	fixture := newTestFixtureWithDeps(t, catalog, recordingDownloader{})
	fixture.favorites.ids = []string{"51"}
	if _, err := fixture.catalogService.ListFavoriteMods(context.Background()); !errors.Is(err, catalogErr) {
		t.Fatalf("catalog error = %v", err)
	}
	storeErr := errors.New("storage unavailable")
	fixture.favorites.err = storeErr
	if _, err := fixture.catalogService.ListFavoriteModIDs(context.Background()); !errors.Is(err, storeErr) {
		t.Fatalf("storage error = %v", err)
	}
	if err := fixture.catalogService.SetModFavorite(context.Background(), "51", false); !errors.Is(err, storeErr) {
		t.Fatalf("storage write error = %v", err)
	}
}
