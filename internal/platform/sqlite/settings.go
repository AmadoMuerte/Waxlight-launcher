package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

func (s *SQLiteStore) GetSettings(ctx context.Context) (domain.Settings, error) {
	settings := domain.Settings{Language: "en", DownloadsParallel: 3, ConfirmDeletion: true,
		GlobalLaunchArguments: []string{}, CheckForUpdates: true, UpdateChannel: "stable",
		TelemetryEnabled: false, AutomaticSafetySnapshots: true}
	rows, err := s.db.QueryContext(ctx, `SELECT key,value FROM app_settings`)
	if err != nil {
		return settings, err
	}
	defer rows.Close()
	values := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return settings, err
		}
		values[key] = value
	}
	if value := values["settings"]; value != "" {
		if err := json.Unmarshal([]byte(value), &settings); err != nil {
			return settings, fmt.Errorf("decode settings: %w", err)
		}
	}
	if settings.GlobalLaunchArguments == nil {
		settings.GlobalLaunchArguments = []string{}
	}
	return settings, rows.Err()
}

func (s *SQLiteStore) SaveSettings(ctx context.Context, settings domain.Settings) error {
	encoded, err := json.Marshal(settings)
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
