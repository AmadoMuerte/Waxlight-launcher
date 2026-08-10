package vintagestory

import (
	"context"
	"errors"
	"net/http"

	vsauth "github.com/AmadoMuerte/vintagestory-go/auth"
	"github.com/waxlight/waxlight-launcher/internal/application"
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

func (client *AuthClient) Login(ctx context.Context, email, password, totpCode, preLoginToken string) (application.AuthSession, *application.TOTPChallenge, error) {
	session, challenge, err := client.client.Login(ctx, email, password, totpCode, preLoginToken)
	var mappedChallenge *application.TOTPChallenge
	if challenge != nil {
		mappedChallenge = &application.TOTPChallenge{PreLoginToken: challenge.PreLoginToken}
	}
	if err != nil {
		return application.AuthSession{}, mappedChallenge, mapAuthError(err)
	}
	return application.AuthSession{SessionKey: session.SessionKey, SessionSignature: session.SessionSignature, UID: session.UID, PlayerName: session.PlayerName}, nil, nil
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
		return application.ErrInvalidCredentials
	case errors.Is(err, vsauth.ErrTOTPRequired):
		return application.ErrTOTPRequired
	case errors.Is(err, vsauth.ErrIPChanged):
		return application.ErrIPChanged
	case errors.Is(err, vsauth.ErrTemporarilyBlocked):
		return application.ErrTemporarilyBlocked
	case errors.Is(err, vsauth.ErrInvalidAuthReply):
		return application.ErrInvalidAuthReply
	case errors.Is(err, vsauth.ErrNetwork):
		return application.ErrAuthNetwork
	case errors.Is(err, vsauth.ErrServer):
		return application.ErrAuthServer
	default:
		return application.ErrUnknownAuth
	}
}
