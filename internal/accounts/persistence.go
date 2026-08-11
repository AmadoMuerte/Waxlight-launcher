package accounts

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/waxlight/waxlight-launcher/internal/errs"
)

func (service *Service) persistSession(ctx context.Context, expectedAccountID, email string, session Session) (LoginResult, error) {
	release, gateErr := service.beginMutation()
	if gateErr != nil {
		return LoginResult{}, gateErr
	}
	defer release()
	service.persistMu.Lock()
	defer service.persistMu.Unlock()
	storedAccounts, err := service.repository.ListAccounts(ctx)
	if err != nil {
		return LoginResult{}, err
	}
	var account Account
	found := false
	for _, candidate := range storedAccounts {
		if candidate.UID == session.UID && candidate.UID != "" {
			account, found = candidate, true
			break
		}
	}
	if expectedAccountID != "" {
		expected, err := service.repository.GetAccount(ctx, expectedAccountID)
		if err != nil {
			return LoginResult{}, err
		}
		if expected.UID != "" && expected.UID != session.UID {
			return LoginResult{Status: LoginStatusInvalidCredentials}, nil
		}
		account, found = expected, true
	}
	now := service.now()
	if !found {
		account.ID = uuid.NewString()
		account.CreatedAt = now
		account.IsDefault = len(storedAccounts) == 0
	}
	account.Username = session.PlayerName
	account.DisplayName = session.PlayerName
	account.Email = email
	account.UID = session.UID
	account.Status = StatusValid
	account.UpdatedAt = now
	account.LastValidatedAt = &now
	slog.Info("account authenticated", "account", account.ID, "fresh", !found)

	credential := Credential{SessionKey: session.SessionKey, SessionSignature: session.SessionSignature}
	previous, previousErr := service.credentials.Get(ctx, account.ID)
	hadPrevious := previousErr == nil
	if previousErr != nil && !errors.Is(previousErr, ErrCredentialsNotFound) {
		return LoginResult{}, credentialStoreError("Could not read the existing account session", previousErr)
	}
	if service.pending != nil {
		if err := service.pending.MarkPending(ctx, account.ID); err != nil {
			return LoginResult{}, credentialStoreError("Could not prepare the account session commit", err)
		}
	}
	if err := service.credentials.Set(ctx, account.ID, credential); err != nil {
		service.clearPending(account.ID)
		return LoginResult{}, credentialStoreError("Could not save the account session", err)
	}
	if err := service.repository.SaveAccountAndSelect(ctx, account, account.IsDefault); err != nil {
		service.rollbackCredential(account.ID, previous, hadPrevious)
		service.clearPending(account.ID)
		return LoginResult{}, err
	}
	service.clearPending(account.ID)
	safe := safeAccount(account)
	return LoginResult{Status: LoginStatusSuccess, Account: &safe}, nil
}

func (service *Service) rollbackCredential(accountID string, previous Credential, hadPrevious bool) {
	var err error
	if hadPrevious {
		err = service.credentials.Set(context.Background(), accountID, previous)
	} else {
		err = service.credentials.Delete(context.Background(), accountID)
	}
	if err != nil {
		slog.Error("could not roll back the account session", "account", accountID, "error", err)
	}
}

func (service *Service) clearPending(accountID string) {
	if service.pending != nil {
		if err := service.pending.ClearPending(context.Background(), accountID); err != nil {
			slog.Warn("could not clear the pending account session marker", "account", accountID, "error", err)
		}
	}
}

func (service *Service) ListAccounts(ctx context.Context) ([]Account, error) {
	storedAccounts, err := service.repository.ListAccounts(ctx)
	for index := range storedAccounts {
		storedAccounts[index] = safeAccount(storedAccounts[index])
	}
	return storedAccounts, err
}

func (service *Service) GetAccount(ctx context.Context, accountID string) (Account, error) {
	account, err := service.repository.GetAccount(ctx, accountID)
	return safeAccount(account), err
}

func (service *Service) ValidateStaleAccounts(ctx context.Context, maxAge time.Duration) {
	storedAccounts, err := service.repository.ListAccounts(ctx)
	if err != nil {
		slog.Warn("could not list accounts for stale validation", "error", err)
		return
	}
	threshold := service.now().Add(-maxAge)
	stale := 0
	for _, account := range storedAccounts {
		if account.UID == "" || (account.LastValidatedAt != nil && account.LastValidatedAt.After(threshold)) {
			continue
		}
		stale++
		if _, err := service.ValidateAccount(ctx, account.ID); err != nil {
			slog.Warn("stale account validation failed", "account", account.ID, "error", err)
		}
	}
	if stale > 0 {
		slog.Info("validated stale accounts", "count", stale)
	}
}

func (service *Service) SelectAccount(ctx context.Context, accountID string) error {
	release, err := service.beginMutation()
	if err != nil {
		return err
	}
	defer release()
	if _, err := service.repository.GetAccount(ctx, accountID); err != nil {
		return err
	}
	slog.Info("default account selected", "account", accountID)
	return service.repository.SetDefaultAccount(ctx, accountID)
}

