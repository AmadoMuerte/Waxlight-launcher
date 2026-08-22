package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/operations"
)

func (s *SQLiteStore) ListOperations(ctx context.Context, limit int) ([]operations.Operation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, type, resource_id, title, status, progress, current_bytes,
		total_bytes, bytes_per_second, error_code, error_message, created_at, started_at, finished_at,
		title_key, title_params FROM operations ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []operations.Operation
	for rows.Next() {
		operation, err := scanOperation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, operation)
	}
	return result, rows.Err()
}

func scanOperation(row scanner) (operations.Operation, error) {
	var operation operations.Operation
	var resource, errorCode, errorMessage, started, finished, titleKey, titleParams sql.NullString
	var created string
	err := row.Scan(&operation.ID, &operation.Type, &resource, &operation.Title, &operation.Status, &operation.Progress,
		&operation.CurrentBytes, &operation.TotalBytes, &operation.BytesPerSecond, &errorCode, &errorMessage,
		&created, &started, &finished, &titleKey, &titleParams)
	if resource.Valid {
		operation.ResourceID = &resource.String
	}
	if errorCode.Valid {
		operation.ErrorCode = &errorCode.String
	}
	if errorMessage.Valid {
		operation.ErrorMessage = &errorMessage.String
	}
	if titleKey.Valid {
		operation.TitleKey = titleKey.String
	}
	if titleParams.Valid && titleParams.String != "" {
		params := map[string]string{}
		if err := json.Unmarshal([]byte(titleParams.String), &params); err == nil && len(params) > 0 {
			operation.TitleParams = params
		}
	}
	operation.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	operation.StartedAt = parseTS(started)
	operation.FinishedAt = parseTS(finished)
	return operation, err
}

func (s *SQLiteStore) SaveOperation(ctx context.Context, operation operations.Operation) error {
	params, err := json.Marshal(operation.TitleParams)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO operations(id, type, resource_id, title, status, progress,
		current_bytes, total_bytes, bytes_per_second, error_code, error_message, created_at, started_at,
		finished_at, title_key, title_params) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET type=excluded.type, resource_id=excluded.resource_id, title=excluded.title, status=excluded.status,
		progress=excluded.progress, current_bytes=excluded.current_bytes, total_bytes=excluded.total_bytes,
		bytes_per_second=excluded.bytes_per_second, error_code=excluded.error_code, error_message=excluded.error_message,
		started_at=excluded.started_at, finished_at=excluded.finished_at, title_key=excluded.title_key,
		title_params=excluded.title_params`, operation.ID, operation.Type, operation.ResourceID, operation.Title,
		operation.Status, operation.Progress, operation.CurrentBytes, operation.TotalBytes, operation.BytesPerSecond,
		operation.ErrorCode, operation.ErrorMessage, ts(operation.CreatedAt), optTS(operation.StartedAt),
		optTS(operation.FinishedAt), nullableString(operation.TitleKey), nullableString(string(params)))
	return err
}

func (s *SQLiteStore) ReconcileInterruptedOperations(
	ctx context.Context,
	finishedAt time.Time,
	errorCode string,
	errorMessage string,
) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE operations
		SET status='failed', error_code=?, error_message=?, finished_at=?, bytes_per_second=0
		WHERE status IN ('queued', 'running')`, errorCode, errorMessage, ts(finishedAt))
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *SQLiteStore) DeleteFinishedOperation(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM operations WHERE id=? AND status IN ('completed', 'failed', 'cancelled')`, id)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil {
		return err
	} else if count == 0 {
		return errs.NewError(errs.ErrOperationNotFound, "The finished operation was not found")
	}
	return nil
}

func (s *SQLiteStore) ClearFinishedOperations(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM operations WHERE status IN ('completed', 'failed', 'cancelled')`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
