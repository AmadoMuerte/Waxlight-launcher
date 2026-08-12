package credentials

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
	keyring "github.com/zalando/go-keyring"
)

const ServiceNamespace = "com.waxlight.launcher"

type keyringBackend interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

// Store persists versioned session material in the operating system's native
// credential service. It intentionally has no file or in-memory fallback.
type Store struct {
	backend     keyringBackend
	pendingPath string
	pendingMu   sync.Mutex
}

func NewStore(dataRoot string) *Store {
	return &Store{backend: systemBackend{}, pendingPath: filepath.Join(dataRoot, "security", "pending-credential-commits.json")}
}

func newStoreWithBackend(backend keyringBackend) *Store { return &Store{backend: backend} }

func (store *Store) Get(ctx context.Context, accountID string) (accounts.Credential, error) {
	if err := validateRequest(ctx, accountID); err != nil {
		return accounts.Credential{}, err
	}
	value, err := store.backend.Get(ServiceNamespace, accountID)
	if err != nil {
		slog.Warn("credential store read failed", "error", err)
		return accounts.Credential{}, mapStoreError(err)
	}
	return decodeSecret(value)
}

func (store *Store) Set(ctx context.Context, accountID string, secret accounts.Credential) error {
	if err := validateRequest(ctx, accountID); err != nil {
		return err
	}
	value, err := encodeSecret(secret)
	if err != nil {
		return err
	}
	if err := store.backend.Set(ServiceNamespace, accountID, value); err != nil {
		slog.Warn("credential store write failed", "error", err)
		return mapStoreError(err)
	}
	return nil
}

func (store *Store) Delete(ctx context.Context, accountID string) error {
	if err := validateRequest(ctx, accountID); err != nil {
		return err
	}
	if err := store.backend.Delete(ServiceNamespace, accountID); err != nil {
		return mapStoreError(err)
	}
	return nil
}

func validateRequest(ctx context.Context, accountID string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: operation cancelled", accounts.ErrStoreUnavailable)
	}
	if accountID == "" || len(accountID) > 128 || strings.ContainsAny(accountID, "\x00\r\n") {
		return fmt.Errorf("%w: invalid account identifier", accounts.ErrPermissionDenied)
	}
	return nil
}

func mapStoreError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, keyring.ErrNotFound) {
		return accounts.ErrCredentialsNotFound
	}
	if errors.Is(err, keyring.ErrUnsupportedPlatform) {
		return accounts.ErrStoreUnavailable
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "locked"), strings.Contains(message, "islocked"):
		return fmt.Errorf("%w: native credential store is locked", accounts.ErrStoreLocked)
	case strings.Contains(message, "denied"), strings.Contains(message, "permission"), strings.Contains(message, "access is denied"):
		return fmt.Errorf("%w: native credential store denied access", accounts.ErrPermissionDenied)
	default:
		return fmt.Errorf("%w: native credential store operation failed", accounts.ErrStoreUnavailable)
	}
}
