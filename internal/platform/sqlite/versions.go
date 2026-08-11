package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

const versionColumns = `id, name, channel, platform, architecture, installation_dir,
	executable_path, status, installed_at, verified_at, size_bytes`

func (s *SQLiteStore) ListVersions(ctx context.Context) ([]versions.GameVersion, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+versionColumns+` FROM game_versions ORDER BY installed_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []versions.GameVersion
	for rows.Next() {
		version, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, version)
	}
	return result, rows.Err()
}

func scanVersion(row scanner) (versions.GameVersion, error) {
	var version versions.GameVersion
	var installed string
	var verified sql.NullString
	err := row.Scan(&version.ID, &version.Name, &version.Channel, &version.Platform, &version.Architecture,
		&version.InstallationDir, &version.ExecutablePath, &version.Status, &installed, &verified, &version.SizeBytes)
	version.InstalledAt, _ = time.Parse(time.RFC3339Nano, installed)
	version.VerifiedAt = parseTS(verified)
	return version, err
}

func (s *SQLiteStore) GetVersion(ctx context.Context, id string) (versions.GameVersion, error) {
	version, err := scanVersion(s.db.QueryRowContext(ctx, `SELECT `+versionColumns+` FROM game_versions WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return version, domain.NewError(domain.ErrVersionNotFound, "Game version not found")
	}
	return version, err
}

func (s *SQLiteStore) SaveVersion(ctx context.Context, version versions.GameVersion) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO game_versions(`+versionColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		version.ID, version.Name, version.Channel, version.Platform, version.Architecture, version.InstallationDir,
		version.ExecutablePath, version.Status, ts(version.InstalledAt), optTS(version.VerifiedAt), version.SizeBytes)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return domain.NewError(domain.ErrVersionExists, "This game version is already installed")
	}
	return err
}

func (s *SQLiteStore) UpdateVersion(ctx context.Context, version versions.GameVersion) error {
	result, err := s.db.ExecContext(ctx, `UPDATE game_versions SET name=?, channel=?, platform=?, architecture=?,
		installation_dir=?, executable_path=?, status=?, installed_at=?, verified_at=?, size_bytes=? WHERE id=?`,
		version.Name, version.Channel, version.Platform, version.Architecture, version.InstallationDir,
		version.ExecutablePath, version.Status, ts(version.InstalledAt), optTS(version.VerifiedAt), version.SizeBytes, version.ID)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil {
		return err
	} else if count == 0 {
		return domain.NewError(domain.ErrVersionNotFound, "Game version not found")
	}
	return nil
}

func (s *SQLiteStore) VersionReference(ctx context.Context, id string) (string, bool, error) {
	var name string
	err := s.db.QueryRowContext(ctx, `SELECT name FROM instances WHERE game_version_id=? ORDER BY created_at LIMIT 1`, id).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return name, err == nil, err
}

func (s *SQLiteStore) DeleteVersion(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM game_versions WHERE id=?`, id)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return domain.NewError(domain.ErrVersionNotFound, "Game version not found")
	}
	return nil
}
