package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/securefs"
)

type SQLiteStore struct {
	db *sql.DB
}

func Open(path string) (*SQLiteStore, error) {
	if err := prepareDatabasePath(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &SQLiteStore{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := securefs.Apply(path, 0o600, false); err != nil {
		_ = db.Close()
		return nil, err
	}
	slog.Info("database opened and migrated", "file", filepath.Base(path))
	return store, nil
}

func prepareDatabasePath(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return errors.New("database path is not a regular file")
		}
		return securefs.Apply(path, 0o600, false)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return securefs.Apply(path, 0o600, false)
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

type scanner interface {
	Scan(...any) error
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func optTS(t *time.Time) any {
	if t == nil {
		return nil
	}
	return ts(*t)
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func parseTS(value sql.NullString) *time.Time {
	if !value.Valid {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil
	}
	return &t
}

func btoi(value bool) int {
	if value {
		return 1
	}
	return 0
}
