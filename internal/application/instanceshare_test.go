package application_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/instancepackage"
)

func TestExportInstancePackageExcludesSensitiveData(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	instance, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name:          "Share me",
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}

	modsDirectory := filepath.Join(instance.Directory, "Mods")
	if err := os.MkdirAll(modsDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modsDirectory, "local-mod.zip"), []byte("local"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modsDirectory, "catalog-mod.zip"), []byte("catalog"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instance.Directory, "clientsettings.json"), []byte(`{"stringSettings":{"sessionkey":"TOP_SECRET","sessionsignature":"SIG","playeruid":"UID","playername":"gasada","useremail":"gasada@example.com","mptoken":"token","entitlements":"premium","fov":80},"stringListSettings":{"multiplayerservers":["server (:,192.0.2.1:42420,"],"modPaths":["Mods","/home/user/instances/original/Mods"]},"intsettings":{"viewDistance":256}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(instance.Directory, "Config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instance.Directory, "Config", "mymod.json"), []byte(`{"x":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(instance.Directory, "Logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instance.Directory, "Logs", "game.log"), []byte("log"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(instance.Directory, "SaveGame", "world"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instance.Directory, "SaveGame", "world", "data.txt"), []byte("save"), 0o600); err != nil {
		t.Fatal(err)
	}

	mods, err := fixture.service.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, mod := range mods {
		if mod.FileName == "catalog-mod.zip" {
			mod.Source = "moddb:123:456"
			mod.Managed = true
			if err := fixture.store.SaveMod(ctx, mod); err != nil {
				t.Fatal(err)
			}
		}
	}

	coverPath := filepath.Join(fixture.root, "cover.png")
	if err := os.WriteFile(coverPath, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	cover := coverPath
	instance.CoverPath = &cover
	if err := fixture.store.SaveInstance(ctx, instance); err != nil {
		t.Fatal(err)
	}

	packagePath := filepath.Join(fixture.root, "share.waxlight")
	manifest, err := fixture.service.ExportInstance(ctx, instance.ID, packagePath, domain.ExportInstanceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != domain.InstancePackageSchemaVersion {
		t.Fatalf("unexpected schema version %d", manifest.SchemaVersion)
	}
	if manifest.Name != "Share me" {
		t.Fatalf("unexpected name %q", manifest.Name)
	}
	if manifest.GameVersion.ID != "1.20" || manifest.GameVersion.Name != "1.20" {
		t.Fatalf("unexpected game version %+v", manifest.GameVersion)
	}

	var catalogRefs, embeddedRefs int
	for _, mod := range manifest.Mods {
		switch mod.Source {
		case domain.PackageModSourceCatalog:
			catalogRefs++
			if mod.ModID != "123" || mod.VersionID != "456" {
				t.Fatalf("unexpected catalog reference %+v", mod)
			}
		case domain.PackageModSourceEmbedded:
			embeddedRefs++
		}
	}
	if catalogRefs != 1 || embeddedRefs != 1 {
		t.Fatalf("expected one catalog and one embedded mod, got catalog=%d embedded=%d", catalogRefs, embeddedRefs)
	}

	pkg, err := instancepackage.Open(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"Logs/game.log", "SaveGame/world/data.txt", ".waxlight-instance"} {
		for name := range pkg.Entries {
			if strings.Contains(name, forbidden) {
				t.Fatalf("package leaked forbidden path %q", name)
			}
		}
	}
	if _, ok := pkg.Entries["mods/local-mod.zip"]; !ok {
		t.Fatal("embedded mod file missing from package")
	}

	restored := filepath.Join(fixture.root, "restored")
	if err := os.MkdirAll(restored, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := pkg.ExtractConfigs(ctx, restored); err != nil {
		t.Fatal(err)
	}
	settings, err := os.ReadFile(filepath.Join(restored, "clientsettings.json"))
	if err != nil {
		t.Fatal(err)
	}
	settingsText := string(settings)
	for _, forbidden := range []string{"TOP_SECRET", "sessionkey", "gasada", "useremail", "mptoken", "entitlements", "192.0.2.1", "modPaths"} {
		if strings.Contains(settingsText, forbidden) {
			t.Fatalf("client settings leaked sensitive data %q: %s", forbidden, settings)
		}
	}
	if !strings.Contains(settingsText, "fov") || !strings.Contains(settingsText, "viewDistance") {
		t.Fatalf("client settings lost non-sensitive values: %s", settings)
	}
}

func TestImportPackageCreatesIsolatedInstance(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	source, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name:          "Original",
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	modsDirectory := filepath.Join(source.Directory, "Mods")
	disabledDirectory := filepath.Join(source.Directory, "ModsDisabled")
	if err := os.MkdirAll(modsDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(disabledDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modsDirectory, "enabled-mod.zip"), []byte("enabled"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(disabledDirectory, "disabled-mod.zip"), []byte("disabled"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source.Directory, "Config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source.Directory, "Config", "config.json"), []byte(`{"k":"v"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.ListMods(ctx, source.ID); err != nil {
		t.Fatal(err)
	}

	packagePath := filepath.Join(fixture.root, "share.waxlight")
	if _, err := fixture.service.ExportInstance(ctx, source.ID, packagePath, domain.ExportInstanceOptions{}); err != nil {
		t.Fatal(err)
	}

	inspection, err := fixture.service.InspectPackage(ctx, packagePath)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Name != "Original" {
		t.Fatalf("unexpected inspection name %q", inspection.Name)
	}
	if inspection.VersionStatus != domain.PackageVersionInstalled {
		t.Fatalf("expected installed version status, got %s", inspection.VersionStatus)
	}
	if len(inspection.Mods) != 2 {
		t.Fatalf("expected two inspected mods, got %d", len(inspection.Mods))
	}

	report, err := fixture.service.ImportPackage(ctx, packagePath, domain.ImportInstanceOptions{
		Name: "Imported copy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.InstanceID == source.ID {
		t.Fatal("import reused the source instance id")
	}
	if report.InstanceName != "Imported copy" {
		t.Fatalf("unexpected imported name %q", report.InstanceName)
	}
	if report.GameVersionID != "1.20" {
		t.Fatalf("unexpected imported version %q", report.GameVersionID)
	}

	imported, err := fixture.service.GetInstance(ctx, report.InstanceID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(imported.Directory, "Mods", "enabled-mod.zip")); err != nil {
		t.Fatalf("enabled mod not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(imported.Directory, "ModsDisabled", "disabled-mod.zip")); err != nil {
		t.Fatalf("disabled mod not installed: %v", err)
	}
	configContents, err := os.ReadFile(filepath.Join(imported.Directory, "Config", "config.json"))
	if err != nil || string(configContents) != `{"k":"v"}` {
		t.Fatalf("config not restored: %v %q", err, configContents)
	}

	if _, err := fixture.service.GetInstance(ctx, source.ID); err != nil {
		t.Fatalf("source instance was modified: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source.Directory, "Mods", "enabled-mod.zip")); err != nil {
		t.Fatalf("source instance files were removed: %v", err)
	}

	installed := 0
	for _, mod := range report.Mods {
		if mod.Status == "installed" {
			installed++
		}
	}
	if installed != 2 {
		t.Fatalf("expected two installed mods in report, got %d", installed)
	}
}

func TestExportInstanceRejectsInvalidAuthorLinks(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	instance, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name:          "Author links",
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(fixture.root, "share.waxlight")

	for _, invalid := range []string{"not-a-url", "javascript:alert(1)", "file:///etc/passwd", "ftp://example.com"} {
		if _, err := fixture.service.ExportInstance(ctx, instance.ID, packagePath, domain.ExportInstanceOptions{
			Author: &domain.PackageAuthor{Homepage: invalid},
		}); err == nil {
			t.Fatalf("expected export to reject author link %q", invalid)
		}
	}
	if _, err := fixture.service.ExportInstance(ctx, instance.ID, packagePath, domain.ExportInstanceOptions{
		Author: &domain.PackageAuthor{Homepage: "https://example.com", Source: "http://example.org/repo"},
	}); err != nil {
		t.Fatalf("expected export to accept valid author links: %v", err)
	}
}

func TestImportPackageInstallsMissingGameVersionAndMods(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	fixture.service.ConfigureVersionDownloads(
		staticVersionCatalog{versions: []domain.AvailableGameVersion{
			{
				ID:           "1.21",
				Name:         "1.21",
				Channel:      "stable",
				Platform:     "linux",
				Architecture: "amd64",
				Filename:     "vintagestory.zip",
				DownloadURL:  "https://cdn.vintagestory.at/1.21.zip",
				DownloadSize: 1024,
				Checksum:     "abc",
			},
		}},
		recordingDownloader{},
		fakeGamePackageInstaller{},
	)
	fixture.service.ConfigureDiskSpaceChecker(fixedDiskSpace(1 << 40))

	embeddedSource := filepath.Join(fixture.root, "local.zip")
	if err := os.WriteFile(embeddedSource, []byte("mod-contents"), 0o600); err != nil {
		t.Fatal(err)
	}

	packagePath := filepath.Join(fixture.root, "share.waxlight")
	manifest := domain.PackageManifest{
		SchemaVersion: domain.InstancePackageSchemaVersion,
		Name:          "Imported with download",
		GameVersion:   domain.PackageGameVersion{ID: "1.21", Name: "1.21"},
		Mods: []domain.PackageMod{
			{
				Name: "Local Helper", Version: "1.0.0", FileName: "local.zip",
				Source: domain.PackageModSourceEmbedded, Checksum: "sha256:x",
				Enabled: true,
			},
		},
	}
	if err := instancepackage.Write(ctx, packagePath, instancepackage.WriteSource{
		Manifest:     manifest,
		InstanceDir:  fixture.root,
		EmbeddedMods: map[string]string{"local.zip": embeddedSource},
	}); err != nil {
		t.Fatal(err)
	}

	report, err := fixture.service.ImportPackage(ctx, packagePath, domain.ImportInstanceOptions{
		Name:           "Imported with download",
		InstallVersion: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.store.GetVersion(ctx, "1.21"); err != nil {
		t.Fatalf("required game version was not installed: %v", err)
	}
	if report.GameVersionID != "1.21" {
		t.Fatalf("unexpected imported version %q", report.GameVersionID)
	}

	imported, err := fixture.service.GetInstance(ctx, report.InstanceID)
	if err != nil {
		t.Fatal(err)
	}
	modPath := filepath.Join(imported.Directory, "Mods", "local.zip")
	contents, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatalf("mod not installed: %v", err)
	}
	if string(contents) != "mod-contents" {
		t.Fatalf("mod contents corrupted: %q", contents)
	}

	if err := assertNoSymlinks(imported.Directory); err != nil {
		t.Fatalf("instance contains symlinks: %v", err)
	}

	validation, err := fixture.service.ValidateLaunch(ctx, imported.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !validation.Valid {
		t.Fatalf("imported instance cannot launch: %v", validation.Issues)
	}
}

func assertNoSymlinks(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return domain.NewError(domain.ErrValidation, "unexpected symlink: "+path)
		}
		return nil
	})
}

func TestImportedInstanceHasNoStaleModPaths(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	source, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name:          "Source with mod paths",
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	// The original instance's client settings carry the absolute mod path that
	// Vintage Story wrote while the source instance was played, stored under
	// the CamelCase sections the game actually uses.
	staleSettings := `{"stringSettings":{"language":"en"},"stringListSettings":{"modPaths":["Mods","/home/user/.config/waxlight/instances/` + source.ID + `/Mods"]}}`
	if err := os.WriteFile(filepath.Join(source.Directory, "clientsettings.json"), []byte(staleSettings), 0o600); err != nil {
		t.Fatal(err)
	}

	packagePath := filepath.Join(fixture.root, "share.waxlight")
	if _, err := fixture.service.ExportInstance(ctx, source.ID, packagePath, domain.ExportInstanceOptions{}); err != nil {
		t.Fatal(err)
	}

	report, err := fixture.service.ImportPackage(ctx, packagePath, domain.ImportInstanceOptions{
		Name: "Imported copy",
	})
	if err != nil {
		t.Fatal(err)
	}
	imported, err := fixture.service.GetInstance(ctx, report.InstanceID)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := os.ReadFile(filepath.Join(imported.Directory, "clientsettings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(settings), source.ID) {
		t.Fatalf("imported client settings still reference the source instance: %s", settings)
	}
	if strings.Contains(strings.ToLower(string(settings)), "modpaths") {
		t.Fatalf("imported client settings still contain modPaths: %s", settings)
	}
	if !strings.Contains(string(settings), "language") {
		t.Fatalf("imported client settings lost non-sensitive values: %s", settings)
	}
}

func TestImportedInstanceSurvivesSourceDeletion(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	source, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name:          "Source pack",
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	modsDirectory := filepath.Join(source.Directory, "Mods")
	if err := os.MkdirAll(modsDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modsDirectory, "helper-mod.zip"), []byte("helper"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.ListMods(ctx, source.ID); err != nil {
		t.Fatal(err)
	}

	packagePath := filepath.Join(fixture.root, "share.waxlight")
	if _, err := fixture.service.ExportInstance(ctx, source.ID, packagePath, domain.ExportInstanceOptions{}); err != nil {
		t.Fatal(err)
	}

	report, err := fixture.service.ImportPackage(ctx, packagePath, domain.ImportInstanceOptions{
		Name: "Imported pack",
	})
	if err != nil {
		t.Fatal(err)
	}
	imported, err := fixture.service.GetInstance(ctx, report.InstanceID)
	if err != nil {
		t.Fatal(err)
	}
	modPath := filepath.Join(imported.Directory, "Mods", "helper-mod.zip")
	if _, err := os.Stat(modPath); err != nil {
		t.Fatalf("imported mod missing before deletion: %v", err)
	}

	if err := fixture.service.DeleteInstance(ctx, source.ID, true); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(modPath); err != nil {
		t.Fatalf("imported mod disappeared after deleting the source instance: %v", err)
	}
	validation, err := fixture.service.ValidateLaunch(ctx, imported.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !validation.Valid {
		t.Fatalf("imported instance cannot launch after source deletion: %v", validation.Issues)
	}
}

func TestImportLeavesNoStrayFilesOutsideInstance(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	source, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name:          "Source pack",
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	modsDirectory := filepath.Join(source.Directory, "Mods")
	if err := os.MkdirAll(modsDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modsDirectory, "local.zip"), []byte("local"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.ListMods(ctx, source.ID); err != nil {
		t.Fatal(err)
	}

	packagePath := filepath.Join(fixture.root, "share.waxlight")
	if _, err := fixture.service.ExportInstance(ctx, source.ID, packagePath, domain.ExportInstanceOptions{}); err != nil {
		t.Fatal(err)
	}

	before := snapshotTree(fixture.root)
	report, err := fixture.service.ImportPackage(ctx, packagePath, domain.ImportInstanceOptions{
		Name: "Imported pack",
	})
	if err != nil {
		t.Fatal(err)
	}
	imported, err := fixture.service.GetInstance(ctx, report.InstanceID)
	if err != nil {
		t.Fatal(err)
	}
	after := snapshotTree(fixture.root)
	allowed := map[string]bool{
		"instances/" + imported.ID:                          true,
		"instances/" + imported.ID + "/.waxlight-instance":  true,
		"instances/" + imported.ID + "/Mods":                true,
		"instances/" + imported.ID + "/Mods/local.zip":      true,
		"instances/" + imported.ID + "/ModsDisabled":        true,
		"instances/" + imported.ID + "/Logs":                true,
		"instances/" + imported.ID + "/clientsettings.json": true,
		"instances/" + imported.ID + "/Config":              true,
	}
	for path := range after {
		if before[path] {
			continue
		}
		if !allowed[path] {
			t.Fatalf("import left a stray file outside the new instance: %s", path)
		}
	}
}

func snapshotTree(root string) map[string]bool {
	result := make(map[string]bool)
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		result[filepath.ToSlash(relative)] = true
		return nil
	})
	return result
}
