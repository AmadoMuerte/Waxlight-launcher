package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/securefs"
)

type SQLiteStore struct {
	db *sql.DB
}

const saveAccountSQL = `
	INSERT INTO accounts(
		id, username, display_name, email, uid, status, is_default,
		last_validated_at, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		username = excluded.username,
		display_name = excluded.display_name,
		email = excluded.email,
		uid = excluded.uid,
		status = excluded.status,
		is_default = excluded.is_default,
		last_validated_at = excluded.last_validated_at,
		updated_at = excluded.updated_at
`

func Open(path string) (*SQLiteStore, error) {
	if err := prepareDatabasePath(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &SQLiteStore{db: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	if err := securefs.Apply(path, 0o600, false); err != nil {
		db.Close()
		return nil, err
	}
	slog.Info("database opened and migrated", "file", filepath.Base(path))
	return s, nil
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

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
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
 default_account_id TEXT, directory TEXT NOT NULL UNIQUE, cover_path TEXT, status TEXT NOT NULL,
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
CREATE TABLE IF NOT EXISTS app_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (1, datetime('now'));
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return err
	}
	for _, column := range []struct {
		name       string
		definition string
	}{
		{name: "email", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "uid", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "last_validated_at", definition: "TEXT"},
	} {
		if err := s.ensureColumn(ctx, "accounts", column.name, column.definition); err != nil {
			return err
		}
	}
	for _, column := range []struct {
		name       string
		definition string
	}{
		{name: "title_key", definition: "TEXT"},
		{name: "title_params", definition: "TEXT"},
	} {
		if err := s.ensureColumn(ctx, "operations", column.name, column.definition); err != nil {
			return err
		}
	}
	_, err := s.db.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (2, datetime('now'))`,
	)
	if err != nil {
		return err
	}
	if _, err = s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS accounts_uid_lookup ON accounts(uid) WHERE uid <> ''`); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (3, datetime('now'))`)
	return err
}

func (s *SQLiteStore) ensureColumn(
	ctx context.Context,
	table string,
	column string,
	definition string,
) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		if name == column {
			return rows.Close()
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition)
	return err
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
func parseTS(v sql.NullString) *time.Time {
	if !v.Valid {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, v.String)
	if err != nil {
		return nil
	}
	return &t
}
func btoi(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (s *SQLiteStore) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	const query = `
		SELECT
			id, username, display_name, email, uid, status, is_default,
			last_validated_at, created_at, updated_at
		FROM accounts
		ORDER BY is_default DESC, display_name
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Account
	for rows.Next() {
		var a domain.Account
		var d int
		var la sql.NullString
		var c, u string
		if err := rows.Scan(
			&a.ID,
			&a.Username,
			&a.DisplayName,
			&a.Email,
			&a.UID,
			&a.Status,
			&d,
			&la,
			&c,
			&u,
		); err != nil {
			return nil, err
		}
		a.IsDefault = d == 1
		a.LastValidatedAt = parseTS(la)
		a.CreatedAt, _ = time.Parse(time.RFC3339Nano, c)
		a.UpdatedAt, _ = time.Parse(time.RFC3339Nano, u)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) GetAccount(ctx context.Context, id string) (domain.Account, error) {
	const query = `
		SELECT
			id, username, display_name, email, uid, status, is_default,
			last_validated_at, created_at, updated_at
		FROM accounts
		WHERE id = ?
	`

	var a domain.Account
	var d int
	var la sql.NullString
	var c, u string
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&a.ID,
		&a.Username,
		&a.DisplayName,
		&a.Email,
		&a.UID,
		&a.Status,
		&d,
		&la,
		&c,
		&u,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return a, domain.NewError(domain.ErrAccountNotFound, "Account not found")
	}
	if err != nil {
		return a, err
	}
	a.IsDefault = d == 1
	a.LastValidatedAt = parseTS(la)
	a.CreatedAt, _ = time.Parse(time.RFC3339Nano, c)
	a.UpdatedAt, _ = time.Parse(time.RFC3339Nano, u)
	return a, nil
}

func (s *SQLiteStore) SaveAccount(ctx context.Context, a domain.Account) error {
	_, err := s.db.ExecContext(
		ctx,
		saveAccountSQL,
		a.ID,
		a.Username,
		a.DisplayName,
		a.Email,
		a.UID,
		a.Status,
		btoi(a.IsDefault),
		optTS(a.LastValidatedAt),
		ts(a.CreatedAt),
		ts(a.UpdatedAt),
	)
	return err
}

