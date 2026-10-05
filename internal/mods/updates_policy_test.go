package mods

import (
	"context"
	"testing"

	vsmodpack "github.com/AmadoMuerte/vintagestory-go/modpack"
)

func TestPendingModUpdatesRespectPolicies(t *testing.T) {
	installed := []InstalledMod{
		{Version: "1.0.0", Source: "moddb:auto:old", Managed: true, UpdatePolicy: UpdatePolicyAutomatic},
		{Version: "1.0.0", Source: "moddb:pinned:old", Managed: true, UpdatePolicy: UpdatePolicyPinned},
		{Version: "1.0.0", Source: "moddb:ignored:old", Managed: true, UpdatePolicy: UpdatePolicyPinned},
	}
	service := &CatalogService{catalog: policyTestCatalog{}}
	pending, skipped, err := service.pendingModUpdates(context.Background(), installed, []ModUpdateTarget{
		{ModID: "auto", VersionID: "new"},
		{ModID: "pinned", VersionID: "new"},
		{ModID: "ignored", VersionID: "new"},
	}, "1.20")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].ModID != "auto" || skipped != 2 {
		t.Fatalf("pending = %+v, skipped = %d", pending, skipped)
	}
}

func TestPendingModUpdatesRequireStrictIncrease(t *testing.T) {
	service := &CatalogService{catalog: policyVersionsCatalog{versions: []ModVersion{
		{ID: "older", Version: "1.9.0"},
		{ID: "equal", Version: "v2.0.0+catalog"},
		{ID: "newer", Version: "2.1.0"},
	}}}
	installed := []InstalledMod{{Version: "2.0", Source: "moddb:Example:installed", Managed: true}}
	pending, _, err := service.pendingModUpdates(context.Background(), installed, []ModUpdateTarget{
		{ModID: " example ", VersionID: "older"},
		{ModID: "EXAMPLE", VersionID: "equal"},
		{ModID: "example", VersionID: "newer"},
	}, "1.20")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].VersionID != "newer" {
		t.Fatalf("pending = %+v", pending)
	}
	if vsmodpack.CompareVersions("v2.0.0+catalog", "2.0") != 0 {
		t.Fatal("test requires semantic equality")
	}
}

func TestPendingModUpdatesUsesHighestInstalledVersion(t *testing.T) {
	service := &CatalogService{catalog: policyVersionsCatalog{versions: []ModVersion{
		{ID: "middle", Version: "2.5.0"}, {ID: "new", Version: "4.0.0"},
	}}}
	installed := []InstalledMod{
		{Version: "2.0.0", Source: "moddb:example:first", Managed: true},
		{Version: "3.0.0", Source: "moddb:example:second", Managed: true},
	}
	pending, _, err := service.pendingModUpdates(context.Background(), installed, []ModUpdateTarget{
		{ModID: "example", VersionID: "middle"}, {ModID: "example", VersionID: "new"},
	}, "1.20")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].VersionID != "new" {
		t.Fatalf("pending = %+v", pending)
	}
}

func TestPendingModUpdatesKeepsInstalledIdentityForRevalidation(t *testing.T) {
	service := &CatalogService{catalog: policyVersionsCatalog{detailsID: "canonical", versions: []ModVersion{{ID: "new", Version: "2.0.0"}}}}
	installed := []InstalledMod{{Version: "1.0.0", Source: "moddb:alias:old", Managed: true}}
	pending, _, err := service.pendingModUpdates(context.Background(), installed, []ModUpdateTarget{{ModID: "alias", VersionID: "new"}}, "1.20")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].ModID != "alias" {
		t.Fatalf("pending = %+v", pending)
	}
	fresh, _, err := service.pendingModUpdates(context.Background(), installed, pending, "1.20")
	if err != nil || len(fresh) != 1 {
		t.Fatalf("fresh = %+v, error = %v", fresh, err)
	}
}

func TestCompatibleOnlyDoesNotSelectOlderRelease(t *testing.T) {
	report := ModUpdateReport{Mods: []vsmodpack.ModUpdate{{
		ModID: "compat", Status: vsmodpack.StatusUpdateAvailable, InstalledVersion: "2.0.0",
	}}}
	installed := []InstalledMod{{Source: "moddb:compat:installed", Managed: true, UpdatePolicy: UpdatePolicyCompatibleOnly}}
	applyUpdatePolicies(context.Background(), &report, installed, "1.20", policyVersionsCatalog{versions: []ModVersion{{
		ID: "old", Version: "1.0.0", GameVersions: []string{"1.20"}, DownloadURL: "https://example.test/old",
	}}})
	if report.Mods[0].Status != vsmodpack.StatusUpToDate || report.Mods[0].TargetVersionID != "" {
		t.Fatalf("older compatible release selected: %+v", report.Mods[0])
	}
}

