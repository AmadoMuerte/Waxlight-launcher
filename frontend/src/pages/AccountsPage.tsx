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
  invalid_credentials: "Неверный email, пароль или код подтверждения.",
  ip_changed:
    "Vintage Story обнаружил изменение IP-адреса. Проверьте почту или повторите попытку позже.",
  temporarily_blocked:
    "Слишком много попыток входа. Авторизация временно заблокирована.",
  network_error:
    "Не удалось подключиться к серверу авторизации Vintage Story.",
  server_error:
    "Сервер авторизации Vintage Story временно недоступен.",
  invalid_response:
    "Сервер авторизации Vintage Story вернул неожиданный ответ.",
  unknown_error: "Не удалось выполнить вход. Повторите попытку позже.",
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
      notify("Аккаунт выбран");
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  async function removeAccount(account: Account) {
    if (
      !window.confirm(
        `Удалить аккаунт «${account.displayName}» из Waxlight Launcher? Серверная сессия на других устройствах не будет отозвана.`,
      )
    ) {
      return;
    }

    try {
      await accountsApi.remove(account.id);
      await refresh();
      notify("Аккаунт удалён из лаунчера");
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  async function validateAccount(account: Account) {
    try {
      await accountsApi.validate(account.id);
      await refresh();
      notify("Сессия аккаунта действительна");
    } catch (error) {
      await refresh();
      notify(errorMessage(error), "error");
    }
  }

  return (
    <>
      <PageHeader
        eyebrow="Vintage Story"
        title="Аккаунты"
        description="Выберите общий аккаунт или назначьте отдельный аккаунт каждой сборке."
        action={<Button onClick={() => setLoginAccount(null)}>＋ Добавить аккаунт</Button>}
      />

      <div className="notice pageNotice">
        <b>Пароль и код 2FA не сохраняются</b>
        <span>
          Waxlight хранит только сессионные ключи. Удаление аккаунта удаляет его
          из лаунчера, но не завершает сессию на других устройствах.
        </span>
      </div>

      {accounts.length === 0 ? (
        <Empty
          icon="♙"
          title="Нет сохранённых аккаунтов"
          description="Войдите в официальный аккаунт Vintage Story, чтобы запускать игру уже авторизованным."
          action={<Button onClick={() => setLoginAccount(null)}>Войти</Button>}
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
                  <span className="defaultMark">★ Выбран</span>
                ) : (
                  <Button
                    variant="secondary"
                    onClick={() => void selectAccount(account.id)}
                  >
                    Выбрать
                  </Button>
                )}

                {account.status === "expired" ||
                account.status === "needs_reauth" ? (
                  <Button
                    variant="secondary"
                    onClick={() => setLoginAccount(account)}
                  >
                    Войти снова
                  </Button>
                ) : (
                  <Button
                    variant="ghost"
                    onClick={() => void validateAccount(account)}
                  >
                    Проверить
                  </Button>
                )}

                <Button
                  variant="ghost"
                  onClick={() => void removeAccount(account)}
                >
                  Удалить из лаунчера
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
            notify("Вход выполнен");
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
      setError("Введите корректный email.");
      setPassword("");
      return;
    }
    if (!password) {
      setError("Введите пароль.");
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
      setError("Введите код подтверждения.");
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
      title={account ? "Повторный вход" : "Вход в Vintage Story"}
      onClose={() => void cancelFlow(true)}
    >
      {step === "credentials" ? (
        <SubmitForm className="form" noValidate onSubmit={submitCredentials}>
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

          <Field label="Пароль">
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
                aria-label={showPassword ? "Скрыть пароль" : "Показать пароль"}
                onClick={() => setShowPassword((value) => !value)}
              >
                {showPassword ? "Скрыть" : "Показать"}
              </button>
            </div>
          </Field>

          <div className="notice">
            <b>Неофициальная интеграция</b>
            <span>
              Waxlight обращается к серверу авторизации Vintage Story. Его API
              публично не документирован и может измениться.
            </span>
          </div>

          {error && <div className="inlineError">{error}</div>}

          <div className="modalActions">
            <Button type="button" variant="ghost" onClick={onClose}>
              Отмена
            </Button>
            <Button busy={busy}>Войти</Button>
          </div>
        </SubmitForm>
      ) : (
        <SubmitForm className="form" onSubmit={submitTOTP}>
          <div className="notice">
            <b>Двухфакторная авторизация</b>
            <span>
              Аккаунт защищён 2FA. Введите код из приложения-аутентификатора.
            </span>
          </div>

          <Field label="Код подтверждения">
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

          {error && <div className="inlineError">{error}</div>}

          <div className="modalActions">
            <Button
              type="button"
              variant="ghost"
              onClick={() => void cancelFlow(false)}
            >
              Назад
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => void cancelFlow(true)}
            >
              Отмена
            </Button>
            <Button busy={busy}>Подтвердить</Button>
          </div>
        </SubmitForm>
      )}
    </Modal>
  );
}
