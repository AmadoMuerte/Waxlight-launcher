package auth

import (
	"errors"
	"fmt"
)

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

type InvalidResponseError struct {
	StatusCode  int
	ContentType string
	BodySize    int
	Cause       error
}

func (err *InvalidResponseError) Error() string {
	return fmt.Sprintf(
		"unexpected authentication response: status=%d content-type=%q size=%d",
		err.StatusCode,
		err.ContentType,
		err.BodySize,
	)
}

func (err *InvalidResponseError) Unwrap() error {
	return err.Cause
}
