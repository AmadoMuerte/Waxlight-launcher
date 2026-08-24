package vintagestory

import (
	"context"
	"errors"
	"net/http"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
	vsauth "github.com/AmadoMuerte/vintagestory-go/auth"
)

// AuthClient isolates the Vintage Story authentication protocol from the
// application-owned account, credential, and user-facing error model.
type AuthClient struct {
	client *vsauth.Client
}

func NewAuthClient(client *http.Client) *AuthClient {
	return &AuthClient{client: vsauth.NewClient(client)}
}

// NewAuthClientWithURLs is intended for tests and controlled integrations.
func NewAuthClientWithURLs(client *http.Client, loginURL, validateURL string) *AuthClient {
	return &AuthClient{client: vsauth.NewClientWithURLs(client, loginURL, validateURL)}
}

func (client *AuthClient) Login(ctx context.Context, email, password, totpCode, preLoginToken string) (accounts.Session, *accounts.TOTPChallenge, error) {
	session, challenge, err := client.client.Login(ctx, email, password, totpCode, preLoginToken)
	var mappedChallenge *accounts.TOTPChallenge
	if challenge != nil {
		mappedChallenge = &accounts.TOTPChallenge{PreLoginToken: challenge.PreLoginToken}
	}
	if err != nil {
		return accounts.Session{}, mappedChallenge, mapAuthError(err)
	}
	return accounts.Session{SessionKey: session.SessionKey, SessionSignature: session.SessionSignature, UID: session.UID, PlayerName: session.PlayerName}, nil, nil
}

func (client *AuthClient) Validate(ctx context.Context, uid, sessionKey string) (bool, error) {
	valid, err := client.client.Validate(ctx, uid, sessionKey)
	return valid, mapAuthError(err)
}

func mapAuthError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, vsauth.ErrInvalidCredentials):
		return accounts.ErrInvalidCredentials
	case errors.Is(err, vsauth.ErrTOTPRequired):
		return accounts.ErrTOTPRequired
	case errors.Is(err, vsauth.ErrIPChanged):
		return accounts.ErrIPChanged
	case errors.Is(err, vsauth.ErrTemporarilyBlocked):
		return accounts.ErrTemporarilyBlocked
	case errors.Is(err, vsauth.ErrInvalidAuthReply):
		return accounts.ErrInvalidAuthReply
	case errors.Is(err, vsauth.ErrNetwork):
		return accounts.ErrAuthNetwork
	case errors.Is(err, vsauth.ErrServer):
		return accounts.ErrAuthServer
	default:
		return accounts.ErrUnknownAuth
	}
}
