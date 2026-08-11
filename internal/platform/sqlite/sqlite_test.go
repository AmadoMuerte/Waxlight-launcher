package sqlite_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/platform/sqlite"
)

func TestLegacyAccountSchemaIsMigrated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE accounts (
		 id TEXT PRIMARY KEY, username TEXT NOT NULL, display_name TEXT NOT NULL,
		 status TEXT NOT NULL, is_default INTEGER NOT NULL DEFAULT 0,
		 last_authenticated TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	account := accounts.Account{
		ID:          "account-id",
		Username:    "Waxlighter",
		DisplayName: "Waxlighter",
		Email:       "player@example.com",
		UID:         "server-uid",
		Status:      accounts.StatusValid,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SaveAccount(context.Background(), account); err != nil {
		t.Fatal(err)
	}
	stored, err := store.GetAccount(context.Background(), account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Email != account.Email || stored.UID != account.UID {
		t.Fatalf("auth columns were not migrated: %#v", stored)
	}
}

func TestFavoriteServersPersistAndDelete(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "favorites.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().UTC().Round(0)
	server := domain.FavoriteServer{
		ID:        "server-id",
		Name:      "Cozy server",
		Address:   "example.org:42420",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.SaveFavoriteServer(context.Background(), server); err != nil {
		t.Fatal(err)
	}

	servers, err := store.ListFavoriteServers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || servers[0].Name != server.Name || servers[0].Address != server.Address {
		t.Fatalf("unexpected favorite servers: %#v", servers)
	}
	if err := store.DeleteFavoriteServer(context.Background(), server.ID); err != nil {
		t.Fatal(err)
	}
	servers, err = store.ListFavoriteServers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 0 {
		t.Fatalf("favorite server was not deleted: %#v", servers)
	}
}

func TestDatabaseRejectsSymlinkAndUsesOwnerOnlyPermissions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "waxlight.db")
	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("unexpected database permissions: %v, %v", info, err)
		}
	}
	target := filepath.Join(root, "target.db")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.db")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := sqlite.Open(link); err == nil {
		t.Fatal("database symlink was accepted")
	}
}

func TestOperationTitleLocalizationSurvivesPersistence(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "operations.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().UTC()
	operation := domain.Operation{
		ID:          "snapshot-1",
		Type:        "snapshot_create",
		Title:       "Creating snapshot",
		TitleKey:    "operation_creating_snapshot",
		TitleParams: map[string]string{"done": "3", "total": "5"},
		Status:      "running",
		Progress:    0.4,
		CreatedAt:   now,
	}
	if err := store.SaveOperation(context.Background(), operation); err != nil {
		t.Fatal(err)
	}

	// A fresh read must preserve the translation key and its parameters so the
	// UI never falls back to the English title after a reload.
	listed, err := store.ListOperations(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected one operation, got %d", len(listed))
	}
	loaded := listed[0]
	if loaded.TitleKey != "operation_creating_snapshot" {
		t.Fatalf("title key was lost: %q", loaded.TitleKey)
	}
	if loaded.TitleParams["done"] != "3" || loaded.TitleParams["total"] != "5" {
		t.Fatalf("title params were lost: %#v", loaded.TitleParams)
	}
	if loaded.Progress != 0.4 {
		t.Fatalf("progress was lost: %f", loaded.Progress)
	}
}

func TestFinishedOperationsCanBeDeletedWithoutTouchingActiveOnes(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "operations.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().UTC()
	operations := []domain.Operation{
		{ID: "running", Type: "download", Title: "Running", Status: "running", CreatedAt: now},
		{ID: "completed", Type: "download", Title: "Completed", Status: "completed", CreatedAt: now},
		{ID: "failed", Type: "download", Title: "Failed", Status: "failed", CreatedAt: now},
		{ID: "cancelled", Type: "download", Title: "Cancelled", Status: "cancelled", CreatedAt: now},
	}
	for _, operation := range operations {
		if err := store.SaveOperation(context.Background(), operation); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.DeleteFinishedOperation(context.Background(), "running"); err == nil {
		t.Fatal("active operation was deletable")
	}
	if err := store.DeleteFinishedOperation(context.Background(), "completed"); err != nil {
		t.Fatal(err)
	}
	removed, err := store.ClearFinishedOperations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Fatalf("expected two remaining finished operations to be removed, got %d", removed)
	}

	remaining, err := store.ListOperations(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].ID != "running" {
		t.Fatalf("unexpected remaining operations: %+v", remaining)
	}
}

