package wails

import (
	"context"
	"testing"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
)

type upgradeListerStub struct{ installed []mods.InstalledMod }

func (stub upgradeListerStub) ListMods(context.Context, string) ([]mods.InstalledMod, error) {
	return stub.installed, nil
}

func newUpgradeController(installed []mods.InstalledMod, versions []mods.ModVersion) *ModManagerController {
	catalog := favoriteCatalogStub{details: mods.ModDetails{
		ModSummary: mods.ModSummary{ID: "example", Name: "Example"},
		Versions:   versions,
	}}
	service := mods.NewCatalogService(
		nil, nil, catalog, nil, nil, nil, nil, upgradeListerStub{installed: installed},
		nil, nil, nil, nil, nil, nil, nil, nil,
	)
	return NewModManagerController(nil, service, newTestLifecycle())
}

func TestGetInstanceModUpgradeVersionsFiltersAndMapsMetadata(t *testing.T) {
	published := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	controller := newUpgradeController([]mods.InstalledMod{{
		Version: "2.0.0", Source: "moddb:example:installed", Managed: true,
	}}, []mods.ModVersion{
		{ID: "old", Version: "1.9.0"},
		{ID: "equal", Version: "v2.0.0+metadata"},
		{ID: "new", Version: "3.0.0", GameVersions: []string{"1.21"}, ReleaseType: "stable", FileName: "new.zip", FileSize: 42, PublishedAt: &published, Changelog: "Changed"},
	})

	versions, err := controller.GetInstanceModUpgradeVersions("instance", " EXAMPLE ")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 {
		t.Fatalf("versions = %+v", versions)
	}
	version := versions[0]
	if version.ID != "new" || version.Version != "3.0.0" || version.ReleaseType != "stable" || version.FileName != "new.zip" || version.FileSize != 42 || version.Changelog != "Changed" || len(version.GameVersions) != 1 || version.PublishedAt == nil || *version.PublishedAt != iso(published) {
		t.Fatalf("metadata not preserved: %+v", version)
	}
}

func TestGetInstanceModUpgradeVersionsReturnsNonNilEmpty(t *testing.T) {
	controller := newUpgradeController(nil, nil)
	versions, err := controller.GetInstanceModUpgradeVersions("instance", "missing")
	if err != nil {
		t.Fatal(err)
	}
	if versions == nil || len(versions) != 0 {
		t.Fatalf("versions = %#v", versions)
	}
}
