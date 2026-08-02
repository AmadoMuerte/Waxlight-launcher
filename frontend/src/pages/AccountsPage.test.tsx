// @vitest-environment jsdom

import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { Account, LoginResult } from "../shared/api";
import {
  AccountsPage,
  authErrorMessages,
  isValidEmail,
} from "./AccountsPage";

const api = vi.hoisted(() => ({
  login: vi.fn(),
  completeTOTP: vi.fn(),
  cancelLogin: vi.fn(),
  reauthenticate: vi.fn(),
  validate: vi.fn(),
  setDefault: vi.fn(),
  remove: vi.fn(),
}));

vi.mock("../shared/api", () => ({ accountsApi: api }));

const validAccount: Account = {
  id: "first",
  username: "Waxlighter",
  displayName: "Waxlighter",
  email: "player@example.com",
  status: "valid",
  isDefault: true,
};

function renderPage(accounts: Account[] = []) {
  const refresh = vi.fn().mockResolvedValue(undefined);
  const notify = vi.fn();
  render(
    <MemoryRouter>
      <AccountsPage accounts={accounts} refresh={refresh} notify={notify} />
    </MemoryRouter>,
  );
  return { refresh, notify };
}

async function openAndFillCredentials() {
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: /добавить аккаунт/i }));
  const dialog = screen.getByRole("dialog");
  await user.type(within(dialog).getByLabelText("Email"), "player@example.com");
  await user.type(within(dialog).getByLabelText("Пароль"), "super-secret");
  return { user, dialog };
}

describe("account authentication UI", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    Object.defineProperty(window, "localStorage", {
      configurable: true,
      value: {
        length: 0,
        clear: vi.fn(),
        getItem: vi.fn().mockReturnValue(null),
        key: vi.fn().mockReturnValue(null),
        removeItem: vi.fn(),
        setItem: vi.fn(),
      },
    });
    api.cancelLogin.mockResolvedValue(undefined);
    api.setDefault.mockResolvedValue(undefined);
    api.remove.mockResolvedValue(undefined);
    api.validate.mockResolvedValue(validAccount);
  });

  it("opens login, validates email, and clears the password", async () => {
    renderPage();
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /добавить аккаунт/i }));
    const dialog = screen.getByRole("dialog");
    await user.type(within(dialog).getByLabelText("Email"), "wrong-email");
    const password = within(dialog).getByLabelText("Пароль") as HTMLInputElement;
    await user.type(password, "super-secret");
    await user.click(within(dialog).getByRole("button", { name: /^войти$/i }));

    expect(api.login).not.toHaveBeenCalled();
    expect(await screen.findByText("Введите корректный email.")).toBeTruthy();
    expect(password.value).toBe("");
  });

  it("blocks duplicate form submission", async () => {
    let resolveLogin!: (result: LoginResult) => void;
    api.login.mockReturnValue(
      new Promise<LoginResult>((resolve) => {
        resolveLogin = resolve;
      }),
    );
    const { refresh } = renderPage();
    const { dialog } = await openAndFillCredentials();
    const form = within(dialog)
      .getByRole("button", { name: /^войти$/i })
      .closest("form");
    if (!form) throw new Error("login form not found");

    fireEvent.submit(form);
    fireEvent.submit(form);
    expect(api.login).toHaveBeenCalledTimes(1);

    resolveLogin({ status: "success", account: validAccount });
    await waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
  });

  it("continues TOTP, keeps the real token out of React, and clears the code", async () => {
    api.login.mockResolvedValue({ status: "totp_required", flowId: "opaque-flow" });
    api.completeTOTP.mockResolvedValue({ status: "success", account: validAccount });
    const { refresh } = renderPage();
    const { user, dialog } = await openAndFillCredentials();
    await user.click(within(dialog).getByRole("button", { name: /^войти$/i }));

    const code = await screen.findByLabelText("Код подтверждения");
    expect(screen.queryByLabelText("Пароль")).toBeNull();
    await user.type(code, "12ab3456");
    expect((code as HTMLInputElement).value).toBe("123456");
    await user.click(screen.getByRole("button", { name: /подтвердить/i }));

    expect(api.completeTOTP).toHaveBeenCalledWith("opaque-flow", "123456");
    await waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
    expect(window.localStorage.length).toBe(0);
    expect(JSON.stringify(api.completeTOTP.mock.calls)).not.toContain("prelogin");
  });

  it("cancels a TOTP flow and returns to credentials", async () => {
    api.login.mockResolvedValue({ status: "totp_required", flowId: "opaque-flow" });
    renderPage();
    const { user, dialog } = await openAndFillCredentials();
    await user.click(within(dialog).getByRole("button", { name: /^войти$/i }));
    await screen.findByLabelText("Код подтверждения");
    await user.click(screen.getByRole("button", { name: /назад/i }));

    expect(api.cancelLogin).toHaveBeenCalledWith("opaque-flow");
    expect(await screen.findByLabelText("Пароль")).toBeTruthy();
  });

  it("shows typed backend errors and clears the password after failure", async () => {
    api.login.mockResolvedValue({ status: "invalid_credentials" });
    renderPage();
    const { user, dialog } = await openAndFillCredentials();
    const password = within(dialog).getByLabelText("Пароль") as HTMLInputElement;
    await user.click(within(dialog).getByRole("button", { name: /^войти$/i }));

    expect(await screen.findByText(authErrorMessages.invalid_credentials)).toBeTruthy();
    expect(password.value).toBe("");
  });

  it("switches, validates, reauthenticates, and removes accounts", async () => {
    const expired: Account = {
      ...validAccount,
      id: "expired",
      displayName: "Expired",
      email: "expired@example.com",
      status: "expired",
      isDefault: false,
    };
    vi.spyOn(window, "confirm").mockReturnValue(true);
    const { refresh } = renderPage([validAccount, expired]);
    const user = userEvent.setup();

    const expiredCard = screen.getByText("Expired").closest("article");
    if (!expiredCard) throw new Error("expired account card not found");
    await user.click(within(expiredCard).getByRole("button", { name: "Выбрать" }));
    expect(api.setDefault).toHaveBeenCalledWith("expired");

    await user.click(within(expiredCard).getByRole("button", { name: "Войти снова" }));
    const dialog = screen.getByRole("dialog");
    expect((within(dialog).getByLabelText("Email") as HTMLInputElement).value).toBe(
      "expired@example.com",
    );
    await user.click(within(dialog).getByRole("button", { name: "Отмена" }));

    const validCard = screen.getByText("Waxlighter").closest("article");
    if (!validCard) throw new Error("valid account card not found");
    await user.click(within(validCard).getByRole("button", { name: "Проверить" }));
    expect(api.validate).toHaveBeenCalledWith("first");
    await user.click(
      within(validCard).getByRole("button", { name: "Удалить из лаунчера" }),
    );
    expect(api.remove).toHaveBeenCalledWith("first");
    expect(refresh).toHaveBeenCalled();
  });
});

describe("account authentication helpers", () => {
  it("validates email and contains only safe localized errors", () => {
    expect(isValidEmail("player@example.com")).toBe(true);
    expect(isValidEmail("not-an-email")).toBe(false);
    expect(authErrorMessages.network_error).toContain("подключиться");
    expect(JSON.stringify(authErrorMessages)).not.toMatch(
      /sessionkey|sessionsignature|prelogintoken/i,
    );
  });
});
