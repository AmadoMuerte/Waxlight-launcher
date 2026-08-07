package application_test

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/modstorage"
)

// TassFactions ships its modinfo.json with a UTF-8 byte order mark, which the
// standard library rejects even though the JSON is valid. The download must
// still succeed.
func TestDownloadModWithBOMInModinfo(t *testing.T) {
	const bomModinfo = "\xEF\xBB\xBF" + `{"modid":"tassfactions","name":"Tass Factions","version":"0.7.9","side":"Universal","dependencies":{"game":"1.22.0"}}`
	runModinfoDownloadTest(t, "TassFactions", "0.7.9", bomModinfo)
}

// Some mods publish a dependencies block that does not match the expected
// map-of-strings shape. Such entries are ignored so the mod can still be
// matched and installed.
func TestDownloadModWithUnexpectedDependencyShape(t *testing.T) {
	const arrayDepsModinfo = `{"modid":"arraydeps","name":"Array Deps","version":"1.0.0","dependencies":{"game":["1.20"]}}`
	runModinfoDownloadTest(t, "Array Deps", "1.0.0", arrayDepsModinfo)
}

// genelib and bixo (dependencies of Land of Estrellas) ship modinfo.json with
// trailing commas, which the game loads but the standard library rejects. The
// download must still succeed.
func TestDownloadModWithTrailingCommasInModinfo(t *testing.T) {
	const trailingCommasModinfo = `{
  "type": "code",
  "modid": "genelib",
  "name": "Genelib",
  "version": "3.2.0",
  "dependencies": {
    "game": "1.22.0",
  },
  "custom": {"logosource": "x"},
}`
	runModinfoDownloadTest(t, "Genelib", "3.2.0", trailingCommasModinfo)
}

// Lenient modinfo must keep the string dependencies it can read, so the
// launcher still resolves them instead of dropping them silently.
func TestLenientModinfoKeepsStringDependencies(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	root := domain.ModDetails{
		ModSummary: domain.ModSummary{ID: "51", Name: "Root Mod", AuthorName: "Tass", LatestVersion: "1.0.0"},
		Versions: []domain.ModVersion{{
			ID: "9", Version: "1.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable",
			FileName: "root.zip", DownloadURL: "https://cdn.test/root.zip",
		}},
	}
	library := domain.ModDetails{
		ModSummary: domain.ModSummary{ID: "60", Slug: "playermodellib", Name: "Player Model Lib", AuthorName: "Sekel", LatestVersion: "1.23.1"},
		Versions: []domain.ModVersion{{
			ID: "12", Version: "1.23.1", GameVersions: []string{"1.20"}, ReleaseType: "stable",
			FileName: "playermodellib.zip", DownloadURL: "https://cdn.test/playermodellib.zip",
		}},
	}
	const trailingCommasModinfo = `{
  "modid": "rootmod",
  "name": "Root Mod",
  "version": "1.0.0",
  "dependencies": {
    "game": "1.20",
    "playermodellib": "1.23.1",
  },
}`
	fixture.service.ConfigureVersionDownloads(nil, &rawModinfoDownloader{byURL: map[string]string{
		"https://cdn.test/root.zip":           trailingCommasModinfo,
		"https://cdn.test/playermodellib.zip": `{"modid":"playermodellib","name":"Player Model Lib","version":"1.23.1","dependencies":{"game":"1.20"}}`,
	}}, nil)
	fixture.service.ConfigureMods(staticModCatalog{detailsByID: map[string]domain.ModDetails{
		"51":             root,
		"rootmod":        root,
		"60":             library,
		"playermodellib": library,
	}}, modstorage.New(fixture.root))
	instance, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{Name: "I", GameVersionID: "1.20"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := fixture.service.DownloadCatalogMod(ctx, domain.DownloadModRequest{
		ModID: "51", VersionID: "9", InstanceIDs: []string{instance.ID},
	})
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if len(result.Installations) != 1 || !result.Installations[0].Installed {
		t.Fatalf("unexpected installation result: %#v", result.Installations)
	}
	installed, err := fixture.store.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(installed) != 2 {
		t.Fatalf("expected the mod and its dependency to be installed, got %#v", installed)
	}
}

func runModinfoDownloadTest(t *testing.T, name, version, modinfo string) {
	t.Helper()
	fixture := newTestFixture(t)
	ctx := context.Background()
	fixture.service.ConfigureVersionDownloads(nil, &rawModinfoDownloader{modinfo: modinfo}, nil)
	details := domain.ModDetails{
		ModSummary: domain.ModSummary{ID: "51", Name: name, AuthorName: "Tass", LatestVersion: version},
		Versions: []domain.ModVersion{{
			ID: "9", Version: version, GameVersions: []string{"1.20"}, ReleaseType: "stable",
			FileName: "mod.zip", DownloadURL: "https://cdn.test/mod.zip",
		}},
	}
	fixture.service.ConfigureMods(staticModCatalog{details: details}, modstorage.New(fixture.root))
	instance, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{Name: "I", GameVersionID: "1.20"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := fixture.service.DownloadCatalogMod(ctx, domain.DownloadModRequest{
		ModID: "51", VersionID: "9", InstanceIDs: []string{instance.ID},
	})
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if len(result.Installations) != 1 || !result.Installations[0].Installed {
		t.Fatalf("unexpected installation result: %#v", result.Installations)
	}
}

type rawModinfoDownloader struct {
	modinfo string
	byURL   map[string]string
}

func (downloader *rawModinfoDownloader) Download(
	_ context.Context,
	request application.DownloadRequest,
	_ chan<- application.DownloadProgress,
) error {
	content := downloader.modinfo
	if byURL := downloader.byURL[request.URL]; byURL != "" {
		content = byURL
	}
	if err := os.MkdirAll(filepath.Dir(request.DestinationPath), 0o755); err != nil {
		return err
	}
	file, err := os.Create(request.DestinationPath)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	modInfo, err := writer.Create("modinfo.json")
	if err != nil {
		_ = file.Close()
		return err
	}
	if _, err := modInfo.Write([]byte(content)); err != nil {
		_ = writer.Close()
		_ = file.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func (downloader *rawModinfoDownloader) ContentLength(context.Context, string) (int64, error) {
	return 0, nil
}
