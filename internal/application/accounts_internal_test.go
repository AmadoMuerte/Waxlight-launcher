package application

import (
	"context"
	"testing"
	"time"
)

type expiringAuthClient struct{}

func (expiringAuthClient) Login(context.Context, string, string, string, string) (AuthSession, *TOTPChallenge, error) {
	return AuthSession{}, &TOTPChallenge{PreLoginToken: "WAXLIGHT_TEST_PRELOGIN_DO_NOT_LEAK"}, ErrTOTPRequired
}
func (expiringAuthClient) Validate(context.Context, string, string) (bool, error) { return false, nil }

type unusedStore struct{ Store }
type unusedSecrets struct{}

func (unusedSecrets) Get(context.Context, string) (Secret, error) { return Secret{}, ErrSecretNotFound }
func (unusedSecrets) Set(context.Context, string, Secret) error   { return nil }
func (unusedSecrets) Delete(context.Context, string) error        { return nil }

func TestLoginFlowExpiresAndClearsSecrets(t *testing.T) {
	service := NewAccountService(unusedStore{}, expiringAuthClient{}, unusedSecrets{})
	now := time.Now().UTC()
	service.now = func() time.Time { return now }
	service.flowTTL = time.Second
	result, err := service.Login(context.Background(), "player@example.com", "WAXLIGHT_TEST_PASSWORD_DO_NOT_LEAK")
	if err != nil || result.FlowID == "" {
		t.Fatalf("unexpected flow: %#v, %v", result, err)
	}
	flow := service.loginFlow[result.FlowID]
	now = now.Add(2 * time.Second)
	if _, err := service.CompleteTOTP(context.Background(), result.FlowID, "123456"); err == nil {
		t.Fatal("expired flow was accepted")
	}
	if flow.Password != "" || flow.PreLoginToken != "" {
		t.Fatal("expired flow secrets were not cleared")
	}
}
