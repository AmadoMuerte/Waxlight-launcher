import type { TFunction } from "i18next";
import { Eye, EyeOff } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { accountsApi } from "../../entities/account/api";
import type { Account, LoginResult, LoginStatus } from "../../entities/account/model";
import { errorMessage } from "../../shared/api/bridge";
import { Button } from "../../shared/ui/button";
import { DialogFooter } from "../../shared/ui/dialog";
import { Field } from "../../shared/ui/field";
import { IconButton } from "../../shared/ui/icon-button";
import { Input } from "../../shared/ui/input";
import { Modal } from "../../shared/ui/modal";
import { SubmitForm } from "../../shared/ui/submit-form";

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
  const cancelledFlowID = useRef("");

  useEffect(() => {
    return () => {
      if (flowID && cancelledFlowID.current !== flowID) {
        void accountsApi.cancelLogin(flowID).catch(() => undefined);
      }
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

  function cancelFlow(close: boolean) {
    const cancelledFlow = flowID;
    cancelledFlowID.current = cancelledFlow;
    setFlowID("");
    setTOTP("");
    setPassword("");
    setError("");
    if (close) {
      onClose();
    } else {
      setStep("credentials");
    }
    if (cancelledFlow) {
      void accountsApi.cancelLogin(cancelledFlow).catch(() => undefined);
    }
  }

  return (
    <Modal
      title={account ? t("sign_in_again") : t("sign_in_to_vintage_story")}
      onClose={() => cancelFlow(true)}
      closable={!busy}
    >
      {step === "credentials" ? (
        <SubmitForm className="dialogForm" noValidate onSubmit={submitCredentials}>
          <div className="modalBody formFields">
            <Field label={t("email")}>
              <Input
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
                <Input
                  required
                  type={showPassword ? "text" : "password"}
                  autoComplete="current-password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                />
                <IconButton
                  variant="ghost"
                  size="sm"
                  className="absolute top-1/2 right-2 -translate-y-1/2"
                  aria-label={showPassword ? t("hide_password") : t("show_password")}
                  onClick={() => setShowPassword((shown) => !shown)}
                >
                  {showPassword ? (
                    <EyeOff size={16} aria-hidden="true" />
                  ) : (
                    <Eye size={16} aria-hidden="true" />
                  )}
                </IconButton>
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

          <DialogFooter>
            <Button type="button" variant="ghost" disabled={busy} onClick={() => cancelFlow(true)}>
              {t("cancel")}
            </Button>
            <Button type="submit" busy={busy}>
              {t("sign_in")}
            </Button>
          </DialogFooter>
        </SubmitForm>
      ) : (
        <SubmitForm className="dialogForm" onSubmit={submitTOTP}>
          <div className="modalBody formFields">
            <div className="notice">
              <b>{t("two_factor_authentication")}</b>
              <span>{t("two_factor_prompt")}</span>
            </div>

            <Field label={t("verification_code")}>
              <Input
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

          <DialogFooter>
            <Button type="button" variant="ghost" disabled={busy} onClick={() => cancelFlow(false)}>
              {t("back")}
            </Button>
            <Button type="button" variant="ghost" disabled={busy} onClick={() => cancelFlow(true)}>
              {t("cancel")}
            </Button>
            <Button type="submit" busy={busy}>
              {t("verify")}
            </Button>
          </DialogFooter>
        </SubmitForm>
      )}
    </Modal>
  );
}
