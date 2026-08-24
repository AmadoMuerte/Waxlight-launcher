package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/servers"
)

const favoriteServerColumns = `id, name, address, instance_id, created_at, updated_at`

func (s *SQLiteStore) ListFavoriteServers(ctx context.Context) ([]servers.FavoriteServer, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+favoriteServerColumns+` FROM favorite_servers ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	servers := []servers.FavoriteServer{}
	for rows.Next() {
		server, err := scanFavoriteServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, rows.Err()
}

func (s *SQLiteStore) GetFavoriteServer(ctx context.Context, id string) (servers.FavoriteServer, error) {
	server, err := scanFavoriteServer(s.db.QueryRowContext(ctx, `SELECT `+favoriteServerColumns+` FROM favorite_servers WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return server, errs.NewError(errs.ErrServerNotFound, "Favorite server not found")
	}
	return server, err
}

func (s *SQLiteStore) SaveFavoriteServer(ctx context.Context, server servers.FavoriteServer) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO favorite_servers(`+favoriteServerColumns+`) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, address=excluded.address,
		instance_id=excluded.instance_id, updated_at=excluded.updated_at`,
		server.ID, server.Name, server.Address, server.InstanceID, ts(server.CreatedAt), ts(server.UpdatedAt))
	return err
}

func (s *SQLiteStore) DeleteFavoriteServer(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM favorite_servers WHERE id=?`, id)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return errs.NewError(errs.ErrServerNotFound, "Favorite server not found")
	}
	return nil
}

func scanFavoriteServer(row scanner) (servers.FavoriteServer, error) {
	var server servers.FavoriteServer
	var instanceID sql.NullString
	var created, updated string
	err := row.Scan(&server.ID, &server.Name, &server.Address, &instanceID, &created, &updated)
	if instanceID.Valid {
		server.InstanceID = &instanceID.String
	}
	server.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	server.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return server, err
}
