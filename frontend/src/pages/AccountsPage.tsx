import type { TFunction } from "i18next";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";

import { accountsApi, type Account, type LoginResult, type LoginStatus } from "../shared/api";
import { errorMessage } from "../shared/api/bridge";
import { Button, Empty, Field, Modal, PageHeader, StatusPill, SubmitForm } from "../shared/ui";

type Notify = (message: string, type?: "ok" | "error") => void;

interface AccountsPageProps {
  accounts: Account[];
  refresh: () => Promise<void>;
  notify: Notify;
}

export const authErrorMessages: Record<
  Exclude<LoginStatus, "success" | "totp_required">,
  string
> = {
  invalid_credentials: "auth_invalid_credentials",
  ip_changed: "auth_ip_blocked",
  temporarily_blocked: "auth_rate_limited",
  network_error: "auth_network_error",
  server_error: "auth_service_unavailable",
  invalid_response: "auth_unexpected_response",
  unknown_error: "auth_unknown_error",
};

export function isValidEmail(value: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim());
}

function resultError(result: LoginResult, t: TFunction): string {
  if (result.status === "success" || result.status === "totp_required") {
    return "";
  }
  return t(authErrorMessages[result.status]);
}

export function AccountsPage({ accounts, refresh, notify }: AccountsPageProps) {
  const { t } = useTranslation();
  const [loginAccount, setLoginAccount] = useState<Account | null>();
  const [searchParams, setSearchParams] = useSearchParams();

  useEffect(() => {
    if (searchParams.get("add") === "1") {
      setLoginAccount(null);
      setSearchParams({}, { replace: true });
    }
  }, [searchParams, setSearchParams]);

  async function selectAccount(accountID: string) {
    try {
      await accountsApi.setDefault(accountID);
      await refresh();
      notify(t("account_selected"));
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  async function removeAccount(account: Account) {
    if (!window.confirm(t("remove_account_confirmation", { name: account.displayName }))) {
      return;
    }

    try {
      await accountsApi.remove(account.id);
      await refresh();
      notify(t("account_removed"));
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  async function validateAccount(account: Account) {
    try {
      await accountsApi.validate(account.id);
      await refresh();
      notify(t("account_session_valid"));
    } catch (error) {
      await refresh();
      notify(errorMessage(error), "error");
    }
  }

  return (
    <>
      <PageHeader
        eyebrow="Vintage Story"
        title={t("accounts")}
        description={t("accounts_description")}
        action={<Button onClick={() => setLoginAccount(null)}>{t("add_account")}</Button>}
      />

      <div className="notice pageNotice">
        <b>{t("credentials_never_stored")}</b>
        <span>{t("session_credentials_notice")}</span>
      </div>

      {accounts.length === 0 ? (
        <Empty
          icon="♙"
          title={t("no_saved_accounts")}
          description={t("official_account_description")}
          action={<Button onClick={() => setLoginAccount(null)}>{t("sign_in")}</Button>}
        />
      ) : (
        <div className="accountGrid">
          {accounts.map((account) => (
            <article className="accountCard" key={account.id}>
              <div className="avatar">{account.displayName.slice(0, 1).toUpperCase()}</div>

              <div className="accountIdentity">
                <strong>{account.displayName}</strong>
                <small>{account.email}</small>
                <StatusPill status={account.status} />
              </div>

              <div className="accountActions">
                {account.isDefault ? (
                  <span className="defaultMark">{t("selected_status")}</span>
                ) : (
                  <Button variant="secondary" onClick={() => void selectAccount(account.id)}>
                    {t("select")}
                  </Button>
                )}

                {account.status === "expired" || account.status === "needs_reauth" ? (
                  <Button variant="secondary" onClick={() => setLoginAccount(account)}>
                    {t("sign_in_again")}
                  </Button>
                ) : (
                  <Button variant="ghost" onClick={() => void validateAccount(account)}>
                    {t("validate")}
                  </Button>
                )}

                <Button variant="ghost" onClick={() => void removeAccount(account)}>
                  {t("remove_from_launcher")}
                </Button>
              </div>
            </article>
          ))}
        </div>
      )}

      {loginAccount !== undefined && (
        <LoginModal
          account={loginAccount ?? undefined}
          onClose={() => setLoginAccount(undefined)}
          onDone={async () => {
            setLoginAccount(undefined);
            await refresh();
            notify(t("signed_in_successfully"));
          }}
        />
      )}
    </>
  );
}

interface LoginModalProps {
  account?: Account;
  onClose: () => void;
  onDone: () => Promise<void>;
}

function LoginModal({ account, onClose, onDone }: LoginModalProps) {
  const { t } = useTranslation();
  const [step, setStep] = useState<"credentials" | "totp">("credentials");
  const [email, setEmail] = useState(account?.email ?? "");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [totp, setTOTP] = useState("");
  const [flowID, setFlowID] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const submitting = useRef(false);

  useEffect(() => {
    return () => {
      if (flowID) void accountsApi.cancelLogin(flowID).catch(() => undefined);
    };
  }, [flowID]);

  async function submitCredentials() {
    if (submitting.current) return;
    if (!isValidEmail(email)) {
      setError(t("enter_valid_email"));
      setPassword("");
      return;
    }
    if (!password) {
      setError(t("enter_password"));
      return;
    }

    submitting.current = true;
    setBusy(true);
    setError("");
    try {
      const result = account
        ? await accountsApi.reauthenticate(account.id, email.trim(), password)
        : await accountsApi.login(email.trim(), password);
      setPassword("");
      if (result.status === "success") {
        await onDone();
      } else if (result.status === "totp_required" && result.flowId) {
        setFlowID(result.flowId);
        setStep("totp");
      } else {
        setError(resultError(result, t));
      }
    } catch (submitError) {
      setPassword("");
      setError(errorMessage(submitError));
    } finally {
      submitting.current = false;
      setBusy(false);
    }
  }

  async function submitTOTP() {
    if (submitting.current) return;
    if (!totp || totp.length > 16) {
      setError(t("enter_verification_code"));
      return;
    }
    submitting.current = true;
    setBusy(true);
    setError("");
    try {
      const result = await accountsApi.completeTOTP(flowID, totp);
      setTOTP("");
      if (result.status === "success") {
        await onDone();
      } else {
        setError(resultError(result, t));
      }
    } catch (submitError) {
      setTOTP("");
      setError(errorMessage(submitError));
    } finally {
      submitting.current = false;
      setBusy(false);
    }
  }

  async function cancelFlow(close: boolean) {
    if (flowID) {
      try {
        await accountsApi.cancelLogin(flowID);
      } catch {
        // The flow is in-memory and expires automatically.
      }
    }
    setFlowID("");
    setTOTP("");
    setPassword("");
    setError("");
    if (close) {
      onClose();
    } else {
      setStep("credentials");
    }
  }

  return (
    <Modal
      title={account ? t("sign_in_again") : t("sign_in_to_vintage_story")}
      onClose={() => void cancelFlow(true)}
    >
      {step === "credentials" ? (
        <SubmitForm className="dialogForm" noValidate onSubmit={submitCredentials}>
          <div className="modalBody formFields">
            <Field label={t("email")}>
              <input
                required
                type="email"
                autoComplete="username"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="you@example.com"
              />
            </Field>

            <Field label={t("password")}>
              <div className="passwordField">
                <input
                  required
                  type={showPassword ? "text" : "password"}
                  autoComplete="current-password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                />
                <button
                  type="button"
                  aria-label={showPassword ? t("hide_password") : t("show_password")}
                  onClick={() => setShowPassword((value) => !value)}
                >
                  {showPassword ? t("hide") : t("show")}
                </button>
              </div>
            </Field>

            <div className="notice">
              <b>{t("unofficial_integration")}</b>
              <span>{t("unofficial_integration_notice")}</span>
            </div>

            {error && (
              <div className="inlineError" role="alert">
                {error}
              </div>
            )}
          </div>

          <div className="dialogFooter">
            <Button type="button" variant="ghost" onClick={() => void cancelFlow(true)}>
              {t("cancel")}
            </Button>
            <Button busy={busy}>{t("sign_in")}</Button>
          </div>
        </SubmitForm>
      ) : (
        <SubmitForm className="dialogForm" onSubmit={submitTOTP}>
          <div className="modalBody formFields">
            <div className="notice">
              <b>{t("two_factor_authentication")}</b>
              <span>{t("two_factor_prompt")}</span>
            </div>

            <Field label={t("verification_code")}>
              <input
                required
                inputMode="numeric"
                autoComplete="one-time-code"
                maxLength={16}
                value={totp}
                onChange={(event) => setTOTP(event.target.value.replace(/[^0-9]/g, ""))}
              />
            </Field>

            {error && (
              <div className="inlineError" role="alert">
                {error}
              </div>
            )}
          </div>

          <div className="dialogFooter">
            <Button type="button" variant="ghost" onClick={() => void cancelFlow(false)}>
              {t("back")}
            </Button>
            <Button type="button" variant="ghost" onClick={() => void cancelFlow(true)}>
              {t("cancel")}
            </Button>
            <Button busy={busy}>{t("verify")}</Button>
          </div>
        </SubmitForm>
      )}
    </Modal>
  );
}
