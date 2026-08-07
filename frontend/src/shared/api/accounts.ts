import type { presentation } from "../../wailsjs/go/models";
import {
  CancelLogin,
  CompleteTOTP,
  ListAccounts,
  Login,
  ReauthenticateAccount,
  RemoveAccount,
  SetDefaultAccount,
  ValidateAccount,
} from "../../wailsjs/go/presentation/AccountController";
import type { Account, LoginResult } from "./types";

export const accountsApi = {
  list: async () => (await ListAccounts()) as Account[],
  login: async (email: string, password: string) => loginResult(await Login(email, password)),
  completeTOTP: async (flowId: string, code: string) =>
    loginResult(await CompleteTOTP(flowId, code)),
  cancelLogin: (flowId: string) => CancelLogin(flowId),
  reauthenticate: async (accountId: string, email: string, password: string) =>
    loginResult(await ReauthenticateAccount(accountId, email, password)),
  validate: async (id: string) => await ValidateAccount(id),
  setDefault: (id: string) => SetDefaultAccount(id),
  remove: (id: string) => RemoveAccount(id),
};

function loginResult(result: presentation.LoginResultDTO): LoginResult {
  return {
    status: loginStatus(result.status),
    account: result.account,
    flowId: result.flowId,
    message: result.message,
  };
}

function loginStatus(status: string): LoginResult["status"] {
  switch (status) {
    case "success":
    case "totp_required":
    case "invalid_credentials":
    case "ip_changed":
    case "temporarily_blocked":
    case "network_error":
    case "server_error":
    case "invalid_response":
    case "unknown_error":
      return status;
    default:
      return "unknown_error";
  }
}
