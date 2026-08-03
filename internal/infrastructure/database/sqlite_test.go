package database_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/database"
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

	store, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	account := domain.Account{
		ID:          "account-id",
		Username:    "Waxlighter",
		DisplayName: "Waxlighter",
		Email:       "player@example.com",
		UID:         "server-uid",
		Status:      domain.AccountStatusValid,
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

func TestDatabaseRejectsSymlinkAndUsesOwnerOnlyPermissions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "waxlight.db")
	store, err := database.Open(path)
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
	if _, err := database.Open(link); err == nil {
		t.Fatal("database symlink was accepted")
	}
}

func TestFinishedOperationsCanBeDeletedWithoutTouchingActiveOnes(t *testing.T) {
	store, err := database.Open(filepath.Join(t.TempDir(), "operations.db"))
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
