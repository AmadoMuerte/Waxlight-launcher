package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/news"
)

const newsStateKey = "news_state"

func (store *SQLiteStore) LoadNewsState(ctx context.Context) (news.State, bool, error) {
	var raw string
	err := store.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key=?`, newsStateKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return news.State{}, false, nil
	}
	if err != nil {
		return news.State{}, false, err
	}
	var state news.State
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return news.State{}, false, fmt.Errorf("decode news state: %w", err)
	}
	return state, true, nil
}

func (store *SQLiteStore) SaveNewsState(ctx context.Context, state news.State) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = store.db.ExecContext(ctx, `INSERT INTO app_settings(key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, newsStateKey, string(data))
	return err
}
