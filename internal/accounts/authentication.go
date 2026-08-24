package accounts

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/google/uuid"
)

const defaultLoginFlowTTL = 5 * time.Minute

type pendingLogin struct {
	Email             string
	Password          string
	PreLoginToken     string
	ExpectedAccountID string
	ExpiresAt         time.Time
}

type Service struct {
	repository        Repository
	authenticator     Authenticator
	credentials       Credentials
	pending           PendingCredentials
	cleanupInstances  InstanceCleanup
	reportAuthFailure AuthFailureReporter
	mutationGate      MutationGate
	flowTTL           time.Duration
	now               func() time.Time
	flowMu            sync.Mutex
	persistMu         sync.Mutex
	loginFlow         map[string]*pendingLogin
}

func NewService(
	repository Repository,
	authenticator Authenticator,
	credentials Credentials,
	pending PendingCredentials,
	cleanupInstances InstanceCleanup,
	reportAuthFailure AuthFailureReporter,
	mutationGate MutationGate,
) *Service {
	return &Service{
		repository: repository, authenticator: authenticator, credentials: credentials,
		pending: pending, cleanupInstances: cleanupInstances, reportAuthFailure: reportAuthFailure,
		mutationGate: mutationGate,
		flowTTL:      defaultLoginFlowTTL, now: func() time.Time { return time.Now().UTC() },
		loginFlow: make(map[string]*pendingLogin),
	}
}

func (service *Service) beginMutation() (func(), error) {
	if service.mutationGate == nil {
		return func() {}, nil
	}
	if err := service.mutationGate.Begin(); err != nil {
		return nil, err
	}
	return service.mutationGate.End, nil
}

func (service *Service) Login(ctx context.Context, email, password string) (LoginResult, error) {
	return service.startLogin(ctx, "", email, password)
}

func (service *Service) ReauthenticateAccount(ctx context.Context, accountID, email, password string) (LoginResult, error) {
	if _, err := service.repository.GetAccount(ctx, accountID); err != nil {
		return LoginResult{}, err
	}
	return service.startLogin(ctx, accountID, email, password)
}

func (service *Service) startLogin(ctx context.Context, expectedAccountID, email, password string) (LoginResult, error) {
	email = strings.TrimSpace(email)
	if email == "" || !strings.Contains(email, "@") {
		return LoginResult{}, errs.NewError(errs.ErrValidation, "Enter a valid email address")
	}
	if password == "" {
		return LoginResult{}, errs.NewError(errs.ErrValidation, "Enter your password")
	}

	session, challenge, err := service.authenticator.Login(ctx, email, password, "", "")
	if errors.Is(err, ErrTOTPRequired) && challenge != nil {
		flowID := uuid.NewString()
		service.flowMu.Lock()
		service.purgeExpiredLocked()
		service.loginFlow[flowID] = &pendingLogin{
			Email: email, Password: password, PreLoginToken: challenge.PreLoginToken,
			ExpectedAccountID: expectedAccountID, ExpiresAt: service.now().Add(service.flowTTL),
		}
		service.flowMu.Unlock()
		slog.Info("login requires a TOTP challenge")
		return LoginResult{Status: LoginStatusTOTPRequired, FlowID: flowID}, nil
	}
	if err != nil {
		service.reportFailure(err)
		return loginFailure(err), nil
	}
	return service.persistSession(ctx, expectedAccountID, email, session)
}

func (service *Service) CompleteTOTP(ctx context.Context, flowID, code string) (LoginResult, error) {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 16 {
		return LoginResult{}, errs.NewError(errs.ErrValidation, "Enter the authentication code")
	}
	service.flowMu.Lock()
	service.purgeExpiredLocked()
	flowPointer, ok := service.loginFlow[flowID]
	var flow pendingLogin
	if ok {
		flow = *flowPointer
	}
	service.flowMu.Unlock()
	if !ok {
		return LoginResult{}, errs.NewError(errs.ErrAuthFlowExpired, "The login attempt has expired")
	}
	session, _, err := service.authenticator.Login(ctx, flow.Email, flow.Password, code, flow.PreLoginToken)
	if err != nil {
		service.reportFailure(err)
		return loginFailure(err), nil
	}
	result, err := service.persistSession(ctx, flow.ExpectedAccountID, flow.Email, session)
	if err == nil && result.Status == LoginStatusSuccess {
		service.deleteFlow(flowID)
	}
	return result, err
}

func (service *Service) CancelLogin(flowID string) error {
	service.deleteFlow(flowID)
	return nil
}

func (service *Service) deleteFlow(flowID string) {
	service.flowMu.Lock()
	defer service.flowMu.Unlock()
	if flow, ok := service.loginFlow[flowID]; ok {
		flow.Password = ""
		flow.PreLoginToken = ""
		delete(service.loginFlow, flowID)
	}
}

func (service *Service) purgeExpiredLocked() {
	now := service.now()
	for id, flow := range service.loginFlow {
		if !flow.ExpiresAt.After(now) {
			flow.Password = ""
			flow.PreLoginToken = ""
			delete(service.loginFlow, id)
		}
	}
}

func (service *Service) reportFailure(err error) {
	if service.reportAuthFailure != nil && (errors.Is(err, ErrAuthNetwork) || errors.Is(err, ErrAuthServer)) {
		service.reportAuthFailure(context.Background())
	}
}

func loginFailure(err error) LoginResult {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		return LoginResult{Status: LoginStatusInvalidCredentials}
	case errors.Is(err, ErrIPChanged):
		return LoginResult{Status: LoginStatusIPChanged}
	case errors.Is(err, ErrTemporarilyBlocked):
		return LoginResult{Status: LoginStatusTemporarilyBlocked}
	case errors.Is(err, ErrAuthNetwork):
		return LoginResult{Status: LoginStatusNetworkError}
	case errors.Is(err, ErrAuthServer):
		return LoginResult{Status: LoginStatusServerError}
	case errors.Is(err, ErrInvalidAuthReply):
		return LoginResult{Status: LoginStatusInvalidResponse}
	default:
		return LoginResult{Status: LoginStatusUnknownError}
	}
}
