package mods_test

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDeleteModRemovesUnusedDependencies(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := fixture.createTestInstance(t, "Instance")
	installModWithDeps(t, fixture, instance.ID, "rootmod", "Root Mod", map[string]string{"libmod": "1.0"})
	installModWithDeps(t, fixture, instance.ID, "standalone", "Standalone", map[string]string{})
	installModWithDeps(t, fixture, instance.ID, "libmod", "Lib Mod", map[string]string{})
	mods, err := fixture.modsService.ListMods(ctx, instance.ID)
	if err != nil || len(mods) != 3 {
		t.Fatalf("expected three installed mods, got %#v, %v", mods, err)
	}
	root := installedModByName(mods, "Root Mod")
	lib := installedModByName(mods, "Lib Mod")
	if err := fixture.modsService.DeleteMod(ctx, root.ID, true); err != nil {
		t.Fatal(err)
	}
	remaining, err := fixture.modsService.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].Name != "Standalone" {
		t.Fatalf("expected only the standalone mod to remain, got %#v", remaining)
	}
	assertFileRemoved(t, lib.FilePath)
}
func TestDeleteModKeepsDependencyUsedByAnotherMod(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := fixture.createTestInstance(t, "Instance")
	installModWithDeps(t, fixture, instance.ID, "roota", "Root A", map[string]string{"sharedlib": "1.0"})
	installModWithDeps(t, fixture, instance.ID, "rootb", "Root B", map[string]string{"sharedlib": "1.0"})
	installModWithDeps(t, fixture, instance.ID, "sharedlib", "Shared Lib", map[string]string{})
	mods, err := fixture.modsService.ListMods(ctx, instance.ID)
	if err != nil || len(mods) != 3 {
		t.Fatalf("expected three installed mods, got %#v, %v", mods, err)
	}
	rootA := installedModByName(mods, "Root A")
	lib := installedModByName(mods, "Shared Lib")
	if err := fixture.modsService.DeleteMod(ctx, rootA.ID, true); err != nil {
		t.Fatal(err)
	}
	remaining, err := fixture.modsService.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 2 {
		t.Fatalf("shared dependency must be kept, got %#v", remaining)
	}
	assertFileExists(t, lib.FilePath)
	rootB := installedModByName(remaining, "Root B")
	if err := fixture.modsService.DeleteMod(ctx, rootB.ID, true); err != nil {
		t.Fatal(err)
	}
	finalMods, err := fixture.modsService.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(finalMods) != 0 {
		t.Fatalf("expected the shared dependency to be removed with its last user, got %#v", finalMods)
	}
	assertFileRemoved(t, lib.FilePath)
}
func TestDeleteModRemovesTransitiveDependencies(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := fixture.createTestInstance(t, "Instance")
	installModWithDeps(t, fixture, instance.ID, "rootmod", "Root Mod", map[string]string{"libmod": "1.0"})
	installModWithDeps(t, fixture, instance.ID, "libmod", "Lib Mod", map[string]string{"sublib": "1.0"})
	installModWithDeps(t, fixture, instance.ID, "sublib", "Sub Lib", map[string]string{})
	mods, err := fixture.modsService.ListMods(ctx, instance.ID)
	if err != nil || len(mods) != 3 {
		t.Fatalf("expected three installed mods, got %#v, %v", mods, err)
	}
	root := installedModByName(mods, "Root Mod")
	lib := installedModByName(mods, "Lib Mod")
	sublib := installedModByName(mods, "Sub Lib")
	if err := fixture.modsService.DeleteMod(ctx, root.ID, true); err != nil {
		t.Fatal(err)
	}
	remaining, err := fixture.modsService.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected all transitive dependencies to be removed, got %#v", remaining)
	}
	assertFileRemoved(t, lib.FilePath)
	assertFileRemoved(t, sublib.FilePath)
}
func TestModDeletePreviewListsUnusedDependencies(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := fixture.createTestInstance(t, "Instance")
	installModWithDeps(t, fixture, instance.ID, "rootmod", "Root Mod", map[string]string{"libmod": "1.0"})
	installModWithDeps(t, fixture, instance.ID, "other", "Other Mod", map[string]string{"libmod": "1.0"})
	installModWithDeps(t, fixture, instance.ID, "libmod", "Lib Mod", map[string]string{})
	mods, err := fixture.modsService.ListMods(ctx, instance.ID)
	if err != nil || len(mods) != 3 {
		t.Fatalf("expected three installed mods, got %#v, %v", mods, err)
	}
	root := installedModByName(mods, "Root Mod")
	other := installedModByName(mods, "Other Mod")
	// The shared dependency is not listed while another mod requires it.
	preview, err := fixture.modsService.ModDeletePreview(ctx, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if preview.ModID != root.ID || preview.ModName != "Root Mod" || len(preview.Dependencies) != 0 {
		t.Fatalf("unexpected preview for shared dependency: %#v", preview)
	}
	if preview.Dependencies == nil {
		t.Fatal("preview dependencies must be an empty slice, not nil")
	}
	// After the other user is gone, the dependency appears in the preview.
	if err := fixture.modsService.DeleteMod(ctx, other.ID, false); err != nil {
		t.Fatal(err)
	}
	preview, err = fixture.modsService.ModDeletePreview(ctx, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Dependencies) != 1 || preview.Dependencies[0].Name != "Lib Mod" {
		t.Fatalf("expected Lib Mod in the preview, got %#v", preview)
	}
}
func TestDeleteModWithoutDependenciesKeepsSiblings(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := fixture.createTestInstance(t, "Instance")
	installModWithDeps(t, fixture, instance.ID, "alpha", "Alpha", map[string]string{})
	installModWithDeps(t, fixture, instance.ID, "beta", "Beta", map[string]string{})
	mods, err := fixture.modsService.ListMods(ctx, instance.ID)
	if err != nil || len(mods) != 2 {
		t.Fatalf("expected two installed mods, got %#v, %v", mods, err)
	}
	alpha := installedModByName(mods, "Alpha")
	if err := fixture.modsService.DeleteMod(ctx, alpha.ID, false); err != nil {
		t.Fatal(err)
	}
	remaining, err := fixture.modsService.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].Name != "Beta" {
		t.Fatalf("expected only Beta to remain, got %#v", remaining)
	}
}
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}
func assertFileRemoved(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, stat error: %v", path, err)
	}
}
func installModWithDeps(
	t *testing.T,
	fixture testFixture,
	instanceID string,
	modID string,
	name string,
	dependencies map[string]string,
) {
	t.Helper()
	path := filepath.Join(fixture.root, modID+".zip")
	writeModZipWithDeps(t, path, modID, name, dependencies)
	if _, err := fixture.modsService.InstallModFile(context.Background(), instanceID, path, name, "1.0"); err != nil {
		t.Fatalf("install %s: %v", name, err)
	}
}
func writeModZipWithDeps(t *testing.T, path, modID, name string, dependencies map[string]string) {
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
		"version":      "1.0",
		"dependencies": dependencies,
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
