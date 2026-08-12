package accounts

import (
	"context"
	"errors"
	"time"
)

type Status string

const (
	StatusValid       Status = "valid"
	StatusExpired     Status = "expired"
	StatusUnknown     Status = "unknown"
	StatusNeedsReauth Status = "needs_reauth"
)

type Account struct {
	ID               string     `json:"id"`
	Username         string     `json:"username"`
	DisplayName      string     `json:"displayName"`
	Email            string     `json:"email"`
	UID              string     `json:"uid"`
	SessionKey       string     `json:"-"`
	SessionSignature string     `json:"-"`
	Status           Status     `json:"status"`
	IsDefault        bool       `json:"isDefault"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	LastValidatedAt  *time.Time `json:"lastValidatedAt,omitempty"`
}

const (
	LoginStatusSuccess            = "success"
	LoginStatusTOTPRequired       = "totp_required"
	LoginStatusInvalidCredentials = "invalid_credentials"
	LoginStatusIPChanged          = "ip_changed"
	LoginStatusTemporarilyBlocked = "temporarily_blocked"
	LoginStatusNetworkError       = "network_error"
	LoginStatusServerError        = "server_error"
	LoginStatusInvalidResponse    = "invalid_response"
	LoginStatusUnknownError       = "unknown_error"
)

type LoginResult struct {
	Status  string   `json:"status"`
	Account *Account `json:"account,omitempty"`
	FlowID  string   `json:"flowId,omitempty"`
	Message string   `json:"message,omitempty"`
}

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrTOTPRequired        = errors.New("totp required")
	ErrIPChanged           = errors.New("ip changed")
	ErrTemporarilyBlocked  = errors.New("temporarily blocked")
	ErrInvalidAuthReply    = errors.New("invalid auth response")
	ErrAuthNetwork         = errors.New("auth network error")
	ErrAuthServer          = errors.New("auth server error")
	ErrUnknownAuth         = errors.New("unknown auth error")
	ErrCredentialsNotFound = errors.New("account secret not found")
	ErrStoreLocked         = errors.New("credential store locked")
	ErrStoreUnavailable    = errors.New("credential store unavailable")
	ErrPermissionDenied    = errors.New("credential store permission denied")
	ErrCorruptCredentials  = errors.New("stored account secret is corrupt")
)

type Session struct {
	SessionKey       string
	SessionSignature string
	UID              string
	PlayerName       string
}

type TOTPChallenge struct {
	PreLoginToken string
}

type Credential struct {
	SessionKey       string `json:"-"`
	SessionSignature string `json:"-"`
}

type Repository interface {
	ListAccounts(context.Context) ([]Account, error)
	GetAccount(context.Context, string) (Account, error)
	SaveAccount(context.Context, Account) error
	SaveAccountAndSelect(context.Context, Account, bool) error
	SetDefaultAccount(context.Context, string) error
	DeleteAccount(context.Context, string) error
}

type Authenticator interface {
	Login(context.Context, string, string, string, string) (Session, *TOTPChallenge, error)
	Validate(context.Context, string, string) (bool, error)
}

type Credentials interface {
	Get(context.Context, string) (Credential, error)
	Set(context.Context, string, Credential) error
	Delete(context.Context, string) error
}

type PendingCredentials interface {
	MarkPending(context.Context, string) error
	ClearPending(context.Context, string) error
}

type InstanceCleanup func(context.Context, string) error

type MutationGate interface {
	Begin() error
	End()
}

// AuthFailureReporter receives only the fixed, privacy-safe authentication
// server-unavailable category. It never receives credentials or raw errors.
type AuthFailureReporter func(context.Context)
