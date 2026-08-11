package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

const sessionColumns = `id, instance_id, account_id, version_id, process_id, started_at,
	ended_at, duration_sec, exit_code, crashed, recovered`

func (s *SQLiteStore) SaveSession(ctx context.Context, session domain.PlaySession) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO play_sessions(`+sessionColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.InstanceID, session.AccountID, session.VersionID, session.ProcessID, ts(session.StartedAt),
		optTS(session.EndedAt), session.DurationSec, session.ExitCode, btoi(session.Crashed), btoi(session.Recovered))
	return err
}

func (s *SQLiteStore) FinishSession(ctx context.Context, id string, exit int, crashed bool, duration int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE play_sessions SET ended_at=?, duration_sec=?, exit_code=?, crashed=? WHERE id=?`,
		ts(time.Now().UTC()), duration, exit, btoi(crashed), id)
	return err
}

func (s *SQLiteStore) ListSessions(ctx context.Context, instanceID string, limit int) ([]domain.PlaySession, error) {
	query := `SELECT ` + sessionColumns + ` FROM play_sessions`
	var args []any
	if instanceID != "" {
		query += ` WHERE instance_id=?`
		args = append(args, instanceID)
	}
	query += ` ORDER BY started_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []domain.PlaySession
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func scanSession(row scanner) (domain.PlaySession, error) {
	var session domain.PlaySession
	var account, ended sql.NullString
	var processID, exitCode sql.NullInt64
	var started string
	var crashed, recovered int
	err := row.Scan(&session.ID, &session.InstanceID, &account, &session.VersionID, &processID, &started,
		&ended, &session.DurationSec, &exitCode, &crashed, &recovered)
	if account.Valid {
		session.AccountID = &account.String
	}
	if processID.Valid {
		value := int(processID.Int64)
		session.ProcessID = &value
	}
	session.StartedAt, _ = time.Parse(time.RFC3339Nano, started)
	session.EndedAt = parseTS(ended)
	if exitCode.Valid {
		value := int(exitCode.Int64)
		session.ExitCode = &value
	}
	session.Crashed = crashed == 1
	session.Recovered = recovered == 1
	return session, err
}

func (s *SQLiteStore) RecoverSessions(context.Context, interface{ Unix() int64 }) error { return nil }

func (s *SQLiteStore) RecoverOpenSessions(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE play_sessions SET ended_at=?, duration_sec=MAX(0,
		CAST(strftime('%s', ?) - strftime('%s', started_at) AS INTEGER)), crashed=1, recovered=1 WHERE ended_at IS NULL`,
		ts(now), ts(now))
	_, _ = s.db.ExecContext(ctx, `UPDATE instances SET status='ready' WHERE status='running'`)
	return err
}
