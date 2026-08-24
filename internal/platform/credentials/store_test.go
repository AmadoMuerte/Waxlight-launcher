package credentials

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
	keyring "github.com/zalando/go-keyring"
)

type memoryBackend struct {
	values map[string]string
	err    error
}

func (backend *memoryBackend) key(service, user string) string { return service + ":" + user }
func (backend *memoryBackend) Get(service, user string) (string, error) {
	if backend.err != nil {
		return "", backend.err
	}
	value, ok := backend.values[backend.key(service, user)]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value, nil
}
func (backend *memoryBackend) Set(service, user, password string) error {
	if backend.err != nil {
		return backend.err
	}
	backend.values[backend.key(service, user)] = password
	return nil
}
func (backend *memoryBackend) Delete(service, user string) error {
	if backend.err != nil {
		return backend.err
	}
	key := backend.key(service, user)
	if _, ok := backend.values[key]; !ok {
		return keyring.ErrNotFound
	}
	delete(backend.values, key)
	return nil
}

func TestStoreRoundTripUpdateMultipleAccountsAndDelete(t *testing.T) {
	backend := &memoryBackend{values: map[string]string{}}
	store := newStoreWithBackend(backend)
	ctx := context.Background()
	first := accounts.Credential{SessionKey: "first-key", SessionSignature: "first-signature"}
	second := accounts.Credential{SessionKey: "second-key", SessionSignature: "second-signature"}
	if err := store.Set(ctx, "account-1", first); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(ctx, "account-2", second); err != nil {
		t.Fatal(err)
	}
	first.SessionKey = "updated-key"
	if err := store.Set(ctx, "account-1", first); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, "account-1")
	if err != nil || got != first {
		t.Fatalf("unexpected secret: %#v, %v", got, err)
	}
	got, err = store.Get(ctx, "account-2")
	if err != nil || got != second {
		t.Fatalf("unexpected second secret: %#v, %v", got, err)
	}
	if err := store.Delete(ctx, "account-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "account-1"); !errors.Is(err, accounts.ErrCredentialsNotFound) {
		t.Fatalf("unexpected missing error: %v", err)
	}
	if err := store.Delete(ctx, "account-1"); !errors.Is(err, accounts.ErrCredentialsNotFound) {
		t.Fatalf("unexpected repeated delete: %v", err)
	}
}

func TestStoreRejectsCorruptAndUnsupportedSecrets(t *testing.T) {
	backend := &memoryBackend{values: map[string]string{}}
	store := newStoreWithBackend(backend)
	for name, value := range map[string]string{
		"malformed":       `{`,
		"unknown-version": `{"version":2,"sessionKey":"key","sessionSignature":"sig"}`,
		"unknown-field":   `{"version":1,"sessionKey":"key","sessionSignature":"sig","password":"leak"}`,
	} {
		t.Run(name, func(t *testing.T) {
			backend.values[backend.key(ServiceNamespace, name)] = value
			if _, err := store.Get(context.Background(), name); !errors.Is(err, accounts.ErrCorruptCredentials) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestStoreMapsLockedDeniedUnavailableAndCancellation(t *testing.T) {
	for name, tc := range map[string]struct {
		source error
		target error
	}{
		"locked":      {errors.New("collection is locked"), accounts.ErrStoreLocked},
		"denied":      {errors.New("access denied"), accounts.ErrPermissionDenied},
		"unavailable": {errors.New("dbus connection failed"), accounts.ErrStoreUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			store := newStoreWithBackend(&memoryBackend{values: map[string]string{}, err: tc.source})
			if err := store.Set(context.Background(), "account", accounts.Credential{SessionKey: "key", SessionSignature: "sig"}); !errors.Is(err, tc.target) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	store := newStoreWithBackend(&memoryBackend{values: map[string]string{}})
	if _, err := store.Get(cancelled, "account"); !errors.Is(err, accounts.ErrStoreUnavailable) {
		t.Fatalf("unexpected cancellation error: %v", err)
	}
}

func TestSecretEncodingDoesNotUseLegacyFieldNames(t *testing.T) {
	value, err := encodeSecret(accounts.Credential{SessionKey: "key", SessionSignature: "signature"})
	if err != nil {
		t.Fatal(err)
	}
	if value != `{"version":1,"sessionKey":"key","sessionSignature":"signature"}` {
		t.Fatalf("unexpected encoding: %s", value)
	}
}

func TestPendingCredentialReconciliationDeletesOnlyOrphans(t *testing.T) {
	backend := &memoryBackend{values: map[string]string{}}
	store := &Store{backend: backend, pendingPath: filepath.Join(t.TempDir(), "security", "pending.json")}
	ctx := context.Background()
	secret := accounts.Credential{SessionKey: "key", SessionSignature: "signature"}
	for _, id := range []string{"committed", "orphan"} {
		if err := store.MarkPending(ctx, id); err != nil {
			t.Fatal(err)
		}
		if err := store.Set(ctx, id, secret); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.ReconcilePending(ctx, []string{"committed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "committed"); err != nil {
		t.Fatalf("committed secret was deleted: %v", err)
	}
	if _, err := store.Get(ctx, "orphan"); !errors.Is(err, accounts.ErrCredentialsNotFound) {
		t.Fatalf("orphan was retained: %v", err)
	}
	if _, err := os.Stat(store.pendingPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending journal remains: %v", err)
	}
}
