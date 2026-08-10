package application

import "errors"

// Authentication errors are application-level outcomes. Infrastructure maps
// Vintage Story protocol errors here so callers never depend on a provider.
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTOTPRequired       = errors.New("totp required")
	ErrIPChanged          = errors.New("ip changed")
	ErrTemporarilyBlocked = errors.New("temporarily blocked")
	ErrInvalidAuthReply   = errors.New("invalid auth response")
	ErrAuthNetwork        = errors.New("auth network error")
	ErrAuthServer         = errors.New("auth server error")
	ErrUnknownAuth        = errors.New("unknown auth error")
)

type AuthSession struct {
	SessionKey       string
	SessionSignature string
	UID              string
	PlayerName       string
}

type TOTPChallenge struct {
	PreLoginToken string
}
