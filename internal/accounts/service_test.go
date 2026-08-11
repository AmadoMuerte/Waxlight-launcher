package accounts_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/platform/sqlite"
)

type fakeAuthClient struct {
	mu               sync.Mutex
	session          accounts.Session
	challenge        bool
	loginErr         error
	validateResult   bool
	validateErr      error
	lastPassword     string
	lastTOTP         string
	lastPreLogin     string
	validationUID    string
	validationSecret string
}

func (client *fakeAuthClient) Login(
	_ context.Context,
	_ string,
	password string,
	totp string,
	preLogin string,
) (accounts.Session, *accounts.TOTPChallenge, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.lastPassword = password
	client.lastTOTP = totp
	client.lastPreLogin = preLogin
	if client.challenge && totp == "" {
		return accounts.Session{}, &accounts.TOTPChallenge{PreLoginToken: "pre-login"}, accounts.ErrTOTPRequired
	}
	return client.session, nil, client.loginErr
}

func (client *fakeAuthClient) Validate(
	_ context.Context,
	uid string,
	sessionKey string,
) (bool, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.validationUID = uid
	client.validationSecret = sessionKey
	return client.validateResult, client.validateErr
}

type memorySecretStore struct {
	values    map[string]accounts.Credential
	setErr    error
	deleteErr error
}

func newMemorySecretStore() *memorySecretStore {
	return &memorySecretStore{values: map[string]accounts.Credential{}}
}

func (store *memorySecretStore) Get(_ context.Context, id string) (accounts.Credential, error) {
	secret, ok := store.values[id]
	if !ok {
		return accounts.Credential{}, accounts.ErrCredentialsNotFound
	}
	return secret, nil
}

func (store *memorySecretStore) Set(_ context.Context, id string, secret accounts.Credential) error {
	if store.setErr != nil {
		return store.setErr
	}
	store.values[id] = secret
	return nil
}

func (store *memorySecretStore) Delete(_ context.Context, id string) error {
	if store.deleteErr != nil {
		return store.deleteErr
	}
	delete(store.values, id)
	return nil
}

type failingAccountCommitStore struct {
	accounts.Repository
	err error
}

func (store failingAccountCommitStore) SaveAccountAndSelect(context.Context, accounts.Account, bool) error {
	return store.err
}

func newService(repository accounts.Repository, client accounts.Authenticator, credentials accounts.Credentials) *accounts.Service {
	return accounts.NewService(repository, client, credentials, nil, nil, nil)
}

func newAccountFixture(t *testing.T) (*accounts.Service, *sqlite.SQLiteStore, *fakeAuthClient, *memorySecretStore) {
	t.Helper()
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "accounts.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	client := &fakeAuthClient{
		session: accounts.Session{
			SessionKey:       "session-key",
			SessionSignature: "signature",
			UID:              "server-uid",
			PlayerName:       "Waxlighter",
		},
		validateResult: true,
	}
	secrets := newMemorySecretStore()
	return newService(store, client, secrets), store, client, secrets
}

func TestAccountLoginStoresMetadataAndSecretSeparately(t *testing.T) {
	service, store, _, secrets := newAccountFixture(t)
	result, err := service.Login(context.Background(), "player@example.com", "password")
	if err != nil || result.Status != accounts.LoginStatusSuccess || result.Account == nil {
		t.Fatalf("unexpected login result: %#v, %v", result, err)
	}
	if result.Account.SessionKey != "" || result.Account.SessionSignature != "" {
		t.Fatal("login result exposed session secrets")
	}
	encoded, err := json.Marshal(result.Account)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" || containsAny(string(encoded), "session-key", "signature") {
		t.Fatalf("serialized account contains a secret: %s", encoded)
	}
	stored, err := store.GetAccount(context.Background(), result.Account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.UID != "server-uid" || stored.Email != "player@example.com" || !stored.IsDefault {
		t.Fatalf("unexpected stored metadata: %#v", stored)
	}
	secret := secrets.values[stored.ID]
	if secret.SessionKey != "session-key" || secret.SessionSignature != "signature" {
		t.Fatalf("unexpected separate secret: %#v", secret)
	}
}

func TestDatabaseContainsNoAuthenticationSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.db")
	metadata, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeAuthClient{session: accounts.Session{
		SessionKey:       "WAXLIGHT_TEST_SESSION_KEY_DO_NOT_LEAK",
		SessionSignature: "WAXLIGHT_TEST_SIGNATURE_DO_NOT_LEAK",
		UID:              "server-uid", PlayerName: "Waxlighter",
	}}
	service := newService(metadata, client, newMemorySecretStore())
	if _, err := service.Login(context.Background(), "player@example.com", "WAXLIGHT_TEST_PASSWORD_DO_NOT_LEAK"); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Close(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, sentinel := range []string{"WAXLIGHT_TEST_SESSION_KEY_DO_NOT_LEAK", "WAXLIGHT_TEST_SIGNATURE_DO_NOT_LEAK", "WAXLIGHT_TEST_PASSWORD_DO_NOT_LEAK"} {
		if strings.Contains(string(contents), sentinel) {
			t.Fatalf("database contains secret sentinel %s", sentinel)
		}
	}
}

func TestAccountLoginUpdatesByUIDAndSupportsSeveralAccounts(t *testing.T) {
	service, _, client, _ := newAccountFixture(t)
	first, err := service.Login(context.Background(), "old@example.com", "password")
	if err != nil {
		t.Fatal(err)
	}
	client.session.PlayerName = "Renamed"
	updated, err := service.Login(context.Background(), "new@example.com", "password")
	if err != nil {
		t.Fatal(err)
	}
	if first.Account == nil || updated.Account == nil || first.Account.ID != updated.Account.ID {
		t.Fatal("the same UID created a duplicate account")
	}
	client.session.UID = "second-uid"
	client.session.PlayerName = "Second"
	if _, err := service.Login(context.Background(), "second@example.com", "password"); err != nil {
		t.Fatal(err)
	}
	accounts, err := service.ListAccounts(context.Background())
	if err != nil || len(accounts) != 2 {
		t.Fatalf("expected two accounts, got %d, %v", len(accounts), err)
	}
}

func TestConcurrentDuplicateLoginCreatesOneAccount(t *testing.T) {
	service, _, _, _ := newAccountFixture(t)
	var wait sync.WaitGroup
	errorsFound := make(chan error, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := service.Login(context.Background(), "player@example.com", "password")
			errorsFound <- err
		}()
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	accounts, err := service.ListAccounts(context.Background())
	if err != nil || len(accounts) != 1 {
		t.Fatalf("concurrent duplicate created %d accounts: %v", len(accounts), err)
	}
}

func TestLoginRollsBackCredentialWhenSecretOrMetadataCommitFails(t *testing.T) {
	service, metadata, client, secrets := newAccountFixture(t)
	secrets.setErr = accounts.ErrStoreLocked
	result, err := service.Login(context.Background(), "player@example.com", "WAXLIGHT_TEST_PASSWORD_DO_NOT_LEAK")
	if err == nil || result.Account != nil {
		t.Fatalf("expected closed credential-store failure: %#v, %v", result, err)
	}
	accounts, listErr := metadata.ListAccounts(context.Background())
	if listErr != nil || len(accounts) != 0 {
		t.Fatalf("metadata committed after secret failure: %#v, %v", accounts, listErr)
	}
	if strings.Contains(err.Error(), "WAXLIGHT_TEST_PASSWORD_DO_NOT_LEAK") {
		t.Fatalf("password leaked in error: %v", err)
	}

	secrets.setErr = nil
	wrapped := failingAccountCommitStore{Repository: metadata, err: errors.New("injected metadata failure")}
	service = newService(wrapped, client, secrets)
	result, err = service.Login(context.Background(), "player@example.com", "password")
	if err == nil || result.Account != nil {
		t.Fatalf("expected metadata failure: %#v, %v", result, err)
	}
	if len(secrets.values) != 0 {
		t.Fatalf("orphaned credential after metadata failure: %#v", secrets.values)
	}
}

