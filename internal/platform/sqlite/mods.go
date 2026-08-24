package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
)

const modColumns = `id, instance_id, name, version, file_name, file_path,
	enabled, managed, source, update_policy, size_bytes, installed_at, updated_at`

func (s *SQLiteStore) ListMods(ctx context.Context, instanceID string) ([]mods.InstalledMod, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+modColumns+` FROM installed_mods WHERE instance_id=? ORDER BY name`, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var mods []mods.InstalledMod
	for rows.Next() {
		mod, err := scanMod(rows)
		if err != nil {
			return nil, err
		}
		mods = append(mods, mod)
	}
	return mods, rows.Err()
}

func scanMod(row scanner) (mods.InstalledMod, error) {
	var mod mods.InstalledMod
	var enabled, managed int
	var installed, updated string
	err := row.Scan(&mod.ID, &mod.InstanceID, &mod.Name, &mod.Version, &mod.FileName, &mod.FilePath,
		&enabled, &managed, &mod.Source, &mod.UpdatePolicy, &mod.SizeBytes, &installed, &updated)
	mod.Enabled = enabled == 1
	mod.Managed = managed == 1
	mod.UpdatePolicy = mods.NormalizeUpdatePolicy(mod.UpdatePolicy)
	mod.InstalledAt, _ = time.Parse(time.RFC3339Nano, installed)
	mod.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return mod, err
}

func (s *SQLiteStore) GetMod(ctx context.Context, id string) (mods.InstalledMod, error) {
	mod, err := scanMod(s.db.QueryRowContext(ctx, `SELECT `+modColumns+` FROM installed_mods WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return mod, errs.NewError(mods.ErrModNotFound, "Mod not found")
	}
	return mod, err
}

func (s *SQLiteStore) SaveMod(ctx context.Context, mod mods.InstalledMod) error {
	mod.UpdatePolicy = mods.NormalizeUpdatePolicy(mod.UpdatePolicy)
	_, err := s.db.ExecContext(ctx, `INSERT INTO installed_mods(`+modColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, version=excluded.version, file_name=excluded.file_name,
		file_path=excluded.file_path, enabled=excluded.enabled, managed=excluded.managed, source=excluded.source, update_policy=excluded.update_policy,
		size_bytes=excluded.size_bytes, updated_at=excluded.updated_at`,
		mod.ID, mod.InstanceID, mod.Name, mod.Version, mod.FileName, mod.FilePath, btoi(mod.Enabled), btoi(mod.Managed),
		mod.Source, mod.UpdatePolicy, mod.SizeBytes, ts(mod.InstalledAt), ts(mod.UpdatedAt))
	return err
}

func (s *SQLiteStore) DeleteMod(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM installed_mods WHERE id=?`, id)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return errs.NewError(mods.ErrModNotFound, "Mod not found")
	}
	return nil
}
