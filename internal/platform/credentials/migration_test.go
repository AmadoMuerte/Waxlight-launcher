package credentials

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
)

type migrationStore struct {
	values      map[string]accounts.Credential
	setCalls    int
	failAt      int
	readCorrupt bool
	err         error
}

func (store *migrationStore) Get(_ context.Context, id string) (accounts.Credential, error) {
	if store.err != nil {
		return accounts.Credential{}, store.err
	}
	secret, ok := store.values[id]
	if !ok {
		return accounts.Credential{}, accounts.ErrCredentialsNotFound
	}
	if store.readCorrupt {
		secret.SessionKey += "-corrupt"
	}
	return secret, nil
}
func (store *migrationStore) Set(_ context.Context, id string, secret accounts.Credential) error {
	store.setCalls++
	if store.err != nil || (store.failAt > 0 && store.setCalls == store.failAt) {
		if store.err != nil {
			return store.err
		}
		return accounts.ErrStoreUnavailable
	}
	store.values[id] = secret
	return nil
}
func (store *migrationStore) Delete(_ context.Context, id string) error {
	delete(store.values, id)
	return nil
}

func writeLegacy(t *testing.T, root, contents string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(root, "account-secrets.json")
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

const validLegacy = `{"version":1,"secrets":{"account-1":{"sessionkey":"key-1","sessionsignature":"sig-1"},"account-2":{"sessionkey":"key-2","sessionsignature":"sig-2"}}}`

func TestMigrationImportsVerifiesDeletesAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	path := writeLegacy(t, root, validLegacy, 0o600)
	store := &migrationStore{values: map[string]accounts.Credential{}}
	migrator := NewMigrator(root, store)
	if err := migrator.Run(context.Background(), []string{"account-1", "account-2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("plaintext source remains: %v", err)
	}
	if len(store.values) != 2 || store.values["account-1"].SessionKey != "key-1" {
		t.Fatalf("unexpected import: %#v", store.values)
	}
	if err := migrator.Run(context.Background(), []string{"account-1", "account-2"}); err != nil {
		t.Fatalf("repeated migration failed: %v", err)
	}
	if matches, _ := filepath.Glob(path + ".bak*"); len(matches) != 0 {
		t.Fatalf("plaintext backup created: %v", matches)
	}
	state, err := os.ReadFile(filepath.Join(root, "security", "credential-migration.json"))
	if err != nil || !strings.Contains(string(state), `"status":"complete"`) {
		t.Fatalf("unexpected state: %s, %v", state, err)
	}
}

func TestMigrationPartialFailureAndVerificationFailureRetainSource(t *testing.T) {
	for name, store := range map[string]*migrationStore{
		"partial":      {values: map[string]accounts.Credential{}, failAt: 2},
		"verification": {values: map[string]accounts.Credential{}, readCorrupt: true},
		"denied":       {values: map[string]accounts.Credential{}, err: accounts.ErrPermissionDenied},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := writeLegacy(t, root, validLegacy, 0o600)
			if err := NewMigrator(root, store).Run(context.Background(), []string{"account-1", "account-2"}); err == nil {
				t.Fatal("expected migration failure")
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("source was not retained: %v", err)
			}
		})
	}
}

func TestMigrationPreservesCredentialStoreErrorCategory(t *testing.T) {
	for name, want := range map[string]error{
		"locked":  accounts.ErrStoreLocked,
		"denied":  accounts.ErrPermissionDenied,
		"offline": accounts.ErrStoreUnavailable,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeLegacy(t, root, validLegacy, 0o600)
			store := &migrationStore{values: map[string]accounts.Credential{}, err: want}
			err := NewMigrator(root, store).Run(context.Background(), []string{"account-1", "account-2"})
			if !errors.Is(err, want) {
				t.Fatalf("migration error lost credential store category: %v", err)
			}
		})
	}
}

func TestMigrationRejectsMalformedOversizedAndUnknownAccounts(t *testing.T) {
	oversized := strings.Repeat("x", maxLegacyFileBytes+1)
	for name, contents := range map[string]string{
		"malformed":       `{`,
		"unknown-field":   `{"version":1,"secrets":{},"password":"bad"}`,
		"oversized":       oversized,
		"unknown-account": validLegacy,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := writeLegacy(t, root, contents, 0o600)
			ids := []string{"account-1", "account-2"}
			if name == "unknown-account" {
				ids = []string{"account-1"}
			}
			store := &migrationStore{values: map[string]accounts.Credential{}}
			if err := NewMigrator(root, store).Run(context.Background(), ids); err == nil {
				t.Fatal("expected rejection")
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("source was not retained: %v", err)
			}
			if len(store.values) != 0 {
				t.Fatalf("secrets imported before validation: %#v", store.values)
			}
		})
	}
}
