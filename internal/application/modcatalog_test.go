package application_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/modstorage"
)

type staticModCatalog struct {
	details domain.ModDetails
}

func (catalog staticModCatalog) Search(
	context.Context,
	domain.ModSearchQuery,
) (domain.ModSearchResult, error) {
	return domain.ModSearchResult{Items: []domain.ModSummary{catalog.details.ModSummary}, Page: 1, PageSize: 24, TotalItems: 1}, nil
}

func (catalog staticModCatalog) Get(context.Context, string) (domain.ModDetails, error) {
	return catalog.details, nil
}

func TestDownloadCatalogModInstallsIntoSeveralInstances(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	first, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{Name: "First", GameVersionID: "1.20"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{Name: "Second", GameVersionID: "1.20"})
	if err != nil {
		t.Fatal(err)
	}
	details := domain.ModDetails{ModSummary: domain.ModSummary{ID: "51", Name: "Player Corpse", AuthorName: "Ada", LatestVersion: "2.0.0"}, Versions: []domain.ModVersion{{
		ID: "7", Version: "2.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable",
		FileName: "playercorpse.zip", DownloadURL: "https://cdn.test/playercorpse.zip",
	}}}
	fixture.service.ConfigureVersionDownloads(nil, recordingDownloader{}, nil)
	fixture.service.ConfigureMods(staticModCatalog{details: details}, modstorage.New(fixture.root))
	result, err := fixture.service.DownloadCatalogMod(ctx, domain.DownloadModRequest{
		ModID: "51", VersionID: "7", InstanceIDs: []string{first.ID, second.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Installations) != 2 || !result.Installations[0].Installed || !result.Installations[1].Installed {
		t.Fatalf("unexpected installation result: %#v", result.Installations)
	}
	for _, instance := range []domain.Instance{first, second} {
		installed, listErr := fixture.store.ListMods(ctx, instance.ID)
		if listErr != nil || len(installed) != 1 || installed[0].Source != "moddb:51:7" {
			t.Fatalf("unexpected installed mods: %#v, %v", installed, listErr)
		}
		if _, statErr := os.Stat(filepath.Join(instance.Directory, "Mods", "playercorpse.zip")); statErr != nil {
			t.Fatal(statErr)
		}
	}

	// Installing an already downloaded version reuses the cache and does not duplicate records.
	result, err = fixture.service.InstallDownloadedMod(ctx, "51", "7", []string{first.ID}, false)
	if err != nil || !result.Installations[0].Installed {
		t.Fatalf("unexpected repeated install: %#v, %v", result, err)
	}
	installed, _ := fixture.store.ListMods(ctx, first.ID)
	if len(installed) != 1 {
		t.Fatalf("repeated install created %d records", len(installed))
	}
}
