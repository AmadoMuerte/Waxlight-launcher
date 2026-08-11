package application_test

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/modstorage"
	"github.com/waxlight/waxlight-launcher/internal/instances"
)

func writeLocalModZip(t *testing.T, path, modID, name, version string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	modInfo, err := archive.Create("modinfo.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{
		"modid":        modID,
		"name":         name,
		"version":      version,
		"dependencies": map[string]string{},
	}
	if err := json.NewEncoder(modInfo).Encode(manifest); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func corpseCatalog() staticModCatalog {
	details := domain.ModDetails{
		ModSummary: domain.ModSummary{
			ID: "51", Slug: "playercorpse", Name: "Player Corpse", AuthorName: "Ada",
			ModIDStrings: []string{"playercorpse"}, LatestVersion: "2.0.0",
		},
		Versions: []domain.ModVersion{{
			ID: "7", Version: "2.0.0", GameVersions: []string{"1.20"}, ReleaseType: "stable",
			FileName: "playercorpse.zip", DownloadURL: "https://cdn.test/playercorpse.zip",
		}},
	}
	return staticModCatalog{details: details}
}

func TestLinkLocalModsBindsByModIDAndVersion(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance, err := fixture.service.CreateInstance(ctx, instances.CreateInput{
		Name: "Linked", GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	modPath := filepath.Join(instance.Directory, "Mods", "playercorpse.zip")
	writeLocalModZip(t, modPath, "playercorpse", "Player Corpse", "2.0.0")
	fixture.service.ConfigureMods(corpseCatalog(), modstorage.New(fixture.root))

	result, err := fixture.service.LinkLocalMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Linked) != 1 || len(result.NotMatched) != 0 || len(result.Failed) != 0 {
		t.Fatalf("unexpected link result: %#v", result)
	}
	link := result.Linked[0]
	if link.ModID != "51" || link.VersionID != "7" || link.Version != "2.0.0" {
		t.Fatalf("unexpected link: %#v", link)
	}

	installed, err := fixture.store.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(installed) != 1 || !installed[0].Managed || installed[0].Source != "moddb:51:7" {
		t.Fatalf("installed mod was not bound: %#v", installed)
	}

	downloaded, err := fixture.service.ListDownloadedMods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(downloaded) != 1 || downloaded[0].ModID != "51" || downloaded[0].VersionID != "7" {
		t.Fatalf("unexpected downloaded mods: %#v", downloaded)
	}
	if len(downloaded[0].InstalledInstances) != 1 {
		t.Fatalf("expected the linked instance, got %#v", downloaded[0].InstalledInstances)
	}

	// A second run must not re-link an already managed mod.
	result, err = fixture.service.LinkLocalMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Linked) != 0 || len(result.NotMatched) != 0 {
		t.Fatalf("managed mod was relinked: %#v", result)
	}
}

func TestLinkLocalModsReportsUnmatchedMod(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance, err := fixture.service.CreateInstance(ctx, instances.CreateInput{
		Name: "Unmatched", GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	modPath := filepath.Join(instance.Directory, "Mods", "mysterymod.zip")
	writeLocalModZip(t, modPath, "mysterymod", "Mystery Mod", "1.0.0")
	fixture.service.ConfigureMods(corpseCatalog(), modstorage.New(fixture.root))

	result, err := fixture.service.LinkLocalMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Linked) != 0 || len(result.NotMatched) != 1 || len(result.Failed) != 0 {
		t.Fatalf("unexpected link result: %#v", result)
	}
	installed, err := fixture.store.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(installed) != 1 || installed[0].Managed || installed[0].Source != "local" {
		t.Fatalf("unmatched mod must stay local: %#v", installed)
	}
}

func TestLinkLocalModsSkipsModWithoutCatalogModID(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance, err := fixture.service.CreateInstance(ctx, instances.CreateInput{
		Name: "ByName", GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	// The modid is not listed in the catalog; the matching name must not be
	// used as a fallback.
	modPath := filepath.Join(instance.Directory, "Mods", "player-corpse.zip")
	writeLocalModZip(t, modPath, "some-other-modid", "Player Corpse", "2.0.0")
	fixture.service.ConfigureMods(corpseCatalog(), modstorage.New(fixture.root))

	result, err := fixture.service.LinkLocalMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Linked) != 0 || len(result.NotMatched) != 1 || len(result.Failed) != 0 {
		t.Fatalf("expected a strict modid-based miss, got %#v", result)
	}
}

func TestUploadModsBindsAndCopiesIntoLibrary(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	corpsePath := filepath.Join(fixture.root, "corpse.zip")
	writeLocalModZip(t, corpsePath, "playercorpse", "Player Corpse", "2.0.0")
	mysteryPath := filepath.Join(fixture.root, "mystery.zip")
	writeLocalModZip(t, mysteryPath, "mysterymod", "Mystery Mod", "1.0.0")
	unsupported := filepath.Join(fixture.root, "note.txt")
	if err := os.WriteFile(unsupported, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixture.service.ConfigureMods(corpseCatalog(), modstorage.New(fixture.root))

	result, err := fixture.service.UploadMods(ctx, []string{corpsePath, mysteryPath, unsupported})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Linked) != 1 || len(result.NotMatched) != 1 || len(result.Failed) != 1 {
		t.Fatalf("unexpected upload result: %#v", result)
	}

	downloaded, err := fixture.service.ListDownloadedMods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(downloaded) != 1 || downloaded[0].ModID != "51" {
		t.Fatalf("unexpected downloaded mods: %#v", downloaded)
	}
	if _, err := os.Stat(downloaded[0].FilePath); err != nil {
		t.Fatalf("bound file was not copied into the library: %v", err)
	}
	if filepath.Clean(downloaded[0].FilePath) == filepath.Clean(corpsePath) {
		t.Fatal("uploaded file must be copied, not referenced in place")
	}

	// Uploading the same mod again is reported as skipped.
	result, err = fixture.service.UploadMods(ctx, []string{corpsePath})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Linked) != 0 || len(result.Skipped) != 1 || len(result.NotMatched) != 0 {
		t.Fatalf("expected the duplicate to be skipped: %#v", result)
	}
}

func TestLinkLocalModsBindsModAlreadyInLibrary(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	second, err := fixture.service.CreateInstance(ctx, instances.CreateInput{
		Name: "Fresh", GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.ConfigureMods(corpseCatalog(), modstorage.New(fixture.root))

	// Import the mod into the library first, so a cache record already exists.
	corpsePath := filepath.Join(fixture.root, "corpse.zip")
	writeLocalModZip(t, corpsePath, "playercorpse", "Player Corpse", "2.0.0")
	if _, err := fixture.service.UploadMods(ctx, []string{corpsePath}); err != nil {
		t.Fatal(err)
	}

	// A second instance carries the same file as a plain local mod.
	modPath := filepath.Join(second.Directory, "Mods", "playercorpse.zip")
	writeLocalModZip(t, modPath, "playercorpse", "Player Corpse", "2.0.0")
	result, err := fixture.service.LinkLocalMods(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Linked) != 1 || len(result.Failed) != 0 {
		t.Fatalf("already-cached mod must be linked, not failed: %#v", result)
	}
	installed, err := fixture.store.ListMods(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(installed) != 1 || !installed[0].Managed || installed[0].Source != "moddb:51:7" {
		t.Fatalf("installed mod was not bound: %#v", installed)
	}
}

func TestUploadModsRecognizesModInstalledInInstance(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	fixture.setDownloader(recordingDownloader{})
	fixture.service.ConfigureMods(corpseCatalog(), modstorage.New(fixture.root))

	instance, err := fixture.service.CreateInstance(ctx, instances.CreateInput{
		Name: "AlreadyInstalled", GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	modPath := filepath.Join(instance.Directory, "Mods", "playercorpse.zip")
	writeLocalModZip(t, modPath, "playercorpse", "Player Corpse", "2.0.0")
	if _, err := fixture.service.ListMods(ctx, instance.ID); err != nil {
		t.Fatal(err)
	}

	// Uploading the same local file must also bind the instance-local mod.
	result, err := fixture.service.UploadMods(ctx, []string{modPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Linked) != 1 {
		t.Fatalf("expected one linked upload: %#v", result)
	}

	downloaded, err := fixture.service.ListDownloadedMods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(downloaded) != 1 || len(downloaded[0].InstalledInstances) != 1 {
		t.Fatalf("downloaded mod must be recognized as installed: %#v", downloaded)
	}
	installed, err := fixture.store.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(installed) != 1 || !installed[0].Managed || installed[0].Source != "moddb:51:7" {
		t.Fatalf("instance-local mod was not bound on upload: %#v", installed)
	}

	// Installing the downloaded mod into the same instance is a no-op, not an error.
	install, err := fixture.service.InstallDownloadedMod(ctx, "51", "7", []string{instance.ID}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !install.Installations[0].Installed || install.Installations[0].Message != "Already installed" {
		t.Fatalf("expected an already-installed result, got %#v", install.Installations)
	}
}

func TestInstallModFileBindsToExistingLibraryMod(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	fixture.setDownloader(recordingDownloader{})
	fixture.service.ConfigureMods(corpseCatalog(), modstorage.New(fixture.root))

	// The mod is already in the library.
	corpsePath := filepath.Join(fixture.root, "corpse.zip")
	writeLocalModZip(t, corpsePath, "playercorpse", "Player Corpse", "2.0.0")
	if _, err := fixture.service.UploadMods(ctx, []string{corpsePath}); err != nil {
		t.Fatal(err)
	}

	instance, err := fixture.service.CreateInstance(ctx, instances.CreateInput{
		Name: "NewInstance", GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	localPath := filepath.Join(fixture.root, "local-copy.zip")
	writeLocalModZip(t, localPath, "playercorpse", "Player Corpse", "2.0.0")
	if _, err := fixture.service.InstallModFile(ctx, instance.ID, localPath, "", ""); err != nil {
		t.Fatal(err)
	}

	installed, err := fixture.store.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(installed) != 1 || !installed[0].Managed || installed[0].Source != "moddb:51:7" {
		t.Fatalf("locally installed mod was not bound to the existing library entry: %#v", installed)
	}
	downloaded, err := fixture.service.ListDownloadedMods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(downloaded) != 1 || len(downloaded[0].InstalledInstances) != 1 {
		t.Fatalf("downloaded mod must list the instance: %#v", downloaded)
	}
}
