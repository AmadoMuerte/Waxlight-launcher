package auth

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTOTPRequired       = errors.New("totp required")
	ErrIPChanged          = errors.New("ip changed")
	ErrTemporarilyBlocked = errors.New("temporarily blocked")
	ErrSessionExpired     = errors.New("session expired")
	ErrInvalidAuthReply   = errors.New("invalid auth response")
	ErrNetwork            = errors.New("auth network error")
	ErrServer             = errors.New("auth server error")
	ErrUnknown            = errors.New("unknown auth error")
)
