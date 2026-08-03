package application

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/waxlight/waxlight-launcher/internal/auth"
	"github.com/waxlight/waxlight-launcher/internal/domain"
)

const defaultLoginFlowTTL = 5 * time.Minute

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
	Status  string          `json:"status"`
	Account *domain.Account `json:"account,omitempty"`
	FlowID  string          `json:"flowId,omitempty"`
	Message string          `json:"message,omitempty"`
}

type PendingLogin struct {
	Email             string
	Password          string
	PreLoginToken     string
	ExpectedAccountID string
	ExpiresAt         time.Time
}

type AccountService struct {
	store            Store
	client           AuthClient
	secrets          SecretStore
	flowTTL          time.Duration
	now              func() time.Time
	flowMu           sync.Mutex
	persistMu        sync.Mutex
	loginFlow        map[string]*PendingLogin
	cleanupInstances func(context.Context, string) error
}

func (service *AccountService) ConfigureInstanceCleanup(cleanup func(context.Context, string) error) {
	service.cleanupInstances = cleanup
}

func NewAccountService(store Store, client AuthClient, secrets SecretStore) *AccountService {
	return &AccountService{
		store:     store,
		client:    client,
		secrets:   secrets,
		flowTTL:   defaultLoginFlowTTL,
		now:       func() time.Time { return time.Now().UTC() },
		loginFlow: make(map[string]*PendingLogin),
	}
}

func (service *AccountService) Login(
	ctx context.Context,
	email string,
	password string,
) (LoginResult, error) {
	return service.startLogin(ctx, "", email, password)
}

func (service *AccountService) ReauthenticateAccount(
	ctx context.Context,
	accountID string,
	email string,
	password string,
) (LoginResult, error) {
	if _, err := service.store.GetAccount(ctx, accountID); err != nil {
		return LoginResult{}, err
	}
	return service.startLogin(ctx, accountID, email, password)
}

func (service *AccountService) startLogin(
	ctx context.Context,
	expectedAccountID string,
	email string,
	password string,
) (LoginResult, error) {
	email = strings.TrimSpace(email)
	if email == "" || !strings.Contains(email, "@") {
		return LoginResult{}, domain.NewError(domain.ErrValidation, "Enter a valid email address")
	}
	if password == "" {
		return LoginResult{}, domain.NewError(domain.ErrValidation, "Enter your password")
	}

	session, challenge, err := service.client.Login(ctx, email, password, "", "")
	if errors.Is(err, auth.ErrTOTPRequired) && challenge != nil {
		flowID := uuid.NewString()
		service.flowMu.Lock()
		service.purgeExpiredLocked()
		service.loginFlow[flowID] = &PendingLogin{
			Email:             email,
			Password:          password,
			PreLoginToken:     challenge.PreLoginToken,
			ExpectedAccountID: expectedAccountID,
			ExpiresAt:         service.now().Add(service.flowTTL),
		}
		service.flowMu.Unlock()
		return LoginResult{Status: LoginStatusTOTPRequired, FlowID: flowID}, nil
	}
	if err != nil {
		return loginFailure(err), nil
	}
	return service.persistSession(ctx, expectedAccountID, email, session)
}

func (service *AccountService) CompleteTOTP(
	ctx context.Context,
	flowID string,
	code string,
) (LoginResult, error) {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 16 {
		return LoginResult{}, domain.NewError(domain.ErrValidation, "Enter the authentication code")
	}

	service.flowMu.Lock()
	service.purgeExpiredLocked()
	flowPointer, ok := service.loginFlow[flowID]
	var flow PendingLogin
	if ok {
		flow = *flowPointer
	}
	service.flowMu.Unlock()
	if !ok {
		return LoginResult{}, domain.NewError(domain.ErrAuthFlowExpired, "The login attempt has expired")
	}

	session, _, err := service.client.Login(
		ctx,
		flow.Email,
		flow.Password,
		code,
		flow.PreLoginToken,
	)
	if err != nil {
		return loginFailure(err), nil
	}

	result, err := service.persistSession(ctx, flow.ExpectedAccountID, flow.Email, session)
	if err == nil && result.Status == LoginStatusSuccess {
		service.deleteFlow(flowID)
	}
	return result, err
}