func (service *Service) RemoveAccount(ctx context.Context, accountID string) error {
	release, err := service.beginMutation()
	if err != nil {
		return err
	}
	defer release()
	if _, err := service.repository.GetAccount(ctx, accountID); err != nil {
		return err
	}
	if service.cleanupInstances != nil {
		if err := service.cleanupInstances(ctx, accountID); err != nil {
			return err
		}
	}
	credential, credentialErr := service.credentials.Get(ctx, accountID)
	if credentialErr != nil && !errors.Is(credentialErr, ErrCredentialsNotFound) {
		return credentialStoreError("Could not read the saved account session", credentialErr)
	}
	if err := service.credentials.Delete(ctx, accountID); err != nil && !errors.Is(err, ErrCredentialsNotFound) {
		return &errs.AppError{Code: errs.ErrSecretStorage, Message: "Could not remove the saved account session", Cause: err}
	}
	if err := service.repository.DeleteAccount(ctx, accountID); err != nil {
		if credentialErr == nil {
			service.rollbackCredential(accountID, credential, true)
		}
		return err
	}
	slog.Info("account removed", "account", accountID)
	return nil
}

func (service *Service) LogOutLocally(ctx context.Context, accountID string) error {
	release, err := service.beginMutation()
	if err != nil {
		return err
	}
	defer release()
	account, err := service.repository.GetAccount(ctx, accountID)
	if err != nil {
		return err
	}
	if service.cleanupInstances != nil {
		if err := service.cleanupInstances(ctx, accountID); err != nil {
			return err
		}
	}
	if err := service.credentials.Delete(ctx, accountID); err != nil && !errors.Is(err, ErrCredentialsNotFound) {
		return credentialStoreError("Could not remove the saved account session", err)
	}
	account.Status = StatusNeedsReauth
	account.UpdatedAt = service.now()
	slog.Info("account logged out locally", "account", accountID)
	return service.repository.SaveAccount(ctx, account)
}

func (service *Service) ValidateAccount(ctx context.Context, accountID string) (Account, error) {
	account, err := service.ValidateAuthorizedAccount(ctx, accountID)
	return safeAccount(account), err
}

func (service *Service) ValidateAuthorizedAccount(ctx context.Context, accountID string) (Account, error) {
	account, err := service.authorizedAccount(ctx, accountID)
	if err != nil {
		return account, err
	}
	valid, err := service.authenticator.Validate(ctx, account.UID, account.SessionKey)
	if err != nil {
		service.reportFailure(err)
		return account, mapAuthError(err)
	}
	now := service.now()
	account.LastValidatedAt = &now
	account.UpdatedAt = now
	if !valid {
		account.Status = StatusExpired
		if err := service.saveAccountMutation(ctx, safeAccount(account)); err != nil {
			return account, err
		}
		slog.Warn("account session expired", "account", accountID)
		return account, errs.NewError(errs.ErrSessionExpired, "The account session has expired")
	}
	account.Status = StatusValid
	if err := service.saveAccountMutation(ctx, safeAccount(account)); err != nil {
		return account, err
	}
	return account, nil
}

func (service *Service) AuthorizedAccount(ctx context.Context, accountID string) (Account, error) {
	return service.authorizedAccount(ctx, accountID)
}

func (service *Service) authorizedAccount(ctx context.Context, accountID string) (Account, error) {
	account, err := service.repository.GetAccount(ctx, accountID)
	if err != nil {
		return account, err
	}
	credential, err := service.credentials.Get(ctx, accountID)
	if errors.Is(err, ErrCredentialsNotFound) {
		account.Status = StatusNeedsReauth
		account.UpdatedAt = service.now()
		if saveErr := service.saveAccountMutation(ctx, account); saveErr != nil {
			slog.Warn("could not persist the reauthentication flag", "account", accountID, "error", saveErr)
		}
		return safeAccount(account), errs.NewError(errs.ErrSessionExpired, "The account needs to be authenticated again")
	}
	if err != nil {
		return account, &errs.AppError{Code: errs.ErrSecretStorage, Message: "Could not read the account session", Cause: err}
	}
	account.SessionKey = credential.SessionKey
	account.SessionSignature = credential.SessionSignature
	return account, nil
}

func (service *Service) saveAccountMutation(ctx context.Context, account Account) error {
	release, err := service.beginMutation()
	if err != nil {
		return err
	}
	defer release()
	return service.repository.SaveAccount(ctx, account)
}

func credentialStoreError(message string, err error) error {
	switch {
	case errors.Is(err, ErrStoreLocked):
		message = "The operating-system credential store is locked. Unlock it and retry"
	case errors.Is(err, ErrPermissionDenied):
		message = "The operating-system credential store denied access"
	case errors.Is(err, ErrStoreUnavailable):
		message = "The operating-system credential store is unavailable. Check the desktop keyring service and retry"
	case errors.Is(err, ErrCorruptCredentials):
		message = "The saved account session is corrupt and must be replaced"
	}
	retryable := errors.Is(err, ErrStoreLocked) || errors.Is(err, ErrStoreUnavailable)
	return &errs.AppError{Code: errs.ErrSecretStorage, Message: message, Cause: err, Retryable: retryable}
}

func safeAccount(account Account) Account {
	account.SessionKey = ""
	account.SessionSignature = ""
	return account
}

func mapAuthError(err error) error {
	switch {
	case errors.Is(err, ErrAuthNetwork):
		return &errs.AppError{Code: errs.ErrAuthNetwork, Message: "Could not connect to the Vintage Story authentication server", Retryable: true}
	case errors.Is(err, ErrAuthServer):
		return &errs.AppError{Code: errs.ErrAuthServer, Message: "The Vintage Story authentication server is unavailable", Retryable: true}
	case errors.Is(err, ErrInvalidAuthReply):
		return errs.NewError(errs.ErrAuthInvalidResponse, "The authentication server returned an invalid response")
	default:
		return errs.NewError(errs.ErrAuthInvalidResponse, "Could not validate the account session")
	}
}
