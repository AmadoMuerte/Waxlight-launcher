package credentials

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/platform/atomicfile"
)

const maxLegacyFileBytes = 1 << 20

type legacyFileData struct {
	Version int                           `json:"version"`
	Secrets map[string]legacyStoredSecret `json:"secrets"`
}

type legacyStoredSecret struct {
	SessionKey       string `json:"sessionkey"`
	SessionSignature string `json:"sessionsignature"`
}

type migrationState struct {
	Version   int    `json:"version"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updatedAt"`
	Imported  int    `json:"imported,omitempty"`
}

type Migrator struct {
	legacyPath string
	statePath  string
	store      accounts.Credentials
}

func NewMigrator(dataRoot string, store accounts.Credentials) *Migrator {
	return &Migrator{
		legacyPath: filepath.Join(dataRoot, "account-secrets.json"),
		statePath:  filepath.Join(dataRoot, "security", "credential-migration.json"),
		store:      store,
	}
}

func (m *Migrator) Run(ctx context.Context, accountIDs []string) error {
	contents, err := readLegacyFile(m.legacyPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		_ = m.writeState("blocked", 0)
		return fmt.Errorf("legacy credential migration blocked: %w", err)
	}
	data, err := decodeLegacy(contents)
	if err != nil {
		_ = m.writeState("blocked", 0)
		return fmt.Errorf("legacy credential migration blocked: %w", err)
	}
	known := make(map[string]struct{}, len(accountIDs))
	for _, id := range accountIDs {
		known[id] = struct{}{}
	}
	for id := range data.Secrets {
		if _, ok := known[id]; !ok {
			_ = m.writeState("blocked", 0)
			return errors.New("legacy credential migration blocked: secret references unknown account metadata")
		}
	}

	imported := 0
	for id, legacy := range data.Secrets {
		secret := accounts.Credential{SessionKey: legacy.SessionKey, SessionSignature: legacy.SessionSignature}
		if err := m.store.Set(ctx, id, secret); err != nil {
			_ = m.writeState("retry_required", imported)
			return secretStoreErrorForMigration(err)
		}
		readBack, err := m.store.Get(ctx, id)
		if err != nil || !sameSecret(secret, readBack) {
			_ = m.writeState("retry_required", imported)
			return errors.New("legacy credential migration verification failed; source file was retained")
		}
		imported++
	}
	if err := bestEffortEraseAndRemove(m.legacyPath, int64(len(contents))); err != nil {
		_ = m.writeState("cleanup_required", imported)
		return errors.New("legacy credentials were imported but the plaintext source could not be removed")
	}
	return m.writeState("complete", imported)
}

func decodeLegacy(contents []byte) (legacyFileData, error) {
	if len(contents) == 0 || len(contents) > maxLegacyFileBytes {
		return legacyFileData{}, errors.New("legacy credential file has an invalid size")
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var data legacyFileData
	if err := decoder.Decode(&data); err != nil {
		return data, errors.New("legacy credential file has an invalid schema")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return data, errors.New("legacy credential file contains trailing data")
	}
	if data.Version != 1 || data.Secrets == nil {
		return data, errors.New("legacy credential file has an unsupported schema")
	}
	for id, secret := range data.Secrets {
		if id == "" || len(id) > 128 || secret.SessionKey == "" || secret.SessionSignature == "" {
			return data, errors.New("legacy credential file contains an invalid entry")
		}
	}
	return data, nil
}

func sameSecret(left, right accounts.Credential) bool {
	return subtle.ConstantTimeCompare([]byte(left.SessionKey), []byte(right.SessionKey)) == 1 &&
		subtle.ConstantTimeCompare([]byte(left.SessionSignature), []byte(right.SessionSignature)) == 1
}

func (m *Migrator) writeState(status string, imported int) error {
	directory := filepath.Dir(m.statePath)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return err
	}
	contents, err := json.Marshal(migrationState{Version: 1, Status: status, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano), Imported: imported})
	if err != nil {
		return err
	}
	return atomicfile.Write(m.statePath, append(contents, '\n'), 0o600)
}

func secretStoreErrorForMigration(err error) error {
	switch {
	case errors.Is(err, accounts.ErrStoreLocked):
		return fmt.Errorf("legacy credential migration requires an unlocked operating-system credential store; source file was retained: %w", accounts.ErrStoreLocked)
	case errors.Is(err, accounts.ErrPermissionDenied):
		return fmt.Errorf("operating-system credential store denied the legacy migration; source file was retained: %w", accounts.ErrPermissionDenied)
	default:
		return fmt.Errorf("operating-system credential store is unavailable for legacy migration; source file was retained: %w", accounts.ErrStoreUnavailable)
	}
}

func bestEffortEraseAndRemove(path string, size int64) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	zeroes := make([]byte, 32*1024)
	remaining := size
	for remaining > 0 {
		chunk := int64(len(zeroes))
		if chunk > remaining {
			chunk = remaining
		}
		if _, writeErr := file.Write(zeroes[:chunk]); writeErr != nil {
			_ = file.Close()
			return writeErr
		}
		remaining -= chunk
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Truncate(0); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Remove(path)
}
