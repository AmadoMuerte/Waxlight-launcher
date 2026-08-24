package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/recovery"
)

func (s *SQLiteStore) GetLastKnownGood(ctx context.Context, instanceID string) (recovery.LastKnownGood, error) {
	var marker recovery.LastKnownGood
	var recorded, mods string
	var snapshot sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT recorded_at, game_version, snapshot_id, mods
		FROM last_known_good WHERE instance_id=?`, instanceID).Scan(&recorded, &marker.GameVersion, &snapshot, &mods)
	if errors.Is(err, sql.ErrNoRows) {
		return marker, errs.ErrNotFound
	}
	if err != nil {
		return marker, err
	}
	marker.InstanceID = instanceID
	marker.RecordedAt, _ = time.Parse(time.RFC3339Nano, recorded)
	if snapshot.Valid {
		marker.SnapshotID = snapshot.String
	}
	if err := json.Unmarshal([]byte(mods), &marker.Mods); err != nil {
		return marker, err
	}
	return marker, nil
}

func (s *SQLiteStore) SaveLastKnownGood(ctx context.Context, marker recovery.LastKnownGood) error {
	mods, err := json.Marshal(marker.Mods)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO last_known_good(instance_id, recorded_at, game_version, snapshot_id, mods)
		VALUES (?, ?, ?, ?, ?) ON CONFLICT(instance_id) DO UPDATE SET recorded_at=excluded.recorded_at,
		game_version=excluded.game_version, snapshot_id=excluded.snapshot_id, mods=excluded.mods`,
		marker.InstanceID, ts(marker.RecordedAt), marker.GameVersion, nullableString(marker.SnapshotID), string(mods))
	return err
}

func (s *SQLiteStore) DeleteLastKnownGood(ctx context.Context, instanceID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM last_known_good WHERE instance_id=?`, instanceID)
	return err
}