func (service *AccountService) CancelLogin(flowID string) error {
	service.deleteFlow(flowID)
	return nil
}

func (service *AccountService) deleteFlow(flowID string) {
	service.flowMu.Lock()
	defer service.flowMu.Unlock()
	if flow, ok := service.loginFlow[flowID]; ok {
		flow.Password = ""
		flow.PreLoginToken = ""
		delete(service.loginFlow, flowID)
	}
}

func (service *AccountService) purgeExpiredLocked() {
	now := service.now()
	for id, flow := range service.loginFlow {
		if !flow.ExpiresAt.After(now) {
			flow.Password = ""
			flow.PreLoginToken = ""
			delete(service.loginFlow, id)
		}
	}
}

func (service *AccountService) persistSession(
	ctx context.Context,
	expectedAccountID string,
	email string,
	session auth.Session,
) (LoginResult, error) {
	service.persistMu.Lock()
	defer service.persistMu.Unlock()
	accounts, err := service.store.ListAccounts(ctx)
	if err != nil {
		return LoginResult{}, err
	}

	var account domain.Account
	found := false
	for _, candidate := range accounts {
		if candidate.UID == session.UID && candidate.UID != "" {
			account = candidate
			found = true
			break
		}
	}
	if expectedAccountID != "" {
		expected, err := service.store.GetAccount(ctx, expectedAccountID)
		if err != nil {
			return LoginResult{}, err
		}
		if expected.UID != "" && expected.UID != session.UID {
			return LoginResult{Status: LoginStatusInvalidCredentials}, nil
		}
		account = expected
		found = true
	}

	now := service.now()
	if !found {
		account.ID = uuid.NewString()
		account.CreatedAt = now
		account.IsDefault = len(accounts) == 0
	}
	account.Username = session.PlayerName
	account.DisplayName = session.PlayerName
	account.Email = email
	account.UID = session.UID
	account.Status = domain.AccountStatusValid
	account.UpdatedAt = now
	account.LastValidatedAt = &now

	secret := Secret{
		SessionKey:       session.SessionKey,
		SessionSignature: session.SessionSignature,
	}
	previousSecret, previousErr := service.secrets.Get(ctx, account.ID)
	hadPreviousSecret := previousErr == nil
	if previousErr != nil && !errors.Is(previousErr, ErrSecretNotFound) {
		return LoginResult{}, secretStoreError("Could not read the existing account session", previousErr)
	}
	pendingStore, supportsCrashRecovery := service.secrets.(PendingSecretStore)
	if supportsCrashRecovery {
		if err := pendingStore.MarkPending(ctx, account.ID); err != nil {
			return LoginResult{}, secretStoreError("Could not prepare the account session commit", err)
		}
	}
	if err := service.secrets.Set(ctx, account.ID, secret); err != nil {
		if supportsCrashRecovery {
			_ = pendingStore.ClearPending(context.Background(), account.ID)
		}
		return LoginResult{}, secretStoreError("Could not save the account session", err)
	}
	committer, ok := service.store.(AccountCommitter)
	if !ok {
		if hadPreviousSecret {
			_ = service.secrets.Set(context.Background(), account.ID, previousSecret)
		} else {
			_ = service.secrets.Delete(context.Background(), account.ID)
		}
		if supportsCrashRecovery {
			_ = pendingStore.ClearPending(context.Background(), account.ID)
		}
		return LoginResult{}, errors.New("account metadata store does not support transactional commits")
	}
	if err := committer.SaveAccountAndSelect(ctx, account, account.IsDefault); err != nil {
		if hadPreviousSecret {
			_ = service.secrets.Set(context.Background(), account.ID, previousSecret)
		} else {
			_ = service.secrets.Delete(context.Background(), account.ID)
		}
		if supportsCrashRecovery {
			_ = pendingStore.ClearPending(context.Background(), account.ID)
		}
		return LoginResult{}, err
	}
	if supportsCrashRecovery {
		_ = pendingStore.ClearPending(context.Background(), account.ID)
	}

	safeAccount := account
	safeAccount.SessionKey = ""
	safeAccount.SessionSignature = ""
	return LoginResult{Status: LoginStatusSuccess, Account: &safeAccount}, nil
}

