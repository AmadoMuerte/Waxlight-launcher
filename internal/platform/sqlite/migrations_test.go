package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/waxlight/waxlight-launcher/internal/platform/sqlite"
)

func TestLegacySchemaMigrationPreservesDataAndAddsCurrentColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db := openRawDatabase(t, path)
	_, err := db.Exec(`
		CREATE TABLE accounts (
		 id TEXT PRIMARY KEY, username TEXT NOT NULL, display_name TEXT NOT NULL,
		 status TEXT NOT NULL, is_default INTEGER NOT NULL DEFAULT 0,
		 last_authenticated TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		);
		CREATE TABLE operations (
		 id TEXT PRIMARY KEY, type TEXT NOT NULL, resource_id TEXT, title TEXT NOT NULL, status TEXT NOT NULL,
		 progress REAL NOT NULL, current_bytes INTEGER NOT NULL, total_bytes INTEGER NOT NULL,
		 bytes_per_second INTEGER NOT NULL, error_code TEXT, error_message TEXT, created_at TEXT NOT NULL,
		 started_at TEXT, finished_at TEXT
		);
		INSERT INTO accounts VALUES ('legacy-account', 'player', 'Player', 'valid', 1, NULL,
		 '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z');
		INSERT INTO operations VALUES ('legacy-operation', 'download', NULL, 'Downloading', 'running',
		 0.5, 10, 20, 2, NULL, NULL, '2024-01-01T00:00:00Z', NULL, NULL);
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

	accounts, err := store.ListAccounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].ID != "legacy-account" || accounts[0].Username != "player" {
		t.Fatalf("legacy account data changed: %#v", accounts)
	}
	operations, err := store.ListOperations(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(operations) != 1 || operations[0].ID != "legacy-operation" || operations[0].Title != "Downloading" {
		t.Fatalf("legacy operation data changed: %#v", operations)
	}

	assertColumns(t, path, "accounts", "email", "uid", "last_validated_at")
	assertColumns(t, path, "operations", "title_key", "title_params")
	assertIndex(t, path, "accounts_uid_lookup")
	assertColumns(t, path, "instances", "game_client")
	assertMigrationVersions(t, path, 1, 2, 3, 4, 5, 6)
}

func TestVersionedMigrationContinuesFromLegacyVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "versioned.db")
	db := openRawDatabase(t, path)
	_, err := db.Exec(`
		CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
		CREATE TABLE accounts (
		 id TEXT PRIMARY KEY, username TEXT NOT NULL, display_name TEXT NOT NULL,
		 email TEXT NOT NULL DEFAULT '', uid TEXT NOT NULL DEFAULT '', status TEXT NOT NULL,
		 is_default INTEGER NOT NULL DEFAULT 0, last_validated_at TEXT,
		 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		);
		CREATE TABLE operations (
		 id TEXT PRIMARY KEY, type TEXT NOT NULL, resource_id TEXT, title TEXT NOT NULL, status TEXT NOT NULL,
		 progress REAL NOT NULL, current_bytes INTEGER NOT NULL, total_bytes INTEGER NOT NULL,
		 bytes_per_second INTEGER NOT NULL, error_code TEXT, error_message TEXT, created_at TEXT NOT NULL,
		 started_at TEXT, finished_at TEXT, title_key TEXT, title_params TEXT
		);
		INSERT INTO schema_migrations(version, applied_at) VALUES (1, datetime('now')), (2, datetime('now'));
		INSERT INTO accounts(id, username, display_name, status, created_at, updated_at)
		 VALUES ('kept', 'kept', 'Kept', 'valid', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z');
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
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	assertIndex(t, path, "accounts_uid_lookup")
	assertMigrationVersions(t, path, 1, 2, 3, 4, 5, 6)

	db = openRawDatabase(t, path)
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM accounts WHERE id='kept'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("versioned migration did not preserve account data")
	}
}

func TestGameClientMigrationBackfillsVanilla(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game-client.db")
	db := openRawDatabase(t, path)
	_, err := db.Exec(`
		CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
		CREATE TABLE instances (
		 id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT NOT NULL, game_version_id TEXT NOT NULL,
		 default_account_id TEXT, directory TEXT NOT NULL UNIQUE, cover_path TEXT, status TEXT NOT NULL,
		 launch_arguments TEXT NOT NULL DEFAULT '[]', last_played_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		);
		INSERT INTO schema_migrations(version, applied_at) VALUES
		 (1, datetime('now')), (2, datetime('now')), (3, datetime('now')),
		 (4, datetime('now')), (5, datetime('now'));
		INSERT INTO instances(id, name, description, game_version_id, directory, status, created_at, updated_at)
		 VALUES ('legacy', 'Legacy', '', '1.22.5', '/legacy', 'ready', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
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
	instance, err := store.GetInstance(context.Background(), "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if instance.GameClient != "vanilla" {
		t.Fatalf("migrated game client = %q", instance.GameClient)
	}
}

func openRawDatabase(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertColumns(t *testing.T, path, table string, wanted ...string) {
	t.Helper()
	db := openRawDatabase(t, path)
	defer db.Close()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := map[string]bool{}
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		found[name] = true
	}
	for _, column := range wanted {
		if !found[column] {
			t.Errorf("%s.%s was not created", table, column)
		}
	}
}

func assertIndex(t *testing.T, path, name string) {
	t.Helper()
	db := openRawDatabase(t, path)
	defer db.Close()
	var sqlText string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='index' AND name=?`, name).Scan(&sqlText); err != nil {
		t.Fatalf("index %s was not created: %v", name, err)
	}
	if sqlText == "" {
		t.Fatalf("index %s has no definition", name)
	}
}

func assertMigrationVersions(t *testing.T, path string, wanted ...int) {
	t.Helper()
	db := openRawDatabase(t, path)
	defer db.Close()
	rows, err := db.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []int
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			t.Fatal(err)
		}
		got = append(got, version)
	}
	if len(got) != len(wanted) {
		t.Fatalf("migration versions = %v, want %v", got, wanted)
	}
	for i := range wanted {
		if got[i] != wanted[i] {
			t.Fatalf("migration versions = %v, want %v", got, wanted)
		}
	}
}