func TestCompatibleOnlyFallbackUpdatesPrereleaseMetadata(t *testing.T) {
	report := ModUpdateReport{Mods: []vsmodpack.ModUpdate{{
		ModID: "compat", Status: vsmodpack.StatusUpdateAvailable, InstalledVersion: "2.0.0",
	}}}
	installed := []InstalledMod{{Source: "moddb:compat:installed", Managed: true, UpdatePolicy: UpdatePolicyCompatibleOnly}}
	applyUpdatePolicies(context.Background(), &report, installed, "1.20", policyVersionsCatalog{versions: []ModVersion{{
		ID: "beta", Version: "2.1.0-beta.1", ReleaseType: "beta", GameVersions: []string{"1.20"}, DownloadURL: "https://example.test/beta",
	}}})
	if update := report.Mods[0]; update.TargetVersionID != "beta" || !update.Compatible || !update.Prerelease {
		t.Fatalf("prerelease metadata not updated: %+v", update)
	}
}

type policyVersionsCatalog struct {
	detailsID string
	versions  []ModVersion
}

func (policyVersionsCatalog) List(context.Context) ([]ModSummary, error) { return nil, nil }
func (policyVersionsCatalog) Search(context.Context, ModSearchQuery) (ModSearchResult, error) {
	return ModSearchResult{}, nil
}
func (catalog policyVersionsCatalog) Get(_ context.Context, modID string) (ModDetails, error) {
	if catalog.detailsID != "" {
		modID = catalog.detailsID
	}
	return ModDetails{ModSummary: ModSummary{ID: modID}, Versions: catalog.versions}, nil
}
func (policyVersionsCatalog) ListTags(context.Context) ([]ModTag, error) { return nil, nil }

type policyTestCatalog struct{}

func (policyTestCatalog) List(context.Context) ([]ModSummary, error) { return nil, nil }
func (policyTestCatalog) Search(context.Context, ModSearchQuery) (ModSearchResult, error) {
	return ModSearchResult{}, nil
}
func (policyTestCatalog) Get(_ context.Context, modID string) (ModDetails, error) {
	return ModDetails{ModSummary: ModSummary{ID: modID}, Versions: []ModVersion{{ID: "new", Version: "2.0.0"}}}, nil
}
func (policyTestCatalog) ListTags(context.Context) ([]ModTag, error) { return nil, nil }

func TestInstalledReplacementKeepsPolicy(t *testing.T) {
	if NormalizeUpdatePolicy(UpdatePolicyPinned) != UpdatePolicyPinned {
		t.Fatal("pinned policy was not preserved")
	}
}

func TestLegacyIgnorePolicyBecomesPinned(t *testing.T) {
	if NormalizeUpdatePolicy("ignore") != UpdatePolicyPinned {
		t.Fatal("legacy ignore policy was not preserved as pinned")
	}
}

func TestPendingModUpdatesRespectReleaseChannel(t *testing.T) {
	service := &CatalogService{catalog: policyVersionsCatalog{versions: []ModVersion{
		{ID: "oldstable", Version: "1.9.8", ReleaseType: "stable"},
		{ID: "current", Version: "2.0.0-pre.5", ReleaseType: "beta"},
		{ID: "nextpre", Version: "2.0.0-pre.6", ReleaseType: "beta"},
		{ID: "nextstable", Version: "2.0.0", ReleaseType: "stable"},
	}}}
	installed := []InstalledMod{{Version: "2.0.0-pre.5", Source: "moddb:example:current", Managed: true}}
	pending, _, err := service.pendingModUpdates(context.Background(), installed, []ModUpdateTarget{
		{ModID: "example", VersionID: "oldstable"},
		{ModID: "example", VersionID: "nextpre"},
		{ModID: "example", VersionID: "nextstable"},
	}, "1.20")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 || pending[0].VersionID != "nextpre" || pending[1].VersionID != "nextstable" {
		t.Fatalf("pending = %+v", pending)
	}
}

func TestPendingModUpdatesRejectsPrereleaseForStableInstall(t *testing.T) {
	service := &CatalogService{catalog: policyVersionsCatalog{versions: []ModVersion{
		{ID: "stable", Version: "1.9.1", ReleaseType: "stable"},
		{ID: "beta", Version: "2.0.0-beta.1", ReleaseType: "beta"},
	}}}
	installed := []InstalledMod{{Version: "1.9.0", Source: "moddb:example:installed", Managed: true}}
	pending, _, err := service.pendingModUpdates(context.Background(), installed, []ModUpdateTarget{
		{ModID: "example", VersionID: "stable"},
		{ModID: "example", VersionID: "beta"},
	}, "1.20")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].VersionID != "stable" {
		t.Fatalf("pending = %+v", pending)
	}
}
