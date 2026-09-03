package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
)

// storedPath converts an absolute path inside the data root into a
// root-relative one. Paths outside the root (custom instance directories,
// linked local mod files) stay absolute.
func (s *SQLiteStore) storedPath(path string) string {
	if path == "" || !filepath.IsAbs(path) {
		return path
	}
	rel, err := filepath.Rel(s.dataRoot, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return rel
}

// resolvePath expands a stored path against the current data root. Absolute
// paths (custom directories outside the root) pass through unchanged.
func (s *SQLiteStore) resolvePath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(s.dataRoot, path)
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// relativizePathsUnder rewrites stored absolute paths under root to
// root-relative paths. It is idempotent: relative rows never carry the
// absolute root prefix, so they are left untouched.
func relativizePathsUnder(ctx context.Context, exec execer, root string) error {
	if root == "" {
		return nil
	}
	prefix := root + string(filepath.Separator)
	cut := len(prefix) + 1 // substr is 1-indexed; drop the leading separator too
	statements := []struct {
		table string
		query string
		args  []any
	}{
		{"game_versions", `UPDATE game_versions SET installation_dir=substr(installation_dir, ?) WHERE instr(installation_dir, ?)=1`, []any{cut, prefix}},
		{"game_versions", `UPDATE game_versions SET executable_path=substr(executable_path, ?) WHERE instr(executable_path, ?)=1`, []any{cut, prefix}},
		{"instances", `UPDATE instances SET directory=substr(directory, ?) WHERE instr(directory, ?)=1`, []any{cut, prefix}},
		{"instances", `UPDATE instances SET cover_path=substr(cover_path, ?) WHERE cover_path IS NOT NULL AND instr(cover_path, ?)=1`, []any{cut, prefix}},
		{"installed_mods", `UPDATE installed_mods SET file_path=substr(file_path, ?) WHERE instr(file_path, ?)=1`, []any{cut, prefix}},
	}
	for _, statement := range statements {
		var exists int
		if err := exec.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, statement.table).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			continue
		}
		if _, err := exec.ExecContext(ctx, statement.query, statement.args...); err != nil {
			return err
		}
	}
	return nil
}

// RelocatePaths converts stored paths under the previous and the current data
// root into root-relative paths. Since stored paths are relative, a data-root
// move needs no path rewriting: they resolve against the new root on read.
func (s *SQLiteStore) RelocatePaths(ctx context.Context, oldRoot, newRoot string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, root := range []string{oldRoot, newRoot} {
		if err := relativizePathsUnder(ctx, tx, root); err != nil {
			return err
		}
	}
	return tx.Commit()
}
