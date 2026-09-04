package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestFailedMigrationRollsBackSchemaAndVersion(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	original := baseMigrations
	baseMigrations = []migration{{version: 1, apply: func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `CREATE TABLE migration_will_rollback (id TEXT PRIMARY KEY)`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `THIS IS NOT VALID SQL`)
		return err
	}}}
	t.Cleanup(func() { baseMigrations = original })

	store := &SQLiteStore{db: db}
	if err := store.migrate(context.Background()); err == nil {
		t.Fatal("failed migration unexpectedly succeeded")
	}

	var tableCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='migration_will_rollback'`).Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if tableCount != 0 {
		t.Fatal("schema change from failed migration was not rolled back")
	}
	var versionCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&versionCount); err != nil {
		t.Fatal(err)
	}
	if versionCount != 0 {
		t.Fatal("failed migration version was recorded")
	}
}
