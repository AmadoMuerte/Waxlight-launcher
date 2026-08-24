package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/settings"
)

func (s *SQLiteStore) GetSettings(ctx context.Context) (settings.Settings, error) {
	value := settings.Defaults()
	rows, err := s.db.QueryContext(ctx, `SELECT key,value FROM app_settings`)
	if err != nil {
		return value, err
	}
	defer rows.Close()
	values := map[string]string{}
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			return value, err
		}
		values[key] = raw
	}
	if raw := values["settings"]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return value, fmt.Errorf("decode settings: %w", err)
		}
	}
	if value.GlobalLaunchArguments == nil {
		value.GlobalLaunchArguments = []string{}
	}
	return value, rows.Err()
}

func (s *SQLiteStore) SaveSettings(ctx context.Context, value settings.Settings) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO app_settings(key, value) VALUES ('settings', ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, string(encoded))
	return err
}

func (s *SQLiteStore) GetSettingValue(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key=?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

func (s *SQLiteStore) SetSettingValue(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO app_settings(key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}
