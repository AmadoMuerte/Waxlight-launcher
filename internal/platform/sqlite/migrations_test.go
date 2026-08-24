package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/sqlite"
	_ "github.com/mattn/go-sqlite3"
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
	assertColumns(t, path, "instances", "game_client", "environment_variables", "is_pinned")
	assertColumns(t, path, "installed_mods", "update_policy")
	assertMigrationVersions(t, path, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
}

func TestV035SchemaMigrationPreservesOperations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v0.3.5.db")
	db := openRawDatabase(t, path)
	if _, err := db.Exec(affectedV035Schema); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	operations, err := store.ListOperations(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(operations) != 1 || operations[0].ID != "legacy-operation" || operations[0].Title != "Downloading" {
		t.Fatalf("legacy operation data changed: %#v", operations)
	}
	if operations[0].TitleKey != "" || operations[0].TitleParams != nil {
		t.Fatalf("legacy operation localization defaults changed: %#v", operations[0])
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	assertColumns(t, path, "operations", "id", "type", "resource_id", "title", "status", "progress",
		"current_bytes", "total_bytes", "bytes_per_second", "error_code", "error_message", "created_at",
		"started_at", "finished_at", "title_key", "title_params")
	assertMigrationVersions(t, path, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10)

	store, err = sqlite.Open(path)
	if err != nil {
		t.Fatalf("repeated initialization failed: %v", err)
	}
	defer store.Close()
	if _, err := store.ListOperations(context.Background(), 10); err != nil {
		t.Fatalf("list operations after repeated initialization: %v", err)
	}
}

func TestOperationTitleMigrationHandlesFreshAndPartialSchemas(t *testing.T) {
	t.Run("fresh", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "fresh.db")
		store, err := sqlite.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.ListOperations(context.Background(), 10); err != nil {
			t.Fatal(err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		assertColumns(t, path, "operations", "title_key", "title_params")
	})

	t.Run("title key already exists", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "partial.db")
		db := openRawDatabase(t, path)
		if _, err := db.Exec(affectedV035Schema + `ALTER TABLE operations ADD COLUMN title_key TEXT;`); err != nil {
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
		if _, err := store.ListOperations(context.Background(), 10); err != nil {
			t.Fatal(err)
		}
		assertColumns(t, path, "operations", "title_key", "title_params")
	})
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
	assertMigrationVersions(t, path, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10)

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
		CREATE TABLE operations (
		 id TEXT PRIMARY KEY, type TEXT NOT NULL, resource_id TEXT, title TEXT NOT NULL, status TEXT NOT NULL,
		 progress REAL NOT NULL, current_bytes INTEGER NOT NULL, total_bytes INTEGER NOT NULL,
		 bytes_per_second INTEGER NOT NULL, error_code TEXT, error_message TEXT, created_at TEXT NOT NULL,
		 started_at TEXT, finished_at TEXT, title_key TEXT, title_params TEXT
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
	if instance.IsPinned {
		t.Fatal("legacy instance was migrated as pinned")
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

// affectedV035Schema is the production state after a v0.3.3 database, with
// migrations 1-3 recorded, was opened by v0.3.5 and advanced to version 5.
const affectedV035Schema = `
CREATE TABLE accounts (
 id TEXT PRIMARY KEY, username TEXT NOT NULL, display_name TEXT NOT NULL, email TEXT NOT NULL DEFAULT '',
 uid TEXT NOT NULL DEFAULT '', status TEXT NOT NULL, is_default INTEGER NOT NULL DEFAULT 0,
 last_validated_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE game_versions (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, channel TEXT NOT NULL, platform TEXT NOT NULL, architecture TEXT NOT NULL,
 installation_dir TEXT NOT NULL, executable_path TEXT NOT NULL, status TEXT NOT NULL, installed_at TEXT NOT NULL,
 verified_at TEXT, size_bytes INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE instances (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT NOT NULL, game_version_id TEXT NOT NULL,
 default_account_id TEXT, directory TEXT NOT NULL UNIQUE, cover_path TEXT, status TEXT NOT NULL,
 launch_arguments TEXT NOT NULL DEFAULT '[]', last_played_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 FOREIGN KEY(game_version_id) REFERENCES game_versions(id), FOREIGN KEY(default_account_id) REFERENCES accounts(id) ON DELETE SET NULL
);
CREATE TABLE installed_mods (
 id TEXT PRIMARY KEY, instance_id TEXT NOT NULL, name TEXT NOT NULL, version TEXT NOT NULL, file_name TEXT NOT NULL,
 file_path TEXT NOT NULL, enabled INTEGER NOT NULL, managed INTEGER NOT NULL, source TEXT NOT NULL, size_bytes INTEGER NOT NULL,
 installed_at TEXT NOT NULL, updated_at TEXT NOT NULL, FOREIGN KEY(instance_id) REFERENCES instances(id) ON DELETE CASCADE
);
CREATE TABLE play_sessions (
 id TEXT PRIMARY KEY, instance_id TEXT NOT NULL, account_id TEXT, version_id TEXT NOT NULL, process_id INTEGER,
 started_at TEXT NOT NULL, ended_at TEXT, duration_sec INTEGER NOT NULL DEFAULT 0, exit_code INTEGER,
 crashed INTEGER NOT NULL DEFAULT 0, recovered INTEGER NOT NULL DEFAULT 0,
 FOREIGN KEY(instance_id) REFERENCES instances(id) ON DELETE CASCADE, FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE SET NULL
);
CREATE TABLE operations (
 id TEXT PRIMARY KEY, type TEXT NOT NULL, resource_id TEXT, title TEXT NOT NULL, status TEXT NOT NULL,
 progress REAL NOT NULL, current_bytes INTEGER NOT NULL, total_bytes INTEGER NOT NULL, bytes_per_second INTEGER NOT NULL,
 error_code TEXT, error_message TEXT, created_at TEXT NOT NULL, started_at TEXT, finished_at TEXT
);
CREATE TABLE last_known_good (
 instance_id TEXT PRIMARY KEY, recorded_at TEXT NOT NULL, game_version TEXT NOT NULL,
 snapshot_id TEXT, mods TEXT NOT NULL,
 FOREIGN KEY(instance_id) REFERENCES instances(id) ON DELETE CASCADE
);
CREATE TABLE favorite_servers (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, address TEXT NOT NULL, instance_id TEXT,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 FOREIGN KEY(instance_id) REFERENCES instances(id) ON DELETE SET NULL
);
CREATE TABLE app_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
CREATE INDEX accounts_uid_lookup ON accounts(uid) WHERE uid <> '';
INSERT INTO schema_migrations(version, applied_at) VALUES
 (1, datetime('now')), (2, datetime('now')), (3, datetime('now')), (4, datetime('now')), (5, datetime('now'));
INSERT INTO operations VALUES ('legacy-operation', 'download', NULL, 'Downloading', 'completed',
 1, 20, 20, 0, NULL, NULL, '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z', '2024-01-01T00:01:00Z');
`
