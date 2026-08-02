package application_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/auth"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/database"
)

type fakeAuthClient struct {
	session          auth.Session
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
) (auth.Session, *auth.TOTPChallenge, error) {
	client.lastPassword = password
	client.lastTOTP = totp
	client.lastPreLogin = preLogin
	if client.challenge && totp == "" {
		return auth.Session{}, &auth.TOTPChallenge{PreLoginToken: "pre-login"}, auth.ErrTOTPRequired
	}
	return client.session, nil, client.loginErr
}

func (client *fakeAuthClient) Validate(
	_ context.Context,
	uid string,
	sessionKey string,
) (bool, error) {
	client.validationUID = uid
	client.validationSecret = sessionKey
	return client.validateResult, client.validateErr
}

type memorySecretStore struct {
	values map[string]application.Secret
}

func newMemorySecretStore() *memorySecretStore {
	return &memorySecretStore{values: map[string]application.Secret{}}
}

func (store *memorySecretStore) Get(id string) (application.Secret, error) {
	secret, ok := store.values[id]
	if !ok {
		return application.Secret{}, application.ErrSecretNotFound
	}
	return secret, nil
}

func (store *memorySecretStore) Set(id string, secret application.Secret) error {
	store.values[id] = secret
	return nil
}

func (store *memorySecretStore) Delete(id string) error {
	delete(store.values, id)
	return nil
}

func newAccountFixture(t *testing.T) (*application.AccountService, *database.SQLiteStore, *fakeAuthClient, *memorySecretStore) {
	t.Helper()
	store, err := database.Open(filepath.Join(t.TempDir(), "accounts.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	client := &fakeAuthClient{
		session: auth.Session{
			SessionKey:       "session-key",
			SessionSignature: "signature",
			UID:              "server-uid",
			PlayerName:       "Waxlighter",
		},
		validateResult: true,
	}
	secrets := newMemorySecretStore()
	return application.NewAccountService(store, client, secrets), store, client, secrets
}

func TestAccountLoginStoresMetadataAndSecretSeparately(t *testing.T) {
	service, store, _, secrets := newAccountFixture(t)
	result, err := service.Login(context.Background(), "player@example.com", "password")
	if err != nil || result.Status != application.LoginStatusSuccess || result.Account == nil {
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

func TestTOTPFlowUsesOpaqueIDAndCanBeCancelled(t *testing.T) {
	service, _, client, _ := newAccountFixture(t)
	client.challenge = true
	result, err := service.Login(context.Background(), "player@example.com", "password")
	if err != nil || result.Status != application.LoginStatusTOTPRequired || result.FlowID == "" {
		t.Fatalf("unexpected TOTP result: %#v, %v", result, err)
	}
	if result.FlowID == "pre-login" || result.FlowID == "password" {
		t.Fatal("flow ID exposed a secret")
	}
	completed, err := service.CompleteTOTP(context.Background(), result.FlowID, "123456")
	if err != nil || completed.Status != application.LoginStatusSuccess {
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
	if err != nil || validated.Status != domain.AccountStatusValid {
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
	if err != nil || stored.Status != domain.AccountStatusExpired {
		t.Fatalf("expired status was not stored: %#v, %v", stored, err)
	}

	stored.Status = domain.AccountStatusValid
	if err := store.SaveAccount(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	client.validateErr = auth.ErrNetwork
	if _, err := service.ValidateAccount(context.Background(), id); err == nil {
		t.Fatal("expected network error")
	}
	stored, _ = store.GetAccount(context.Background(), id)
	if stored.Status != domain.AccountStatusValid {
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

var _ application.AuthClient = (*fakeAuthClient)(nil)
var _ application.SecretStore = (*memorySecretStore)(nil)