func (service *AccountService) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	accounts, err := service.store.ListAccounts(ctx)
	for index := range accounts {
		accounts[index].SessionKey = ""
		accounts[index].SessionSignature = ""
	}
	return accounts, err
}

func (service *AccountService) ValidateStaleAccounts(
	ctx context.Context,
	maxAge time.Duration,
) {
	accounts, err := service.store.ListAccounts(ctx)
	if err != nil {
		return
	}
	threshold := service.now().Add(-maxAge)
	for _, account := range accounts {
		if account.UID == "" ||
			(account.LastValidatedAt != nil && account.LastValidatedAt.After(threshold)) {
			continue
		}
		_, _ = service.ValidateAccount(ctx, account.ID)
	}
}

func (service *AccountService) SelectAccount(ctx context.Context, accountID string) error {
	if _, err := service.store.GetAccount(ctx, accountID); err != nil {
		return err
	}
	return service.store.SetDefaultAccount(ctx, accountID)
}

func (service *AccountService) RemoveAccount(ctx context.Context, accountID string) error {
	if _, err := service.store.GetAccount(ctx, accountID); err != nil {
		return err
	}
	if service.cleanupInstances != nil {
		if err := service.cleanupInstances(ctx, accountID); err != nil {
			return err
		}
	}
	secret, secretErr := service.secrets.Get(ctx, accountID)
	if secretErr != nil && !errors.Is(secretErr, ErrSecretNotFound) {
		return secretStoreError("Could not read the saved account session", secretErr)
	}
	if err := service.secrets.Delete(ctx, accountID); err != nil && !errors.Is(err, ErrSecretNotFound) {
		return &domain.AppError{Code: domain.ErrSecretStorage, Message: "Could not remove the saved account session", Cause: err}
	}
	if err := service.store.DeleteAccount(ctx, accountID); err != nil {
		if secretErr == nil {
			_ = service.secrets.Set(context.Background(), accountID, secret)
		}
		return err
	}
	return nil
}

// LogOutLocally removes the bearer credential while preserving non-secret
// profile metadata for an explicit future reauthentication.
func (service *AccountService) LogOutLocally(ctx context.Context, accountID string) error {
	account, err := service.store.GetAccount(ctx, accountID)
	if err != nil {
		return err
	}
	if service.cleanupInstances != nil {
		if err := service.cleanupInstances(ctx, accountID); err != nil {
			return err
		}
	}
	if err := service.secrets.Delete(ctx, accountID); err != nil && !errors.Is(err, ErrSecretNotFound) {
		return secretStoreError("Could not remove the saved account session", err)
	}
	account.Status = domain.AccountStatusNeedsReauth
	account.UpdatedAt = service.now()
	return service.store.SaveAccount(ctx, account)
}

func (service *AccountService) ValidateAccount(
	ctx context.Context,
	accountID string,
) (domain.Account, error) {
	account, err := service.ValidateAuthorizedAccount(ctx, accountID)
	return safeAccount(account), err
}

