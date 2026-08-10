package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

const instanceColumns = `id, name, description, game_version_id, default_account_id,
	directory, cover_path, status, launch_arguments, last_played_at, created_at, updated_at`

func (s *SQLiteStore) ListInstances(ctx context.Context) ([]domain.Instance, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+instanceColumns+` FROM instances ORDER BY COALESCE(last_played_at, created_at) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var instances []domain.Instance
	for rows.Next() {
		instance, err := scanInstance(rows)
		if err != nil {
			return nil, err
		}
		instances = append(instances, instance)
	}
	return instances, rows.Err()
}

func scanInstance(row scanner) (domain.Instance, error) {
	var instance domain.Instance
	var account, cover, last sql.NullString
	var arguments, created, updated string
	err := row.Scan(&instance.ID, &instance.Name, &instance.Description, &instance.GameVersionID, &account,
		&instance.Directory, &cover, &instance.Status, &arguments, &last, &created, &updated)
	if account.Valid {
		instance.DefaultAccountID = &account.String
	}
	if cover.Valid {
		instance.CoverPath = &cover.String
	}
	instance.LastPlayedAt = parseTS(last)
	_ = json.Unmarshal([]byte(arguments), &instance.LaunchArguments)
	instance.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	instance.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return instance, err
}

func (s *SQLiteStore) GetInstance(ctx context.Context, id string) (domain.Instance, error) {
	instance, err := scanInstance(s.db.QueryRowContext(ctx, `SELECT `+instanceColumns+` FROM instances WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return instance, domain.NewError(domain.ErrInstanceNotFound, "Instance not found")
	}
	return instance, err
}

func (s *SQLiteStore) SaveInstance(ctx context.Context, instance domain.Instance) error {
	arguments, _ := json.Marshal(instance.LaunchArguments)
	_, err := s.db.ExecContext(ctx, `INSERT INTO instances(`+instanceColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, description=excluded.description,
		game_version_id=excluded.game_version_id, default_account_id=excluded.default_account_id,
		directory=excluded.directory, cover_path=excluded.cover_path, status=excluded.status,
		launch_arguments=excluded.launch_arguments, last_played_at=excluded.last_played_at, updated_at=excluded.updated_at`,
		instance.ID, instance.Name, instance.Description, instance.GameVersionID, instance.DefaultAccountID,
		instance.Directory, instance.CoverPath, instance.Status, string(arguments), optTS(instance.LastPlayedAt),
		ts(instance.CreatedAt), ts(instance.UpdatedAt))
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: instances.directory") {
		return domain.NewError(domain.ErrDirectoryConflict, "The directory is already used by another instance")
	}
	return err
}

func (s *SQLiteStore) DeleteInstance(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM instances WHERE id=?`, id)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return domain.NewError(domain.ErrInstanceNotFound, "Instance not found")
	}
	return nil
}

func (s *SQLiteStore) IsDirectoryUsed(ctx context.Context, path, except string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM instances WHERE directory=? AND id<>?`, path, except).Scan(&count)
	return count > 0, err
}

type relocationStatement struct {
	query string
	args  []any
}

func (s *SQLiteStore) RelocatePaths(ctx context.Context, oldRoot, newRoot string) error {
	prefixLength := len(oldRoot) + 1
	statements := []relocationStatement{
		{query: `UPDATE game_versions SET installation_dir=? || substr(installation_dir, ?),
			executable_path=? || substr(executable_path, ?) WHERE instr(installation_dir, ?)=1 OR instr(executable_path, ?)=1`,
			args: []any{newRoot, prefixLength, newRoot, prefixLength, oldRoot, oldRoot}},
		{query: `UPDATE instances SET directory=? || substr(directory, ?), cover_path=? || substr(cover_path, ?)
			WHERE instr(directory, ?)=1 OR instr(cover_path, ?)=1`,
			args: []any{newRoot, prefixLength, newRoot, prefixLength, oldRoot, oldRoot}},
		{query: `UPDATE installed_mods SET file_path=? || substr(file_path, ?) WHERE instr(file_path, ?)=1`,
			args: []any{newRoot, prefixLength, oldRoot}},
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}