func (s *SQLiteStore) SaveAccountAndSelect(ctx context.Context, a domain.Account, selectAccount bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if selectAccount {
		if _, err := tx.ExecContext(ctx, `UPDATE accounts SET is_default=0`); err != nil {
			return err
		}
		a.IsDefault = true
	}
	if _, err := tx.ExecContext(ctx, saveAccountSQL,
		a.ID, a.Username, a.DisplayName, a.Email, a.UID, a.Status, btoi(a.IsDefault),
		optTS(a.LastValidatedAt), ts(a.CreatedAt), ts(a.UpdatedAt)); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *SQLiteStore) SetDefaultAccount(ctx context.Context, id string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `UPDATE accounts SET is_default=0`); e != nil {
		return e
	}
	r, e := tx.ExecContext(ctx, `UPDATE accounts SET is_default=1 WHERE id=?`, id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return domain.NewError(domain.ErrAccountNotFound, "Account not found")
	}
	return tx.Commit()
}
func (s *SQLiteStore) DeleteAccount(ctx context.Context, id string) error {
	r, e := s.db.ExecContext(ctx, `DELETE FROM accounts WHERE id=?`, id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return domain.NewError(domain.ErrAccountNotFound, "Account not found")
	}
	return nil
}

func (s *SQLiteStore) ListVersions(ctx context.Context) ([]domain.GameVersion, error) {
	const query = `
		SELECT
			id, name, channel, platform, architecture, installation_dir,
			executable_path, status, installed_at, verified_at, size_bytes
		FROM game_versions
		ORDER BY installed_at DESC
	`

	rows, e := s.db.QueryContext(ctx, query)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.GameVersion
	for rows.Next() {
		v, e := scanVersion(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(...any) error
}

func scanVersion(r scanner) (domain.GameVersion, error) {
	var v domain.GameVersion
	var ins string
	var ver sql.NullString
	e := r.Scan(
		&v.ID,
		&v.Name,
		&v.Channel,
		&v.Platform,
		&v.Architecture,
		&v.InstallationDir,
		&v.ExecutablePath,
		&v.Status,
		&ins,
		&ver,
		&v.SizeBytes,
	)
	v.InstalledAt, _ = time.Parse(time.RFC3339Nano, ins)
	v.VerifiedAt = parseTS(ver)
	return v, e
}
func (s *SQLiteStore) GetVersion(ctx context.Context, id string) (domain.GameVersion, error) {
	const query = `
		SELECT
			id, name, channel, platform, architecture, installation_dir,
			executable_path, status, installed_at, verified_at, size_bytes
		FROM game_versions
		WHERE id = ?
	`

	v, e := scanVersion(s.db.QueryRowContext(ctx, query, id))
	if errors.Is(e, sql.ErrNoRows) {
		return v, domain.NewError(domain.ErrVersionNotFound, "Game version not found")
	}
	return v, e
}
func (s *SQLiteStore) SaveVersion(ctx context.Context, version domain.GameVersion) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO game_versions(
			id, name, channel, platform, architecture, installation_dir,
			executable_path, status, installed_at, verified_at, size_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		version.ID,
		version.Name,
		version.Channel,
		version.Platform,
		version.Architecture,
		version.InstallationDir,
		version.ExecutablePath,
		version.Status,
		ts(version.InstalledAt),
		optTS(version.VerifiedAt),
		version.SizeBytes,
	)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return domain.NewError(
			domain.ErrVersionExists,
			"This game version is already installed",
		)
	}
	return err
}

func (s *SQLiteStore) UpdateVersion(
	ctx context.Context,
	version domain.GameVersion,
) error {
	const query = `
		UPDATE game_versions SET
			name = ?,
			channel = ?,
			platform = ?,
			architecture = ?,
			installation_dir = ?,
			executable_path = ?,
			status = ?,
			installed_at = ?,
			verified_at = ?,
			size_bytes = ?
		WHERE id = ?
	`

	result, err := s.db.ExecContext(
		ctx,
		query,
		version.Name,
		version.Channel,
		version.Platform,
		version.Architecture,
		version.InstallationDir,
		version.ExecutablePath,
		version.Status,
		ts(version.InstalledAt),
		optTS(version.VerifiedAt),
		version.SizeBytes,
		version.ID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return domain.NewError(
			domain.ErrVersionNotFound,
			"Game version not found",
		)
	}

	return nil
}

