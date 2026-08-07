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
// map-of-strings shape. The core fields are still used so the mod can be
// matched and installed; its dependencies are simply not resolved.
func TestDownloadModWithUnexpectedDependencyShape(t *testing.T) {
	const arrayDepsModinfo = `{"modid":"arraydeps","name":"Array Deps","version":"1.0.0","dependencies":{"game":["1.20"],"othermod":"1.0.0"}}`
	runModinfoDownloadTest(t, "Array Deps", "1.0.0", arrayDepsModinfo)
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
}

func (downloader *rawModinfoDownloader) Download(
	_ context.Context,
	request application.DownloadRequest,
	_ chan<- application.DownloadProgress,
) error {
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
	if _, err := modInfo.Write([]byte(downloader.modinfo)); err != nil {
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
