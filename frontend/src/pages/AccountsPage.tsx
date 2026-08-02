import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";

import {
  accountsApi,
  type Account,
  type LoginResult,
  type LoginStatus,
} from "../shared/api";
import { errorMessage } from "../shared/api/bridge";
import {
  Button,
  Empty,
  Field,
  Modal,
  PageHeader,
  StatusPill,
  SubmitForm,
} from "../shared/ui";

type Notify = (message: string, type?: "ok" | "error") => void;

interface AccountsPageProps {
  accounts: Account[];
  refresh: () => Promise<void>;
  notify: Notify;
}

export const authErrorMessages: Record<Exclude<LoginStatus, "success" | "totp_required">, string> = {
  invalid_credentials: "The email, password, or verification code is incorrect.",
  ip_changed:
    "Vintage Story detected an IP address change. Check your email or try again later.",
  temporarily_blocked:
    "Too many sign-in attempts. Authentication is temporarily blocked.",
  network_error:
    "Could not connect to the Vintage Story authentication server.",
  server_error:
    "The Vintage Story authentication server is temporarily unavailable.",
  invalid_response:
    "The Vintage Story authentication server returned an unexpected response.",
  unknown_error: "Could not sign in. Please try again later.",
};

export function isValidEmail(value: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim());
}

function resultError(result: LoginResult): string {
  if (result.status === "success" || result.status === "totp_required") {
    return "";
  }
  return authErrorMessages[result.status];
}

export function AccountsPage({
  accounts,
  refresh,
  notify,
}: AccountsPageProps) {
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
      notify("Account selected");
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  async function removeAccount(account: Account) {
    if (
      !window.confirm(
        `Remove “${account.displayName}” from Waxlight Launcher? This will not revoke the server session on other devices.`,
      )
    ) {
      return;
    }

    try {
      await accountsApi.remove(account.id);
      await refresh();
      notify("Account removed from the launcher");
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  async function validateAccount(account: Account) {
    try {
      await accountsApi.validate(account.id);
      await refresh();
      notify("The account session is valid");
    } catch (error) {
      await refresh();
      notify(errorMessage(error), "error");
    }
  }

  return (
    <>
      <PageHeader
        eyebrow="Vintage Story"
        title="Accounts"
        description="Select a global account or assign a different account to each instance."
        action={<Button onClick={() => setLoginAccount(null)}>＋ Add account</Button>}
      />

      <div className="notice pageNotice">
        <b>Passwords and 2FA codes are never stored</b>
        <span>
          Waxlight stores only session credentials. Removing an account deletes
          it from the launcher but does not sign out other devices.
        </span>
      </div>

      {accounts.length === 0 ? (
        <Empty
          icon="♙"
          title="No saved accounts"
          description="Sign in with your official Vintage Story account to launch the game already authenticated."
          action={<Button onClick={() => setLoginAccount(null)}>Sign in</Button>}
        />
      ) : (
        <div className="accountGrid">
          {accounts.map((account) => (
            <article className="accountCard" key={account.id}>
              <div className="avatar">
                {account.displayName.slice(0, 1).toUpperCase()}
              </div>

              <div className="accountIdentity">
                <strong>{account.displayName}</strong>
                <small>{account.email}</small>
                <StatusPill status={account.status} />
              </div>

              <div className="accountActions">
                {account.isDefault ? (
                  <span className="defaultMark">★ Selected</span>
                ) : (
                  <Button
                    variant="secondary"
                    onClick={() => void selectAccount(account.id)}
                  >
                    Select
                  </Button>
                )}

                {account.status === "expired" ||
                account.status === "needs_reauth" ? (
                  <Button
                    variant="secondary"
                    onClick={() => setLoginAccount(account)}
                  >
                    Sign in again
                  </Button>
                ) : (
                  <Button
                    variant="ghost"
                    onClick={() => void validateAccount(account)}
                  >
                    Validate
                  </Button>
                )}

                <Button
                  variant="ghost"
                  onClick={() => void removeAccount(account)}
                >
                  Remove from launcher
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
            notify("Signed in successfully");
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
  const [step, setStep] = useState<"credentials" | "totp">("credentials");
  const [email, setEmail] = useState(account?.email ?? "");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [totp, setTOTP] = useState("");
  const [flowID, setFlowID] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const submitting = useRef(false);

  async function submitCredentials() {
    if (submitting.current) return;
    if (!isValidEmail(email)) {
      setError("Enter a valid email address.");
      setPassword("");
      return;
    }
    if (!password) {
      setError("Enter your password.");
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
        setError(resultError(result));
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
      setError("Enter the verification code.");
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
        setError(resultError(result));
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
      title={account ? "Sign in again" : "Sign in to Vintage Story"}
      onClose={() => void cancelFlow(true)}
    >
      {step === "credentials" ? (
        <SubmitForm className="dialogForm" noValidate onSubmit={submitCredentials}>
          <div className="modalBody formFields">
            <Field label="Email">
              <input
                autoFocus
                required
                type="email"
                autoComplete="username"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="you@example.com"
              />
            </Field>

            <Field label="Password">
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
                  aria-label={showPassword ? "Hide password" : "Show password"}
                  onClick={() => setShowPassword((value) => !value)}
                >
                  {showPassword ? "Hide" : "Show"}
                </button>
              </div>
            </Field>

            <div className="notice">
              <b>Unofficial integration</b>
              <span>
                Waxlight connects to the Vintage Story authentication server. Its
                API is not publicly documented and may change.
              </span>
            </div>

            {error && <div className="inlineError" role="alert">{error}</div>}
          </div>

          <div className="dialogFooter">
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button busy={busy}>Sign in</Button>
          </div>
        </SubmitForm>
      ) : (
        <SubmitForm className="dialogForm" onSubmit={submitTOTP}>
          <div className="modalBody formFields">
            <div className="notice">
              <b>Two-factor authentication</b>
              <span>
                This account uses 2FA. Enter the code from your authenticator app.
              </span>
            </div>

            <Field label="Verification code">
              <input
                autoFocus
                required
                inputMode="numeric"
                autoComplete="one-time-code"
                maxLength={16}
                value={totp}
                onChange={(event) =>
                  setTOTP(event.target.value.replace(/[^0-9]/g, ""))
                }
              />
            </Field>

            {error && <div className="inlineError" role="alert">{error}</div>}
          </div>

          <div className="dialogFooter">
            <Button
              type="button"
              variant="ghost"
              onClick={() => void cancelFlow(false)}
            >
              Back
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => void cancelFlow(true)}
            >
              Cancel
            </Button>
            <Button busy={busy}>Verify</Button>
          </div>
        </SubmitForm>
      )}
    </Modal>
  );
}