func (s *SQLiteStore) DeleteVersion(ctx context.Context, id string) error {
	r, e := s.db.ExecContext(ctx, `DELETE FROM game_versions WHERE id=?`, id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return domain.NewError(domain.ErrVersionNotFound, "Game version not found")
	}
	return nil
}

func (s *SQLiteStore) ListInstances(ctx context.Context) ([]domain.Instance, error) {
	const query = `
		SELECT
			id, name, description, game_version_id, default_account_id,
			directory, cover_path, status, launch_arguments, last_played_at,
			created_at, updated_at
		FROM instances
		ORDER BY COALESCE(last_played_at, created_at) DESC
	`

	rows, e := s.db.QueryContext(ctx, query)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Instance
	for rows.Next() {
		i, e := scanInstance(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, i)
	}
	return out, rows.Err()
}
func scanInstance(r scanner) (domain.Instance, error) {
	var i domain.Instance
	var account, cover, last sql.NullString
	var args, c, u string
	e := r.Scan(&i.ID, &i.Name, &i.Description, &i.GameVersionID, &account, &i.Directory, &cover, &i.Status, &args, &last, &c, &u)
	if account.Valid {
		i.DefaultAccountID = &account.String
	}
	if cover.Valid {
		i.CoverPath = &cover.String
	}
	i.LastPlayedAt = parseTS(last)
	_ = json.Unmarshal([]byte(args), &i.LaunchArguments)
	i.CreatedAt, _ = time.Parse(time.RFC3339Nano, c)
	i.UpdatedAt, _ = time.Parse(time.RFC3339Nano, u)
	return i, e
}
func (s *SQLiteStore) GetInstance(ctx context.Context, id string) (domain.Instance, error) {
	const query = `
		SELECT
			id, name, description, game_version_id, default_account_id,
			directory, cover_path, status, launch_arguments, last_played_at,
			created_at, updated_at
		FROM instances
		WHERE id = ?
	`

	i, e := scanInstance(s.db.QueryRowContext(ctx, query, id))
	if errors.Is(e, sql.ErrNoRows) {
		return i, domain.NewError(domain.ErrInstanceNotFound, "Instance not found")
	}
	return i, e
}
func (s *SQLiteStore) SaveInstance(ctx context.Context, i domain.Instance) error {
	const query = `
		INSERT INTO instances(
			id, name, description, game_version_id, default_account_id,
			directory, cover_path, status, launch_arguments, last_played_at,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			game_version_id = excluded.game_version_id,
			default_account_id = excluded.default_account_id,
			directory = excluded.directory,
			cover_path = excluded.cover_path,
			status = excluded.status,
			launch_arguments = excluded.launch_arguments,
			last_played_at = excluded.last_played_at,
			updated_at = excluded.updated_at
	`

	args, _ := json.Marshal(i.LaunchArguments)
	_, e := s.db.ExecContext(
		ctx,
		query,
		i.ID,
		i.Name,
		i.Description,
		i.GameVersionID,
		i.DefaultAccountID,
		i.Directory,
		i.CoverPath,
		i.Status,
		string(args),
		optTS(i.LastPlayedAt),
		ts(i.CreatedAt),
		ts(i.UpdatedAt),
	)
	if e != nil && strings.Contains(e.Error(), "UNIQUE constraint failed: instances.directory") {
		return domain.NewError(domain.ErrDirectoryConflict, "The directory is already used by another instance")
	}
	return e
}
func (s *SQLiteStore) DeleteInstance(ctx context.Context, id string) error {
	r, e := s.db.ExecContext(ctx, `DELETE FROM instances WHERE id=?`, id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return domain.NewError(domain.ErrInstanceNotFound, "Instance not found")
	}
	return nil
}
func (s *SQLiteStore) IsDirectoryUsed(ctx context.Context, path, except string) (bool, error) {
	var n int
	e := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM instances WHERE directory=? AND id<>?`, path, except).Scan(&n)
	return n > 0, e
}

func (s *SQLiteStore) ListMods(ctx context.Context, instanceID string) ([]domain.InstalledMod, error) {
	const query = `
		SELECT
			id, instance_id, name, version, file_name, file_path,
			enabled, managed, source, size_bytes, installed_at, updated_at
		FROM installed_mods
		WHERE instance_id = ?
		ORDER BY name
	`

	rows, e := s.db.QueryContext(ctx, query, instanceID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.InstalledMod
	for rows.Next() {
		m, e := scanMod(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func scanMod(r scanner) (domain.InstalledMod, error) {
	var m domain.InstalledMod
	var en, ma int
	var ins, up string
	e := r.Scan(
		&m.ID,
		&m.InstanceID,
		&m.Name,
		&m.Version,
		&m.FileName,
		&m.FilePath,
		&en,
		&ma,
		&m.Source,
		&m.SizeBytes,
		&ins,
		&up,
	)
	m.Enabled = en == 1
	m.Managed = ma == 1
	m.InstalledAt, _ = time.Parse(time.RFC3339Nano, ins)
	m.UpdatedAt, _ = time.Parse(time.RFC3339Nano, up)
	return m, e
}
func (s *SQLiteStore) GetMod(ctx context.Context, id string) (domain.InstalledMod, error) {
	const query = `
		SELECT
			id, instance_id, name, version, file_name, file_path,
			enabled, managed, source, size_bytes, installed_at, updated_at
		FROM installed_mods
		WHERE id = ?
	`

	m, e := scanMod(s.db.QueryRowContext(ctx, query, id))
	if errors.Is(e, sql.ErrNoRows) {
		return m, domain.NewError(domain.ErrModNotFound, "Mod not found")
	}
	return m, e
}
func (s *SQLiteStore) SaveMod(ctx context.Context, m domain.InstalledMod) error {
	const query = `
		INSERT INTO installed_mods(
			id, instance_id, name, version, file_name, file_path,
			enabled, managed, source, size_bytes, installed_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			version = excluded.version,
			file_name = excluded.file_name,
			file_path = excluded.file_path,
			enabled = excluded.enabled,
			managed = excluded.managed,
			source = excluded.source,
			size_bytes = excluded.size_bytes,
			updated_at = excluded.updated_at
	`

	_, e := s.db.ExecContext(
		ctx,
		query,
		m.ID,
		m.InstanceID,
		m.Name,
		m.Version,
		m.FileName,
		m.FilePath,
		btoi(m.Enabled),
		btoi(m.Managed),
		m.Source,
		m.SizeBytes,
		ts(m.InstalledAt),
		ts(m.UpdatedAt),
	)
	return e
}
func (s *SQLiteStore) DeleteMod(ctx context.Context, id string) error {
	r, e := s.db.ExecContext(ctx, `DELETE FROM installed_mods WHERE id=?`, id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return domain.NewError(domain.ErrModNotFound, "Mod not found")
	}
	return nil
}

func (s *SQLiteStore) SaveSession(ctx context.Context, p domain.PlaySession) error {
	const query = `
		INSERT INTO play_sessions(
			id, instance_id, account_id, version_id, process_id, started_at,
			ended_at, duration_sec, exit_code, crashed, recovered
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, e := s.db.ExecContext(
		ctx,
		query,
		p.ID,
		p.InstanceID,
		p.AccountID,
		p.VersionID,
		p.ProcessID,
		ts(p.StartedAt),
		optTS(p.EndedAt),
		p.DurationSec,
		p.ExitCode,
		btoi(p.Crashed),
		btoi(p.Recovered),
	)
	return e
}

func (s *SQLiteStore) FinishSession(ctx context.Context, id string, exit int, crashed bool, duration int64) error {
	const query = `
		UPDATE play_sessions
		SET ended_at = ?, duration_sec = ?, exit_code = ?, crashed = ?
		WHERE id = ?
	`

	now := time.Now().UTC()
	_, e := s.db.ExecContext(
		ctx,
		query,
		ts(now),
		duration,
		exit,
		btoi(crashed),
		id,
	)
	return e
}

func (s *SQLiteStore) ListSessions(ctx context.Context, instanceID string, limit int) ([]domain.PlaySession, error) {
	query := `
		SELECT
			id, instance_id, account_id, version_id, process_id, started_at,
			ended_at, duration_sec, exit_code, crashed, recovered
		FROM play_sessions
	`
	var args []any
	if instanceID != "" {
		query += ` WHERE instance_id = ?`
		args = append(args, instanceID)
	}
	query += ` ORDER BY started_at DESC LIMIT ?`
	args = append(args, limit)
	rows, e := s.db.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.PlaySession
	for rows.Next() {
		var p domain.PlaySession
		var account, end sql.NullString
		var pid, exit sql.NullInt64
		var start string
		var crashed, recovered int
		if e = rows.Scan(
			&p.ID,
			&p.InstanceID,
			&account,
			&p.VersionID,
			&pid,
			&start,
			&end,
			&p.DurationSec,
			&exit,
			&crashed,
			&recovered,
		); e != nil {
			return nil, e
		}
		if account.Valid {
			p.AccountID = &account.String
		}
		if pid.Valid {
			x := int(pid.Int64)
			p.ProcessID = &x
		}
		p.StartedAt, _ = time.Parse(time.RFC3339Nano, start)
		p.EndedAt = parseTS(end)
		if exit.Valid {
			x := int(exit.Int64)
			p.ExitCode = &x
		}
		p.Crashed = crashed == 1
		p.Recovered = recovered == 1
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *SQLiteStore) RecoverSessions(ctx context.Context, _ interface{ Unix() int64 }) error {
	return nil
}
func (s *SQLiteStore) RecoverOpenSessions(ctx context.Context, now time.Time) error {
	const recoverSessionsQuery = `
		UPDATE play_sessions
		SET
			ended_at = ?,
			duration_sec = MAX(
				0,
				CAST(strftime('%s', ?) - strftime('%s', started_at) AS INTEGER)
			),
			crashed = 1,
			recovered = 1
		WHERE ended_at IS NULL
	`
	_, e := s.db.ExecContext(ctx, recoverSessionsQuery, ts(now), ts(now))
	_, _ = s.db.ExecContext(ctx, `UPDATE instances SET status='ready' WHERE status='running'`)
	return e
}

func (s *SQLiteStore) ListOperations(ctx context.Context, limit int) ([]domain.Operation, error) {
	const query = `
		SELECT
			id, type, resource_id, title, status, progress, current_bytes,
			total_bytes, bytes_per_second, error_code, error_message,
			created_at, started_at, finished_at, title_key, title_params
		FROM operations
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, e := s.db.QueryContext(ctx, query, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Operation
	for rows.Next() {
		var o domain.Operation
		var res, ec, em, st, fin, tk, tp sql.NullString
		var c string
		if e = rows.Scan(
			&o.ID,
			&o.Type,
			&res,
			&o.Title,
			&o.Status,
			&o.Progress,
			&o.CurrentBytes,
			&o.TotalBytes,
			&o.BytesPerSecond,
			&ec,
			&em,
			&c,
			&st,
			&fin,
			&tk,
			&tp,
		); e != nil {
			return nil, e
		}
		if res.Valid {
			o.ResourceID = &res.String
		}
		if ec.Valid {
			o.ErrorCode = &ec.String
		}
		if em.Valid {
			o.ErrorMessage = &em.String
		}
		if tk.Valid {
			o.TitleKey = tk.String
		}
		if tp.Valid && tp.String != "" {
			params := map[string]string{}
			if err := json.Unmarshal([]byte(tp.String), &params); err == nil && len(params) > 0 {
				o.TitleParams = params
			}
		}
		o.CreatedAt, _ = time.Parse(time.RFC3339Nano, c)
		o.StartedAt = parseTS(st)
		o.FinishedAt = parseTS(fin)
		out = append(out, o)
	}
	return out, rows.Err()
}
func (s *SQLiteStore) SaveOperation(ctx context.Context, o domain.Operation) error {
	params, err := json.Marshal(o.TitleParams)
	if err != nil {
		return err
	}
	const query = `
		INSERT INTO operations(
			id, type, resource_id, title, status, progress, current_bytes,
			total_bytes, bytes_per_second, error_code, error_message,
			created_at, started_at, finished_at, title_key, title_params
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			type = excluded.type,
			title = excluded.title,
			status = excluded.status,
			progress = excluded.progress,
			current_bytes = excluded.current_bytes,
			total_bytes = excluded.total_bytes,
			bytes_per_second = excluded.bytes_per_second,
			error_code = excluded.error_code,
			error_message = excluded.error_message,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			title_key = excluded.title_key,
			title_params = excluded.title_params
	`

	_, e := s.db.ExecContext(
		ctx,
		query,
		o.ID,
		o.Type,
		o.ResourceID,
		o.Title,
		o.Status,
		o.Progress,
		o.CurrentBytes,
		o.TotalBytes,
		o.BytesPerSecond,
		o.ErrorCode,
		o.ErrorMessage,
		ts(o.CreatedAt),
		optTS(o.StartedAt),
		optTS(o.FinishedAt),
		nullableString(o.TitleKey),
		nullableString(string(params)),
	)
	return e
}

func (s *SQLiteStore) DeleteFinishedOperation(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(
		ctx,
		`DELETE FROM operations
		 WHERE id = ? AND status IN ('completed', 'failed', 'cancelled')`,
		id,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.NewError(
			domain.ErrOperationNotFound,
			"The finished operation was not found",
		)
	}
	return nil
}

func (s *SQLiteStore) ClearFinishedOperations(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(
		ctx,
		`DELETE FROM operations
		 WHERE status IN ('completed', 'failed', 'cancelled')`,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *SQLiteStore) GetSettings(ctx context.Context) (domain.Settings, error) {
	settings := domain.Settings{
		Language:                 "en",
		DownloadsParallel:        3,
		ConfirmDeletion:          true,
		GlobalLaunchArguments:    []string{},
		CheckForUpdates:          true,
		UpdateChannel:            "stable",
		TelemetryEnabled:         true,
		AutomaticSafetySnapshots: true,
	}
	rows, e := s.db.QueryContext(ctx, `SELECT key,value FROM app_settings`)
	if e != nil {
		return settings, e
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var k, x string
		if e = rows.Scan(&k, &x); e != nil {
			return settings, e
		}
		m[k] = x
	}
	if x := m["settings"]; x != "" {
		if e = json.Unmarshal([]byte(x), &settings); e != nil {
			return settings, fmt.Errorf("decode settings: %w", e)
		}
	}
	if settings.GlobalLaunchArguments == nil {
		settings.GlobalLaunchArguments = []string{}
	}
	return settings, rows.Err()
}

func (s *SQLiteStore) SaveSettings(ctx context.Context, settings domain.Settings) error {
	const query = `
		INSERT INTO app_settings(key, value)
		VALUES ('settings', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`

	encodedSettings, e := json.Marshal(settings)
	if e != nil {
		return e
	}
	_, e = s.db.ExecContext(ctx, query, string(encodedSettings))
	return e
}

// GetSettingValue reads an arbitrary setting value from the shared
// app_settings storage. Telemetry uses it for its installation ID and
// last-heartbeat state. A missing key returns ("", nil).
func (s *SQLiteStore) GetSettingValue(ctx context.Context, key string) (string, error) {
	const query = `SELECT value FROM app_settings WHERE key = ?`
	var value string
	err := s.db.QueryRowContext(ctx, query, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// SetSettingValue stores an arbitrary setting value in the shared app_settings
// storage.
func (s *SQLiteStore) SetSettingValue(ctx context.Context, key, value string) error {
	const query = `
		INSERT INTO app_settings(key, value)
		VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`
	_, err := s.db.ExecContext(ctx, query, key, value)
	return err
}

type relocationStatement struct {
	query string
	args  []any
}

// RelocatePaths rewrites stored absolute paths after the data root has moved.
// Every stored path that begins with oldRoot is re-prefixed with newRoot. It
// runs in a single transaction and is safe to run again after an interruption,
// because already-rewritten paths no longer match the old prefix.
func (s *SQLiteStore) RelocatePaths(ctx context.Context, oldRoot, newRoot string) error {
	prefixLength := len(oldRoot) + 1
	statements := []relocationStatement{
		{
			query: `UPDATE game_versions
				SET installation_dir = ? || substr(installation_dir, ?),
				    executable_path = ? || substr(executable_path, ?)
				WHERE instr(installation_dir, ?) = 1 OR instr(executable_path, ?) = 1`,
			args: []any{newRoot, prefixLength, newRoot, prefixLength, oldRoot, oldRoot},
		},
		{
			query: `UPDATE instances
				SET directory = ? || substr(directory, ?),
				    cover_path = ? || substr(cover_path, ?)
				WHERE instr(directory, ?) = 1 OR instr(cover_path, ?) = 1`,
			args: []any{newRoot, prefixLength, newRoot, prefixLength, oldRoot, oldRoot},
		},
		{
			query: `UPDATE installed_mods
				SET file_path = ? || substr(file_path, ?)
				WHERE instr(file_path, ?) = 1`,
			args: []any{newRoot, prefixLength, oldRoot},
		},
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
