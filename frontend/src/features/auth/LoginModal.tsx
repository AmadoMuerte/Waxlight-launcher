import type { TFunction } from "i18next";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { accountsApi } from "../../entities/account/api";
import type { Account, LoginResult, LoginStatus } from "../../entities/account/model";
import { errorMessage } from "../../shared/api/bridge";
import { Button, Field, Modal, SubmitForm } from "../../shared/ui";

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

interface LoginModalProps {
  account?: Account;
  onClose: () => void;
  onDone: () => Promise<void>;
}

export function LoginModal({ account, onClose, onDone }: LoginModalProps) {
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
