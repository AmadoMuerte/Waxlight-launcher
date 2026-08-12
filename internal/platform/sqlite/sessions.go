package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/sessions"
)

const sessionColumns = `id, instance_id, account_id, version_id, process_id, started_at,
	ended_at, duration_sec, exit_code, crashed, recovered`

func (s *SQLiteStore) SaveSession(ctx context.Context, session sessions.PlaySession) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO play_sessions(`+sessionColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.InstanceID, session.AccountID, session.VersionID, session.ProcessID, ts(session.StartedAt),
		optTS(session.EndedAt), session.DurationSec, session.ExitCode, btoi(session.Crashed), btoi(session.Recovered))
	return err
}

func (s *SQLiteStore) FinishSession(
	ctx context.Context,
	id string,
	endedAt time.Time,
	exit int,
	crashed bool,
	duration int64,
) error {
	_, err := s.db.ExecContext(ctx, `UPDATE play_sessions SET ended_at=?, duration_sec=?, exit_code=?, crashed=? WHERE id=?`,
		ts(endedAt), duration, exit, btoi(crashed), id)
	return err
}

func (s *SQLiteStore) ListSessions(ctx context.Context, instanceID string, limit int) ([]sessions.PlaySession, error) {
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
	var playSessions []sessions.PlaySession
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		playSessions = append(playSessions, session)
	}
	return playSessions, rows.Err()
}

func scanSession(row scanner) (sessions.PlaySession, error) {
	var session sessions.PlaySession
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

func (s *SQLiteStore) SessionStatistics(ctx context.Context) (sessions.StatisticsTotals, error) {
	var totals sessions.StatisticsTotals
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(duration_sec), 0), COUNT(*) FROM play_sessions`).Scan(
		&totals.TotalPlaytimeSeconds,
		&totals.LaunchCount,
	); err != nil {
		return totals, err
	}
	var instanceID string
	err := s.db.QueryRowContext(ctx, `SELECT instance_id FROM play_sessions
		GROUP BY instance_id HAVING SUM(duration_sec) > 0
		ORDER BY SUM(duration_sec) DESC, instance_id ASC LIMIT 1`).Scan(&instanceID)
	if err == nil {
		totals.MostPlayedInstanceID = &instanceID
	} else if !errors.Is(err, sql.ErrNoRows) {
		return totals, err
	}
	return totals, nil
}

func (s *SQLiteStore) InstancePlaytime(ctx context.Context, instanceID string) (int64, error) {
	var total int64
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(duration_sec), 0) FROM play_sessions WHERE instance_id=?`, instanceID).Scan(&total)
	return total, err
}

func (s *SQLiteStore) RecoverOpenSessions(ctx context.Context, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE play_sessions SET ended_at=?, duration_sec=MAX(0,
		CAST(strftime('%s', ?) - strftime('%s', started_at) AS INTEGER)), crashed=1, recovered=1 WHERE ended_at IS NULL`,
		ts(now), ts(now)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE instances SET status=? WHERE status=?`, instances.StatusReady, instances.StatusRunning); err != nil {
		return err
	}
	return tx.Commit()
}