func TestReauthenticationRestoresPreviousCredentialOnMetadataFailure(t *testing.T) {
	service, metadata, client, secrets := newAccountFixture(t)
	created, err := service.Login(context.Background(), "player@example.com", "password")
	if err != nil || created.Account == nil {
		t.Fatal(err)
	}
	id := created.Account.ID
	previous := secrets.values[id]
	client.session.SessionKey = "replacement-key"
	client.session.SessionSignature = "replacement-signature"
	failing := newService(
		failingAccountCommitStore{Repository: metadata, err: errors.New("selection commit failed")},
		client, secrets,
	)
	if _, err := failing.ReauthenticateAccount(context.Background(), id, "player@example.com", "password"); err == nil {
		t.Fatal("expected transactional metadata failure")
	}
	if got := secrets.values[id]; got != previous {
		t.Fatalf("previous credential was not restored: %#v", got)
	}
}

func TestTOTPFlowUsesOpaqueIDAndCanBeCancelled(t *testing.T) {
	service, _, client, _ := newAccountFixture(t)
	client.challenge = true
	result, err := service.Login(context.Background(), "player@example.com", "password")
	if err != nil || result.Status != accounts.LoginStatusTOTPRequired || result.FlowID == "" {
		t.Fatalf("unexpected TOTP result: %#v, %v", result, err)
	}
	if result.FlowID == "pre-login" || result.FlowID == "password" {
		t.Fatal("flow ID exposed a secret")
	}
	completed, err := service.CompleteTOTP(context.Background(), result.FlowID, "123456")
	if err != nil || completed.Status != accounts.LoginStatusSuccess {
		t.Fatalf("unexpected completed flow: %#v, %v", completed, err)
	}
	if client.lastTOTP != "123456" || client.lastPreLogin != "pre-login" || client.lastPassword != "password" {
		t.Fatal("TOTP continuation fields were not retained in the backend flow")
	}
	if _, err := service.CompleteTOTP(context.Background(), result.FlowID, "123456"); err == nil {
		t.Fatal("successful flow was not removed")
	}

	result, err = service.Login(context.Background(), "player@example.com", "password")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.CancelLogin(result.FlowID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteTOTP(context.Background(), result.FlowID, "123456"); err == nil {
		t.Fatal("cancelled flow was not removed")
	}
}

func TestAccountSelectionValidationAndRemoval(t *testing.T) {
	service, store, client, secrets := newAccountFixture(t)
	result, err := service.Login(context.Background(), "player@example.com", "password")
	if err != nil || result.Account == nil {
		t.Fatal(err)
	}
	id := result.Account.ID
	if err := service.SelectAccount(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	validated, err := service.ValidateAccount(context.Background(), id)
	if err != nil || validated.Status != accounts.StatusValid {
		t.Fatalf("unexpected validation: %#v, %v", validated, err)
	}
	if client.validationUID != "server-uid" || client.validationSecret != "session-key" {
		t.Fatal("validation did not use the stored session")
	}

	client.validateResult = false
	_, err = service.ValidateAccount(context.Background(), id)
	if err == nil {
		t.Fatal("expected expired session error")
	}
	stored, err := store.GetAccount(context.Background(), id)
	if err != nil || stored.Status != accounts.StatusExpired {
		t.Fatalf("expired status was not stored: %#v, %v", stored, err)
	}

	stored.Status = accounts.StatusValid
	if err := store.SaveAccount(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	client.validateErr = accounts.ErrAuthNetwork
	if _, err := service.ValidateAccount(context.Background(), id); err == nil {
		t.Fatal("expected network error")
	}
	stored, _ = store.GetAccount(context.Background(), id)
	if stored.Status != accounts.StatusValid {
		t.Fatal("network failure incorrectly expired the session")
	}

	if err := service.RemoveAccount(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if _, ok := secrets.values[id]; ok {
		t.Fatal("account secret was not removed")
	}
	if _, err := store.GetAccount(context.Background(), id); err == nil {
		t.Fatal("account metadata was not removed")
	}
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

var _ accounts.Authenticator = (*fakeAuthClient)(nil)
var _ accounts.Credentials = (*memorySecretStore)(nil)