func (service *AccountService) ValidateAuthorizedAccount(
	ctx context.Context,
	accountID string,
) (domain.Account, error) {
	account, err := service.authorizedAccount(accountID, ctx)
	if err != nil {
		return account, err
	}
	valid, err := service.client.Validate(ctx, account.UID, account.SessionKey)
	if err != nil {
		return account, mapAuthError(err)
	}

	now := service.now()
	account.LastValidatedAt = &now
	account.UpdatedAt = now
	if !valid {
		account.Status = domain.AccountStatusExpired
		if saveErr := service.store.SaveAccount(ctx, safeAccount(account)); saveErr != nil {
			return account, saveErr
		}
		return account, domain.NewError(domain.ErrSessionExpired, "The account session has expired")
	}
	account.Status = domain.AccountStatusValid
	if err := service.store.SaveAccount(ctx, safeAccount(account)); err != nil {
		return account, err
	}
	return account, nil
}

func (service *AccountService) AuthorizedAccount(
	ctx context.Context,
	accountID string,
) (domain.Account, error) {
	return service.authorizedAccount(accountID, ctx)
}

func (service *AccountService) authorizedAccount(
	accountID string,
	ctx context.Context,
) (domain.Account, error) {
	account, err := service.store.GetAccount(ctx, accountID)
	if err != nil {
		return account, err
	}
	secret, err := service.secrets.Get(ctx, accountID)
	if errors.Is(err, ErrSecretNotFound) {
		account.Status = domain.AccountStatusNeedsReauth
		account.UpdatedAt = service.now()
		_ = service.store.SaveAccount(ctx, account)
		return safeAccount(account), domain.NewError(domain.ErrSessionExpired, "The account needs to be authenticated again")
	}
	if err != nil {
		return account, &domain.AppError{Code: domain.ErrSecretStorage, Message: "Could not read the account session", Cause: err}
	}
	account.SessionKey = secret.SessionKey
	account.SessionSignature = secret.SessionSignature
	return account, nil
}

func secretStoreError(message string, err error) error {
	switch {
	case errors.Is(err, ErrStoreLocked):
		message = "The operating-system credential store is locked. Unlock it and retry"
	case errors.Is(err, ErrPermissionDenied):
		message = "The operating-system credential store denied access"
	case errors.Is(err, ErrStoreUnavailable):
		message = "The operating-system credential store is unavailable. Check the desktop keyring service and retry"
	case errors.Is(err, ErrCorruptSecret):
		message = "The saved account session is corrupt and must be replaced"
	}
	retryable := errors.Is(err, ErrStoreLocked) || errors.Is(err, ErrStoreUnavailable)
	return &domain.AppError{Code: domain.ErrSecretStorage, Message: message, Cause: err, Retryable: retryable}
}

func safeAccount(account domain.Account) domain.Account {
	account.SessionKey = ""
	account.SessionSignature = ""
	return account
}

func loginFailure(err error) LoginResult {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		return LoginResult{Status: LoginStatusInvalidCredentials}
	case errors.Is(err, auth.ErrIPChanged):
		return LoginResult{Status: LoginStatusIPChanged}
	case errors.Is(err, auth.ErrTemporarilyBlocked):
		return LoginResult{Status: LoginStatusTemporarilyBlocked}
	case errors.Is(err, auth.ErrNetwork):
		return LoginResult{Status: LoginStatusNetworkError}
	case errors.Is(err, auth.ErrServer):
		return LoginResult{Status: LoginStatusServerError}
	case errors.Is(err, auth.ErrInvalidAuthReply):
		return LoginResult{Status: LoginStatusInvalidResponse}
	default:
		return LoginResult{Status: LoginStatusUnknownError}
	}
}

func mapAuthError(err error) error {
	switch {
	case errors.Is(err, auth.ErrNetwork):
		return &domain.AppError{Code: domain.ErrAuthNetwork, Message: "Could not connect to the Vintage Story authentication server", Retryable: true}
	case errors.Is(err, auth.ErrServer):
		return &domain.AppError{Code: domain.ErrAuthServer, Message: "The Vintage Story authentication server is unavailable", Retryable: true}
	case errors.Is(err, auth.ErrInvalidAuthReply):
		return domain.NewError(domain.ErrAuthInvalidResponse, "The authentication server returned an invalid response")
	default:
		return domain.NewError(domain.ErrAuthInvalidResponse, "Could not validate the account session")
	}
}
