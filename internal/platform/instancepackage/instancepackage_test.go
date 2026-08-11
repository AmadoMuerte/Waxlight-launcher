package instancepackage

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/instances"
)

func TestWriteAndOpenRoundTrip(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	instanceDir := filepath.Join(root, "instance")
	if err := os.MkdirAll(filepath.Join(instanceDir, "Config", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := []byte(`{"stringSettings":{"sessionkey":"SECRET","sessionsignature":"SIG","playername":"u","useremail":"user@example.com","mptoken":"token","entitlements":"premium","fov":90},"stringListSettings":{"multiplayerservers":["server (:,1.2.3.4:42420,"],"modPaths":["Mods","/home/user/instances/original/Mods"]}}`)
	if err := os.WriteFile(filepath.Join(instanceDir, "clientsettings.json"), settings, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instanceDir, "Config", "nested", "mod.json"), []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "local-mod.zip"), []byte("mod-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cover.png"), []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}

	manifest := instances.PackageManifest{
		SchemaVersion: instances.InstancePackageSchemaVersion,
		Name:          "Test pack",
		Description:   "desc",
		GameVersion:   instances.PackageGameVersion{ID: "1.20", Name: "1.20"},
		ConfigFiles:   []string{"clientsettings.json", filepath.Join("Config", "nested", "mod.json")},
		HasIcon:       true,
		Mods: []instances.PackageMod{
			{
				Name: "CatalogMod", Version: "1.0", FileName: "catalog.zip",
				Source: instances.PackageModSourceCatalog, ModID: "12", VersionID: "34",
			},
			{
				Name: "LocalMod", Version: "2.0", FileName: "local-mod.zip",
				Source: instances.PackageModSourceEmbedded, Checksum: "sha256:abc",
			},
		},
	}

	packagePath := filepath.Join(root, "pack.waxlight")
	if err := Write(ctx, packagePath, WriteSource{
		Manifest:     manifest,
		InstanceDir:  instanceDir,
		EmbeddedMods: map[string]string{"local-mod.zip": filepath.Join(root, "local-mod.zip")},
		IconPath:     filepath.Join(root, "cover.png"),
	}); err != nil {
		t.Fatal(err)
	}

	pkg, err := Open(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Manifest.Name != "Test pack" {
		t.Fatalf("unexpected manifest name %q", pkg.Manifest.Name)
	}
	for _, expected := range []string{
		"configs/clientsettings.json",
		"configs/Config/nested/mod.json",
		"mods/local-mod.zip",
		"icon.png",
	} {
		if _, ok := pkg.Entries[expected]; !ok {
			t.Fatalf("missing expected entry %q", expected)
		}
	}

	target := filepath.Join(root, "restored")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := pkg.ExtractConfigs(ctx, target); err != nil {
		t.Fatal(err)
	}
	restoredSettings, err := os.ReadFile(filepath.Join(target, "clientsettings.json"))
	if err != nil {
		t.Fatal(err)
	}
	restoredText := string(restoredSettings)
	for _, forbidden := range []string{"SECRET", "playername", "useremail", "mptoken", "entitlements", "1.2.3.4", "modPaths"} {
		if strings.Contains(restoredText, forbidden) {
			t.Fatalf("client settings were not sanitized (%q present): %s", forbidden, restoredSettings)
		}
	}
	if !strings.Contains(restoredText, "fov") {
		t.Fatalf("client settings lost non-secret values: %s", restoredSettings)
	}
	restoredNested, err := os.ReadFile(filepath.Join(target, "Config", "nested", "mod.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(restoredNested) != `{"a":1}` {
		t.Fatalf("unexpected nested config contents %q", restoredNested)
	}

	modsDirectory := filepath.Join(root, "restoredmods")
	if err := os.MkdirAll(modsDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := pkg.ExtractEmbeddedMod(ctx, "local-mod.zip", modsDirectory); err != nil {
		t.Fatal(err)
	}
	modContents, err := os.ReadFile(filepath.Join(modsDirectory, "local-mod.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if string(modContents) != "mod-bytes" {
		t.Fatalf("unexpected embedded mod contents %q", modContents)
	}
}

func TestExtractConfigsSanitizesStaleClientSettings(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	// A package produced by an older launcher embeds the client settings as
	// they were on the source machine, including absolute mod paths.
	stale := []byte(`{"stringSettings":{"language":"en"},"stringListSettings":{"modPaths":["Mods","/home/user/instances/original/Mods"]}}`)
	if err := os.WriteFile(filepath.Join(root, "clientsettings.json"), stale, 0o600); err != nil {
		t.Fatal(err)
	}

	manifest := instances.PackageManifest{
		SchemaVersion: instances.InstancePackageSchemaVersion,
		Name:          "Stale pack",
		GameVersion:   instances.PackageGameVersion{ID: "1.20", Name: "1.20"},
		ConfigFiles:   []string{"clientsettings.json"},
	}
	packagePath := filepath.Join(root, "pack.waxlight")
	if err := Write(ctx, packagePath, WriteSource{
		Manifest:    manifest,
		InstanceDir: root,
	}); err != nil {
		t.Fatal(err)
	}
	pkg, err := Open(packagePath)
	if err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(root, "restored")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := pkg.ExtractConfigs(ctx, target); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(target, "clientsettings.json"))
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(contents))
	if strings.Contains(lower, "modpaths") || strings.Contains(lower, "instances/original") {
		t.Fatalf("extracted client settings still contain stale mod paths: %s", contents)
	}
	if !strings.Contains(string(contents), "language") {
		t.Fatalf("extracted client settings lost non-sensitive values: %s", contents)
	}
}

func TestRejectPathTraversal(t *testing.T) {
	path := craftArchive(t, map[string]zipContent{
		"manifest.json": {data: []byte(validManifest())},
		"../evil.txt":   {data: []byte("evil")},
	})
	if _, err := Open(path); err == nil {
		t.Fatal("expected traversal package to be rejected")
	} else if code := appErrorCode(err); code != errs.ErrPackageSecurity {
		t.Fatalf("expected security error, got %v", err)
	}
}

func TestRejectAbsolutePath(t *testing.T) {
	path := craftArchive(t, map[string]zipContent{
		"manifest.json": {data: []byte(validManifest())},
		"/etc/passwd":   {data: []byte("evil")},
	})
	if _, err := Open(path); err == nil {
		t.Fatal("expected absolute path to be rejected")
	}
}

func TestRejectUnexpectedEntry(t *testing.T) {
	path := craftArchive(t, map[string]zipContent{
		"manifest.json": {data: []byte(validManifest())},
		"secret.txt":    {data: []byte("evil")},
	})
	if _, err := Open(path); err == nil {
		t.Fatal("expected unexpected entry to be rejected")
	}
}

func TestRejectSymlinkEntry(t *testing.T) {
	path := craftArchive(t, map[string]zipContent{
		"manifest.json": {data: []byte(validManifest())},
		"configs/sym":   {data: []byte("x"), mode: os.ModeSymlink | 0o777},
	})
	if _, err := Open(path); err == nil {
		t.Fatal("expected symlink entry to be rejected")
	}
}

func TestRejectUnsupportedSchemaVersion(t *testing.T) {
	manifest := struct {
		SchemaVersion int                          `json:"schemaVersion"`
		Name          string                       `json:"name"`
		GameVersion   instances.PackageGameVersion `json:"gameVersion"`
		ConfigFiles   []string                     `json:"configFiles"`
	}{
		SchemaVersion: 2,
		Name:          "Future pack",
		GameVersion:   instances.PackageGameVersion{ID: "1.20", Name: "1.20"},
	}
	encoded, _ := json.Marshal(manifest)
	path := craftArchive(t, map[string]zipContent{
		"manifest.json": {data: encoded},
	})
	if _, err := Open(path); err == nil {
		t.Fatal("expected unsupported schema version to be rejected")
	} else if code := appErrorCode(err); code != errs.ErrPackageUnsupported {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}

func TestRejectMissingDeclaredConfig(t *testing.T) {
	manifest := struct {
		SchemaVersion int                          `json:"schemaVersion"`
		Name          string                       `json:"name"`
		GameVersion   instances.PackageGameVersion `json:"gameVersion"`
		ConfigFiles   []string                     `json:"configFiles"`
	}{
		SchemaVersion: instances.InstancePackageSchemaVersion,
		Name:          "Missing config pack",
		GameVersion:   instances.PackageGameVersion{ID: "1.20", Name: "1.20"},
		ConfigFiles:   []string{"Config/missing.json"},
	}
	encoded, _ := json.Marshal(manifest)
	path := craftArchive(t, map[string]zipContent{
		"manifest.json": {data: encoded},
	})
	if _, err := Open(path); err == nil {
		t.Fatal("expected missing declared config to be rejected")
	}
}

type zipContent struct {
	data []byte
	mode os.FileMode
}

func craftArchive(t *testing.T, contents map[string]zipContent) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "package.waxlight")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, content := range contents {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if content.mode != 0 {
			header.SetMode(content.mode)
		} else {
			header.SetMode(0o644)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func validManifest() string {
	manifest := instances.PackageManifest{
		SchemaVersion: instances.InstancePackageSchemaVersion,
		Name:          "Empty pack",
		GameVersion:   instances.PackageGameVersion{ID: "1.20", Name: "1.20"},
		ConfigFiles:   []string{},
	}
	encoded, _ := json.Marshal(manifest)
	return string(encoded)
}

func appErrorCode(err error) string {
	var appError *errs.AppError
	if errors.As(err, &appError) {
		return appError.Code
	}
	return ""
}
