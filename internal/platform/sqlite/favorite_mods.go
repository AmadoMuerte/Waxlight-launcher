package sqlite

import "context"

func (s *SQLiteStore) ListFavoriteModIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT mod_id FROM favorite_mods ORDER BY mod_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *SQLiteStore) SetModFavorite(ctx context.Context, modID string, favorite bool) error {
	if favorite {
		_, err := s.db.ExecContext(ctx, `INSERT INTO favorite_mods(mod_id) VALUES (?) ON CONFLICT DO NOTHING`, modID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM favorite_mods WHERE mod_id=?`, modID)
	return err
}
