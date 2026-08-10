package application_test

import (
	"context"
	"sync"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
)

type fakeAuthClient struct {
	mu               sync.Mutex
	session          accounts.Session
	validateResult   bool
	validateErr      error
	validationUID    string
	validationSecret string
}

func (client *fakeAuthClient) Login(context.Context, string, string, string, string) (accounts.Session, *accounts.TOTPChallenge, error) {
	return client.session, nil, nil
}

func (client *fakeAuthClient) Validate(_ context.Context, uid, sessionKey string) (bool, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.validationUID = uid
	client.validationSecret = sessionKey
	return client.validateResult, client.validateErr
}

type memorySecretStore struct {
	values map[string]accounts.Credential
}

func newMemorySecretStore() *memorySecretStore {
	return &memorySecretStore{values: make(map[string]accounts.Credential)}
}

func (store *memorySecretStore) Get(_ context.Context, id string) (accounts.Credential, error) {
	credential, ok := store.values[id]
	if !ok {
		return accounts.Credential{}, accounts.ErrCredentialsNotFound
	}
	return credential, nil
}

func (store *memorySecretStore) Set(_ context.Context, id string, credential accounts.Credential) error {
	store.values[id] = credential
	return nil
}

func (store *memorySecretStore) Delete(_ context.Context, id string) error {
	delete(store.values, id)
	return nil
}

func newTestAccountService(repository accounts.Repository, auth accounts.Authenticator, credentials accounts.Credentials, cleanup accounts.InstanceCleanup) *accounts.Service {
	return accounts.NewService(repository, auth, credentials, nil, cleanup, nil)
}
