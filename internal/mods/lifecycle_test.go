package mods_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalModLifecycle(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	instance := fixture.createTestInstance(t, "Modded")

	sourcePath := filepath.Join(fixture.root, "sample.zip")
	if err := os.WriteFile(sourcePath, []byte("mod"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := fixture.modsService.InstallModFile(
		ctx,
		instance.ID,
		sourcePath,
		"Sample",
		"1.0",
	)
	if err != nil {
		t.Fatal(err)
	}

	installedMods, err := fixture.modsService.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(installedMods) != 1 {
		t.Fatalf("expected one installed mod, got %d", len(installedMods))
	}
	if installedMods[0].Name != "Sample" || installedMods[0].Version != "1.0" {
		t.Fatalf("stored mod metadata was replaced during scan: %#v", installedMods[0])
	}

	disabledMod, err := fixture.modsService.SetModEnabled(ctx, installedMods[0].ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if disabledMod.Enabled {
		t.Fatal("the mod should be disabled")
	}
	if directoryName := filepath.Base(filepath.Dir(disabledMod.FilePath)); directoryName != "ModsDisabled" {
		t.Fatalf("unexpected disabled mod directory %q", directoryName)
	}

	if err := fixture.modsService.DeleteMod(ctx, disabledMod.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(disabledMod.FilePath); !os.IsNotExist(err) {
		t.Fatal("the deleted mod file still exists")
	}
}

func TestInstallModFilesBatch(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	instance := fixture.createTestInstance(t, "Batch")

	makeMod := func(name string) string {
		path := filepath.Join(fixture.root, name+".zip")
		if err := os.WriteFile(path, []byte("mod"), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	first := makeMod("first")
	second := makeMod("second")
	unsupported := filepath.Join(fixture.root, "not-a-mod.txt")
	if err := os.WriteFile(unsupported, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := fixture.modsService.InstallModFiles(ctx, instance.ID, []string{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Installed) != 2 || len(result.Skipped) != 0 || len(result.Failed) != 0 {
		t.Fatalf("unexpected batch result: %#v", result)
	}

	duplicateResult, err := fixture.modsService.InstallModFiles(
		ctx,
		instance.ID,
		[]string{first, second},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(duplicateResult.Installed) != 0 {
		t.Fatalf("expected no new installs, got %#v", duplicateResult.Installed)
	}
	if len(duplicateResult.Skipped) != 2 {
		t.Fatalf("expected two skipped duplicates, got %#v", duplicateResult.Skipped)
	}
	if len(duplicateResult.Failed) != 0 {
		t.Fatalf("expected no failures, got %#v", duplicateResult.Failed)
	}

	partialResult, err := fixture.modsService.InstallModFiles(
		ctx,
		instance.ID,
		[]string{first, unsupported},
	)
	if err == nil {
		t.Fatal("expected an error when nothing could be installed")
	}
	if len(partialResult.Installed) != 0 || len(partialResult.Skipped) != 1 ||
		len(partialResult.Failed) != 1 || partialResult.Failed[0].Path != unsupported {
		t.Fatalf("unexpected partial result: %#v", partialResult)
	}

	installedMods, err := fixture.modsService.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(installedMods) != 2 {
		t.Fatalf("expected two installed mods, got %d", len(installedMods))
	}

	allFailResult, err := fixture.modsService.InstallModFiles(ctx, instance.ID, []string{unsupported})
	if err == nil {
		t.Fatal("expected an error when nothing could be installed")
	}
	if len(allFailResult.Installed) != 0 || len(allFailResult.Failed) != 1 {
		t.Fatalf("unexpected all-fail result: %#v", allFailResult)
	}
}

func TestListModsReconcilesFilesAddedOutsideLauncher(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := fixture.createTestInstance(t, "Imported")

	archivePath := filepath.Join(instance.Directory, "Mods", "smithingplus.zip")
	writeVintageStoryMod(t, archivePath, `{"modid":"smithingplus","name":"Smithing Plus","version":"2.4.1"}`)
	disabledPath := filepath.Join(instance.Directory, "ModsDisabled", "utility.dll")
	if err := os.WriteFile(disabledPath, []byte("mod"), 0o644); err != nil {
		t.Fatal(err)
	}

	installedMods, err := fixture.modsService.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(installedMods) != 2 {
		t.Fatalf("expected two imported mods, got %#v", installedMods)
	}
	if installedMods[0].Name != "Smithing Plus" || installedMods[0].Version != "2.4.1" || !installedMods[0].Enabled || installedMods[0].Managed {
		t.Fatalf("unexpected imported archive: %#v", installedMods[0])
	}
	importedID := installedMods[0].ID

	movedPath := filepath.Join(instance.Directory, "ModsDisabled", filepath.Base(archivePath))
	if err := os.Rename(archivePath, movedPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(disabledPath); err != nil {
		t.Fatal(err)
	}
	installedMods, err = fixture.modsService.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(installedMods) != 1 || installedMods[0].ID != importedID || installedMods[0].Enabled || installedMods[0].FilePath != movedPath {
		t.Fatalf("filesystem changes were not reconciled: %#v", installedMods)
	}
	persisted, err := fixture.repository.ListMods(ctx, instance.ID)
	if err != nil || len(persisted) != 1 {
		t.Fatalf("unexpected persisted mods: %#v, %v", persisted, err)
	}
}

func TestListModsKeepsRecordsWhenInstanceDirectoryMissing(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance := fixture.createTestInstance(t, "Missing")

	archivePath := filepath.Join(instance.Directory, "Mods", "smithingplus.zip")
	writeVintageStoryMod(t, archivePath, `{"modid":"smithingplus","name":"Smithing Plus","version":"2.4.1"}`)
	installedMods, err := fixture.modsService.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(installedMods) != 1 {
		t.Fatalf("expected one installed mod, got %#v", installedMods)
	}

	if err := os.RemoveAll(instance.Directory); err != nil {
		t.Fatal(err)
	}
	kept, err := fixture.modsService.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 || kept[0].ID != installedMods[0].ID {
		t.Fatalf("stored mod records must survive a missing instance directory: %#v", kept)
	}
	if _, err := os.Stat(instance.Directory); !os.IsNotExist(err) {
		t.Fatal("listing mods must not recreate the instance directory")
	}
}
