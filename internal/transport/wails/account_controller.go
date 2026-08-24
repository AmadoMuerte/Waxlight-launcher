package wails

import (
	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
)

// AccountController exposes account login, reauthentication, and removal to
// the frontend. It stays limited to DTO conversion and feature invocation.
type AccountController struct {
	svc       *accounts.Service
	lifecycle lifecycle
}

func NewAccountController(service *accounts.Service, lifecycle lifecycle) *AccountController {
	return &AccountController{svc: service, lifecycle: lifecycle}
}

func (controller *AccountController) ListAccounts() ([]AccountDTO, error) {
	accounts, err := controller.svc.ListAccounts(controller.lifecycle.Context())
	result := make([]AccountDTO, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, accountDTO(account))
	}
	return result, err
}

func (controller *AccountController) Login(email, password string) (LoginResultDTO, error) {
	result, err := controller.svc.Login(controller.lifecycle.Context(), email, password)
	return loginResultDTO(result), err
}

func (controller *AccountController) CompleteTOTP(flowID, code string) (LoginResultDTO, error) {
	result, err := controller.svc.CompleteTOTP(controller.lifecycle.Context(), flowID, code)
	return loginResultDTO(result), err
}

func (controller *AccountController) CancelLogin(flowID string) error {
	return controller.svc.CancelLogin(flowID)
}

func (controller *AccountController) SetDefaultAccount(id string) error {
	return controller.svc.SelectAccount(controller.lifecycle.Context(), id)
}

func (controller *AccountController) RemoveAccount(id string) error {
	return controller.svc.RemoveAccount(controller.lifecycle.Context(), id)
}

func (controller *AccountController) ValidateAccount(id string) (AccountDTO, error) {
	account, err := controller.svc.ValidateAccount(controller.lifecycle.Context(), id)
	return accountDTO(account), err
}

func (controller *AccountController) ReauthenticateAccount(
	accountID string,
	email string,
	password string,
) (LoginResultDTO, error) {
	result, err := controller.svc.ReauthenticateAccount(
		controller.lifecycle.Context(),
		accountID,
		email,
		password,
	)
	return loginResultDTO(result), err
}
