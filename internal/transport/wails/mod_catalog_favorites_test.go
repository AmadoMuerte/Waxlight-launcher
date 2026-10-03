package wails

import (
	"context"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/mutations"
)

type favoriteCatalogStub struct{ details mods.ModDetails }

func (stub favoriteCatalogStub) List(context.Context) ([]mods.ModSummary, error) {
	return []mods.ModSummary{stub.details.ModSummary}, nil
}
func (favoriteCatalogStub) Search(context.Context, mods.ModSearchQuery) (mods.ModSearchResult, error) {
	return mods.ModSearchResult{}, nil
}
func (stub favoriteCatalogStub) Get(context.Context, string) (mods.ModDetails, error) {
	return stub.details, nil
}
func (favoriteCatalogStub) ListTags(context.Context) ([]mods.ModTag, error) {
	return []mods.ModTag{}, nil
}

type favoriteStoreStub struct{ ids []string }

func (stub *favoriteStoreStub) ListFavoriteModIDs(context.Context) ([]string, error) {
	return append([]string{}, stub.ids...), nil
}
func (stub *favoriteStoreStub) SetModFavorite(_ context.Context, id string, favorite bool) error {
	if favorite {
		stub.ids = append(stub.ids, id)
		return nil
	}
	stub.ids = nil
	return nil
}

type favoriteRepositoryStub struct{}

func (favoriteRepositoryStub) GetInstance(context.Context, string) (mods.InstanceRef, error) {
	return mods.InstanceRef{}, nil
}
func (favoriteRepositoryStub) ListInstances(context.Context) ([]mods.InstanceRef, error) {
	return []mods.InstanceRef{}, nil
}
func (favoriteRepositoryStub) ListMods(context.Context, string) ([]mods.InstalledMod, error) {
	return []mods.InstalledMod{}, nil
}
func (favoriteRepositoryStub) GetMod(context.Context, string) (mods.InstalledMod, error) {
	return mods.InstalledMod{}, nil
}
func (favoriteRepositoryStub) SaveMod(context.Context, mods.InstalledMod) error { return nil }
func (favoriteRepositoryStub) DeleteMod(context.Context, string) error          { return nil }

type favoriteDownloadsStub struct{}

func (favoriteDownloadsStub) List(context.Context) ([]mods.DownloadedMod, error) {
	return []mods.DownloadedMod{}, nil
}
func (favoriteDownloadsStub) Get(context.Context, string, string) (mods.DownloadedMod, error) {
	return mods.DownloadedMod{}, nil
}
func (favoriteDownloadsStub) Save(context.Context, mods.DownloadedMod) error  { return nil }
func (favoriteDownloadsStub) Delete(context.Context, string, string) error    { return nil }
func (favoriteDownloadsStub) FilePath(string, string, string) (string, error) { return "", nil }

func newFavoriteModCatalogController() (*ModCatalogController, *favoriteStoreStub) {
	store := &favoriteStoreStub{}
	service := mods.NewCatalogService(
		favoriteRepositoryStub{}, nil,
		favoriteCatalogStub{details: mods.ModDetails{ModSummary: mods.ModSummary{ID: "canonical", Name: "Example"}}},
		store, favoriteDownloadsStub{}, nil, nil, nil, &mutations.Gate{}, nil, nil, nil, nil, nil, nil, nil,
	)
	return NewModCatalogController(service, newTestLifecycle()), store
}

func TestModCatalogFavoriteMethodsConvertNonNilArrays(t *testing.T) {
	controller, store := newFavoriteModCatalogController()
	ids, err := controller.ListFavoriteModIDs()
	if err != nil || ids == nil || len(ids) != 0 {
		t.Fatalf("ListFavoriteModIDs() = %#v, %v", ids, err)
	}
	items, err := controller.ListFavoriteMods()
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("ListFavoriteMods() = %#v, %v", items, err)
	}
	if err := controller.SetModFavorite("slug", true); err != nil {
		t.Fatal(err)
	}
	if len(store.ids) != 1 || store.ids[0] != "canonical" {
		t.Fatalf("stored IDs = %#v", store.ids)
	}
	items, err = controller.ListFavoriteMods()
	if err != nil || len(items) != 1 || items[0].ID != "canonical" {
		t.Fatalf("ListFavoriteMods() = %#v, %v", items, err)
	}
}