func TestRelocatePathsRewritesStoredAbsolutePaths(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "waxlight.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	oldRoot := filepath.Join(string(filepath.Separator), "old", "waxlight")
	newRoot := filepath.Join(string(filepath.Separator), "new", "waxlight")

	now := time.Now().UTC()
	if err := store.SaveVersion(ctx, domain.GameVersion{
		ID:              "v1",
		Name:            "Test",
		Platform:        "linux",
		Architecture:    "amd64",
		InstallationDir: filepath.Join(oldRoot, "versions", "v1"),
		ExecutablePath:  filepath.Join(oldRoot, "versions", "v1", "game"),
		Status:          "installed",
		InstalledAt:     now,
	}); err != nil {
		t.Fatal(err)
	}
	cover := filepath.Join(oldRoot, "instances", "i1", "cover.png")
	if err := store.SaveInstance(ctx, domain.Instance{
		ID:              "i1",
		Name:            "Setup",
		GameVersionID:   "v1",
		Directory:       filepath.Join(oldRoot, "instances", "i1"),
		CoverPath:       &cover,
		Status:          "ready",
		LaunchArguments: []string{},
		CreatedAt:       now,
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.RelocatePaths(ctx, oldRoot, newRoot); err != nil {
		t.Fatal(err)
	}

	versions, err := store.ListVersions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected one version, got %d", len(versions))
	}
	wantInstall := filepath.Join(newRoot, "versions", "v1")
	if versions[0].InstallationDir != wantInstall {
		t.Fatalf("installation dir = %q, want %q", versions[0].InstallationDir, wantInstall)
	}
	if versions[0].ExecutablePath != filepath.Join(newRoot, "versions", "v1", "game") {
		t.Fatalf("executable path was not rewritten: %q", versions[0].ExecutablePath)
	}

	instances, err := store.ListInstances(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected one instance, got %d", len(instances))
	}
	if instances[0].Directory != filepath.Join(newRoot, "instances", "i1") {
		t.Fatalf("instance directory was not rewritten: %q", instances[0].Directory)
	}
	if instances[0].CoverPath == nil || *instances[0].CoverPath != filepath.Join(newRoot, "instances", "i1", "cover.png") {
		t.Fatalf("instance cover path was not rewritten: %v", instances[0].CoverPath)
	}
}

func TestRelocatePathsIsIdempotent(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "waxlight.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	oldRoot := filepath.Join(string(filepath.Separator), "old", "waxlight")
	newRoot := filepath.Join(string(filepath.Separator), "new", "waxlight")
	now := time.Now().UTC()
	if err := store.SaveVersion(ctx, domain.GameVersion{
		ID:              "v1",
		Name:            "Test",
		Platform:        "linux",
		Architecture:    "amd64",
		InstallationDir: filepath.Join(oldRoot, "versions", "v1"),
		ExecutablePath:  filepath.Join(oldRoot, "versions", "v1", "game"),
		Status:          "installed",
		InstalledAt:     now,
	}); err != nil {
		t.Fatal(err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		if err := store.RelocatePaths(ctx, oldRoot, newRoot); err != nil {
			t.Fatal(err)
		}
	}
	versions, err := store.ListVersions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if versions[0].InstallationDir != filepath.Join(newRoot, "versions", "v1") {
		t.Fatalf("path changed after repeated relocation: %q", versions[0].InstallationDir)
	}
}
