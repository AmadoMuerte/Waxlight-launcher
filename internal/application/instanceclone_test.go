package application_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/filesystem"
	"github.com/waxlight/waxlight-launcher/internal/instances"
)

func TestCloneInstanceCopiesFilesAndModsWithoutSavesOrLogs(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	fixture.service.ConfigureAuthentication(
		newTestAccountService(fixture.store, &fakeAuthClient{}, newMemorySecretStore(), fixture.service.ClearAccountFromInstances),
		filesystem.ClientSettingsService{},
	)

	account := accounts.Account{
		ID:          "acc-1",
		Username:    "gasada",
		DisplayName: "gasada",
		Email:       "gasada@example.com",
		UID:         "uid-123",
		Status:      accounts.StatusValid,
	}
	if err := fixture.store.SaveAccount(ctx, account); err != nil {
		t.Fatal(err)
	}
	accountID := account.ID

	source, err := fixture.service.CreateInstance(ctx, instances.CreateInput{
		Name:             "Warm home",
		Description:      "A cozy base",
		GameVersionID:    "1.20",
		DefaultAccountID: &accountID,
		LaunchArguments:  []string{"--tracelog", "--server"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(source.Directory, "Mods", "local-mod.zip"), []byte("local"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source.Directory, "ModsDisabled", "disabled-mod.zip"), []byte("disabled"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source.Directory, "Config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source.Directory, "Config", "mymod.json"), []byte(`{"x":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source.Directory, "clientsettings.json"), []byte(`{"stringsettings":{"sessionkey":"TOP_SECRET","playername":"gasada","fov":80}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source.Directory, "SaveGame", "world"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source.Directory, "SaveGame", "world", "data.txt"), []byte("save"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source.Directory, "Logs", "game.log"), []byte("log"), 0o600); err != nil {
		t.Fatal(err)
	}

	mods, err := fixture.service.ListMods(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, mod := range mods {
		if mod.FileName == "local-mod.zip" {
			mod.Source = "moddb:123:456"
			mod.Managed = true
			if err := fixture.store.SaveMod(ctx, mod); err != nil {
				t.Fatal(err)
			}
		}
	}

	clone, err := fixture.service.CloneInstance(ctx, source.ID, "Clone of Warm home")
	if err != nil {
		t.Fatal(err)
	}

	if clone.ID == source.ID {
		t.Fatal("clone must have a new ID")
	}
	if clone.Name != "Clone of Warm home" {
		t.Fatalf("unexpected clone name %q", clone.Name)
	}
	if clone.Description != source.Description {
		t.Fatalf("description was not copied: %q", clone.Description)
	}
	if clone.GameVersionID != source.GameVersionID {
		t.Fatalf("game version was not copied: %q", clone.GameVersionID)
	}
	if clone.DefaultAccountID == nil || *clone.DefaultAccountID != accountID {
		t.Fatalf("default account was not copied: %v", clone.DefaultAccountID)
	}
	if strings.Join(clone.LaunchArguments, " ") != "--tracelog --server" {
		t.Fatalf("launch arguments were not copied: %v", clone.LaunchArguments)
	}
	if clone.Directory == source.Directory {
		t.Fatal("clone must use a fresh directory")
	}

	marker, err := os.ReadFile(filepath.Join(clone.Directory, ".waxlight-instance"))
	if err != nil {
		t.Fatal(err)
	}
	if string(marker) != clone.ID {
		t.Fatalf("clone marker contains %q, expected %q", string(marker), clone.ID)
	}

	for _, expected := range []string{"Mods/local-mod.zip", "ModsDisabled/disabled-mod.zip", "Config/mymod.json"} {
		if _, err := os.Stat(filepath.Join(clone.Directory, expected)); err != nil {
			t.Fatalf("expected %q in the clone: %v", expected, err)
		}
	}
	if _, err := os.Stat(filepath.Join(clone.Directory, "SaveGame", "world", "data.txt")); !os.IsNotExist(err) {
		t.Fatalf("clone must not copy save data, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(clone.Directory, "Logs", "game.log")); !os.IsNotExist(err) {
		t.Fatalf("clone must not copy logs, stat error: %v", err)
	}

	settings, err := os.ReadFile(filepath.Join(clone.Directory, "clientsettings.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"TOP_SECRET", "sessionsignature", "playeruid", "playername"} {
		if strings.Contains(string(settings), forbidden) {
			t.Fatalf("clone client settings leaked %q", forbidden)
		}
	}
	sourceSettings, err := os.ReadFile(filepath.Join(source.Directory, "clientsettings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sourceSettings), "TOP_SECRET") {
		t.Fatal("source instance client settings must be left untouched")
	}

	cloneMods, err := fixture.store.ListMods(ctx, clone.ID)
	if err != nil {
		t.Fatal(err)
	}
	sourceMods, err := fixture.store.ListMods(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cloneMods) != len(sourceMods) {
		t.Fatalf("expected %d mods on the clone, got %d", len(sourceMods), len(cloneMods))
	}
	for _, mod := range cloneMods {
		if mod.InstanceID != clone.ID {
			t.Fatalf("clone mod row belongs to %q", mod.InstanceID)
		}
		if mod.FileName == "local-mod.zip" && (mod.Source != "moddb:123:456" || !mod.Managed) {
			t.Fatalf("catalog mod metadata was not preserved: %+v", mod)
		}
		if !strings.HasPrefix(mod.FilePath, clone.Directory) {
			t.Fatalf("clone mod path %q is outside the clone directory", mod.FilePath)
		}
	}
}
