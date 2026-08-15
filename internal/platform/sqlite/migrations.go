package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

const currentSchemaVersion = 6

type migration struct {
	version int
	apply   func(context.Context, *sql.Tx) error
}

var migrations = []migration{
	{version: 1, apply: createBaseSchema},
	{version: 2, apply: addAuthenticationAndOperationTitles},
	{version: 3, apply: addAccountUIDIndex},
	{version: 4, apply: addLastKnownGood},
	{version: 5, apply: addFavoriteServers},
	{version: 6, apply: addInstanceGameClient},
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	applied := make(map[int]bool)
	rows, err := s.db.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan schema version: %w", err)
		}
		if version > currentSchemaVersion {
			_ = rows.Close()
			return fmt.Errorf("database schema version %d is newer than supported version %d", version, currentSchemaVersion)
		}
		applied[version] = true
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range migrations {
		if applied[item.version] {
			continue
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", item.version, err)
		}
		if err := item.apply(ctx, tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", item.version, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES (?, datetime('now'))`, item.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", item.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", item.version, err)
		}
	}
	return nil
}

func createBaseSchema(ctx context.Context, tx *sql.Tx) error {
	const schema = `
CREATE TABLE IF NOT EXISTS accounts (
 id TEXT PRIMARY KEY, username TEXT NOT NULL, display_name TEXT NOT NULL, email TEXT NOT NULL DEFAULT '',
 uid TEXT NOT NULL DEFAULT '', status TEXT NOT NULL, is_default INTEGER NOT NULL DEFAULT 0,
 last_validated_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS game_versions (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, channel TEXT NOT NULL, platform TEXT NOT NULL, architecture TEXT NOT NULL,
 installation_dir TEXT NOT NULL, executable_path TEXT NOT NULL, status TEXT NOT NULL, installed_at TEXT NOT NULL,
 verified_at TEXT, size_bytes INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS instances (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT NOT NULL, game_version_id TEXT NOT NULL,
 game_client TEXT NOT NULL DEFAULT 'vanilla', default_account_id TEXT, directory TEXT NOT NULL UNIQUE, cover_path TEXT, status TEXT NOT NULL,
 launch_arguments TEXT NOT NULL DEFAULT '[]', last_played_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 FOREIGN KEY(game_version_id) REFERENCES game_versions(id), FOREIGN KEY(default_account_id) REFERENCES accounts(id) ON DELETE SET NULL
);
CREATE TABLE IF NOT EXISTS installed_mods (
 id TEXT PRIMARY KEY, instance_id TEXT NOT NULL, name TEXT NOT NULL, version TEXT NOT NULL, file_name TEXT NOT NULL,
 file_path TEXT NOT NULL, enabled INTEGER NOT NULL, managed INTEGER NOT NULL, source TEXT NOT NULL, size_bytes INTEGER NOT NULL,
 installed_at TEXT NOT NULL, updated_at TEXT NOT NULL, FOREIGN KEY(instance_id) REFERENCES instances(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS play_sessions (
 id TEXT PRIMARY KEY, instance_id TEXT NOT NULL, account_id TEXT, version_id TEXT NOT NULL, process_id INTEGER,
 started_at TEXT NOT NULL, ended_at TEXT, duration_sec INTEGER NOT NULL DEFAULT 0, exit_code INTEGER,
 crashed INTEGER NOT NULL DEFAULT 0, recovered INTEGER NOT NULL DEFAULT 0,
 FOREIGN KEY(instance_id) REFERENCES instances(id) ON DELETE CASCADE, FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE SET NULL
);
CREATE TABLE IF NOT EXISTS operations (
 id TEXT PRIMARY KEY, type TEXT NOT NULL, resource_id TEXT, title TEXT NOT NULL, status TEXT NOT NULL,
 progress REAL NOT NULL, current_bytes INTEGER NOT NULL, total_bytes INTEGER NOT NULL, bytes_per_second INTEGER NOT NULL,
 error_code TEXT, error_message TEXT, created_at TEXT NOT NULL, started_at TEXT, finished_at TEXT
);
CREATE TABLE IF NOT EXISTS app_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);`
	_, err := tx.ExecContext(ctx, schema)
	return err
}

func addAuthenticationAndOperationTitles(ctx context.Context, tx *sql.Tx) error {
	columns := []struct {
		table, name, definition string
	}{
		{table: "accounts", name: "email", definition: "TEXT NOT NULL DEFAULT ''"},
		{table: "accounts", name: "uid", definition: "TEXT NOT NULL DEFAULT ''"},
		{table: "accounts", name: "last_validated_at", definition: "TEXT"},
		{table: "operations", name: "title_key", definition: "TEXT"},
		{table: "operations", name: "title_params", definition: "TEXT"},
	}
	for _, column := range columns {
		if err := ensureColumn(ctx, tx, column.table, column.name, column.definition); err != nil {
			return err
		}
	}
	return nil
}

func addAccountUIDIndex(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS accounts_uid_lookup ON accounts(uid) WHERE uid <> ''`)
	return err
}

func addLastKnownGood(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS last_known_good (
	 instance_id TEXT PRIMARY KEY, recorded_at TEXT NOT NULL, game_version TEXT NOT NULL,
	 snapshot_id TEXT, mods TEXT NOT NULL,
	 FOREIGN KEY(instance_id) REFERENCES instances(id) ON DELETE CASCADE
	)`)
	return err
}

func addFavoriteServers(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS favorite_servers (
	 id TEXT PRIMARY KEY, name TEXT NOT NULL, address TEXT NOT NULL, instance_id TEXT,
	 created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
	 FOREIGN KEY(instance_id) REFERENCES instances(id) ON DELETE SET NULL
	)`)
	return err
}

func addInstanceGameClient(ctx context.Context, tx *sql.Tx) error {
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='instances'`).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return nil
	}
	return ensureColumn(ctx, tx, "instances", "game_client", "TEXT NOT NULL DEFAULT 'vanilla'")
}

func ensureColumn(ctx context.Context, tx *sql.Tx, table, column, definition string) error {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return err
		}
		found = found || name == column
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = tx.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition)
	return err
}
