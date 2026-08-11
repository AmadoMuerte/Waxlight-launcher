package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/domain"
)

const saveAccountSQL = `
	INSERT INTO accounts(
		id, username, display_name, email, uid, status, is_default,
		last_validated_at, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		username = excluded.username, display_name = excluded.display_name,
		email = excluded.email, uid = excluded.uid, status = excluded.status,
		is_default = excluded.is_default, last_validated_at = excluded.last_validated_at,
		updated_at = excluded.updated_at
`

func (s *SQLiteStore) ListAccounts(ctx context.Context) ([]accounts.Account, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, username, display_name, email, uid, status, is_default,
		last_validated_at, created_at, updated_at FROM accounts ORDER BY is_default DESC, display_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []accounts.Account
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, account)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) GetAccount(ctx context.Context, id string) (accounts.Account, error) {
	account, err := scanAccount(s.db.QueryRowContext(ctx, `SELECT id, username, display_name, email, uid, status,
		is_default, last_validated_at, created_at, updated_at FROM accounts WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return account, domain.NewError(domain.ErrAccountNotFound, "Account not found")
	}
	return account, err
}

func scanAccount(row scanner) (accounts.Account, error) {
	var account accounts.Account
	var isDefault int
	var validated sql.NullString
	var created, updated string
	err := row.Scan(&account.ID, &account.Username, &account.DisplayName, &account.Email, &account.UID,
		&account.Status, &isDefault, &validated, &created, &updated)
	account.IsDefault = isDefault == 1
	account.LastValidatedAt = parseTS(validated)
	account.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	account.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return account, err
}

func (s *SQLiteStore) SaveAccount(ctx context.Context, account accounts.Account) error {
	_, err := s.db.ExecContext(ctx, saveAccountSQL, account.ID, account.Username, account.DisplayName,
		account.Email, account.UID, account.Status, btoi(account.IsDefault), optTS(account.LastValidatedAt),
		ts(account.CreatedAt), ts(account.UpdatedAt))
	return err
}

func (s *SQLiteStore) SaveAccountAndSelect(ctx context.Context, account accounts.Account, selectAccount bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if selectAccount {
		if _, err := tx.ExecContext(ctx, `UPDATE accounts SET is_default=0`); err != nil {
			return err
		}
		account.IsDefault = true
	}
	if _, err := tx.ExecContext(ctx, saveAccountSQL, account.ID, account.Username, account.DisplayName,
		account.Email, account.UID, account.Status, btoi(account.IsDefault), optTS(account.LastValidatedAt),
		ts(account.CreatedAt), ts(account.UpdatedAt)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) SetDefaultAccount(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE accounts SET is_default=0`); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE accounts SET is_default=1 WHERE id=?`, id)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return domain.NewError(domain.ErrAccountNotFound, "Account not found")
	}
	return tx.Commit()
}

func (s *SQLiteStore) DeleteAccount(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM accounts WHERE id=?`, id)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return domain.NewError(domain.ErrAccountNotFound, "Account not found")
	}
	return nil
}
