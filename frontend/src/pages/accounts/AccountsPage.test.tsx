// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { Account, LoginResult } from "../../shared/api";
import { AccountsPage, authErrorMessages, isValidEmail } from "./AccountsPage";

const api = vi.hoisted(() => ({
  list: vi.fn(),
  login: vi.fn(),
  completeTOTP: vi.fn(),
  cancelLogin: vi.fn(),
  reauthenticate: vi.fn(),
  validate: vi.fn(),
  setDefault: vi.fn(),
  remove: vi.fn(),
}));

const settingsQuery = vi.hoisted(() => ({ useSettingsQuery: vi.fn() }));

vi.mock("../../entities/settings/queries", () => settingsQuery);

vi.mock("../../shared/api/accounts", () => ({ accountsApi: api }));

const validAccount: Account = {
  id: "first",
  username: "Waxlighter",
  displayName: "Waxlighter",
  email: "player@example.com",
  status: "valid",
  isDefault: true,
};

function renderPage(accounts: Account[] = []) {
  api.list.mockResolvedValue(accounts);
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <AccountsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { queryClient };
}

async function openAndFillCredentials() {
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: /add account/i }));
  const dialog = screen.getByRole("dialog");
  await user.type(within(dialog).getByLabelText("Email"), "player@example.com");
  await user.type(within(dialog).getByLabelText("Password"), "super-secret");
  return { user, dialog };
}

describe("account authentication UI", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    settingsQuery.useSettingsQuery.mockReturnValue({ data: undefined });
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
    await user.click(screen.getByRole("button", { name: /add account/i }));
    const dialog = screen.getByRole("dialog");
    await user.type(within(dialog).getByLabelText("Email"), "wrong-email");
    const password = within(dialog).getByLabelText("Password") as HTMLInputElement;
    await user.type(password, "super-secret");
    await user.click(within(dialog).getByRole("button", { name: /^sign in$/i }));

    expect(api.login).not.toHaveBeenCalled();
    expect(await screen.findByText("Enter a valid email address.")).toBeTruthy();
    expect(password.value).toBe("");
  });

  it("blocks duplicate form submission", async () => {
    let resolveLogin!: (result: LoginResult) => void;
    api.login.mockReturnValue(
      new Promise<LoginResult>((resolve) => {
        resolveLogin = resolve;
      }),
    );
    renderPage();
    const { dialog } = await openAndFillCredentials();
    const form = within(dialog)
      .getByRole("button", { name: /^sign in$/i })
      .closest("form");
    if (!form) throw new Error("login form not found");

    fireEvent.submit(form);
    fireEvent.submit(form);
    expect(api.login).toHaveBeenCalledTimes(1);

    resolveLogin({ status: "success", account: validAccount });
    await waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));
  });

  it("continues TOTP, keeps the real token out of React, and clears the code", async () => {
    api.login.mockResolvedValue({ status: "totp_required", flowId: "opaque-flow" });
    api.completeTOTP.mockResolvedValue({ status: "success", account: validAccount });
    renderPage();
    const { user, dialog } = await openAndFillCredentials();
    await user.click(within(dialog).getByRole("button", { name: /^sign in$/i }));

    const code = await screen.findByLabelText("Verification code");
    expect(screen.queryByLabelText("Password")).toBeNull();
    await user.type(code, "12ab3456");
    expect((code as HTMLInputElement).value).toBe("123456");
    await user.click(screen.getByRole("button", { name: /verify/i }));

    expect(api.completeTOTP).toHaveBeenCalledWith("opaque-flow", "123456");
    await waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));
    expect(window.localStorage.length).toBe(0);
    expect(JSON.stringify(api.completeTOTP.mock.calls)).not.toContain("prelogin");
  });

  it("cancels a TOTP flow and returns to credentials", async () => {
    api.login.mockResolvedValue({ status: "totp_required", flowId: "opaque-flow" });
    renderPage();
    const { user, dialog } = await openAndFillCredentials();
    await user.click(within(dialog).getByRole("button", { name: /^sign in$/i }));
    await screen.findByLabelText("Verification code");
    await user.click(screen.getByRole("button", { name: /back/i }));

    expect(api.cancelLogin).toHaveBeenCalledWith("opaque-flow");
    expect(await screen.findByLabelText("Password")).toBeTruthy();
  });

  it("shows typed backend errors and clears the password after failure", async () => {
    api.login.mockResolvedValue({ status: "invalid_credentials" });
    renderPage();
    const { user, dialog } = await openAndFillCredentials();
    const password = within(dialog).getByLabelText("Password") as HTMLInputElement;
    await user.click(within(dialog).getByRole("button", { name: /^sign in$/i }));

    expect(
      await screen.findByText("The email, password, or verification code is incorrect."),
    ).toBeTruthy();
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
    renderPage([validAccount, expired]);
    const user = userEvent.setup();

    const expiredName = await screen.findByText("Expired", { selector: "strong" });
    const expiredCard = expiredName.closest(".accountCard") as HTMLElement | null;
    if (!expiredCard) throw new Error("expired account card not found");
    await user.click(within(expiredCard).getByRole("button", { name: "Select" }));
    expect(api.setDefault).toHaveBeenCalledWith("expired");
    await waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));

    await user.click(within(expiredCard).getByRole("button", { name: "Sign in again" }));
    const dialog = screen.getByRole("dialog");
    const emailInput = within(dialog).getByLabelText("Email");
    if (!(emailInput instanceof HTMLInputElement)) throw new Error("email control is not an input");
    expect(emailInput.value).toBe("expired@example.com");
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));

    const validCard = screen.getByText("Waxlighter").closest(".accountCard") as HTMLElement | null;
    if (!validCard) throw new Error("valid account card not found");
    await user.click(within(validCard).getByRole("button", { name: "Validate" }));
    expect(api.validate).toHaveBeenCalledWith("first");
    await waitFor(() => expect(api.list).toHaveBeenCalledTimes(3));
    await user.click(within(validCard).getByRole("button", { name: "Remove from launcher" }));
    await user.click(await screen.findByRole("button", { name: "Delete" }));
    await waitFor(() => expect(api.remove).toHaveBeenCalledWith("first"));
    await waitFor(() => expect(api.list).toHaveBeenCalledTimes(4));
  });
});

describe("confirmDeletion gate", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    api.remove.mockResolvedValue(undefined);
    api.list.mockResolvedValue([validAccount]);
  });

  it("removes an account directly when confirmDeletion is false", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: { confirmDeletion: false } });
    const user = userEvent.setup();
    renderPage([validAccount]);
    await screen.findByText("Waxlighter");

    await user.click(screen.getByRole("button", { name: "Remove from launcher" }));

    await waitFor(() => expect(api.remove).toHaveBeenCalledWith("first"));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("shows a confirm dialog before removing when confirmDeletion is true", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: { confirmDeletion: true } });
    const user = userEvent.setup();
    renderPage([validAccount]);
    await screen.findByText("Waxlighter");

    await user.click(screen.getByRole("button", { name: "Remove from launcher" }));
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(api.remove).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(api.remove).toHaveBeenCalledWith("first"));
  });

  it("shows a confirm dialog when settings are still loading", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: undefined });
    const user = userEvent.setup();
    renderPage([validAccount]);
    await screen.findByText("Waxlighter");

    await user.click(screen.getByRole("button", { name: "Remove from launcher" }));
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(api.remove).not.toHaveBeenCalled();
  });
});

describe("account authentication helpers", () => {
  it("validates email and contains only safe localized errors", () => {
    expect(isValidEmail("player@example.com")).toBe(true);
    expect(isValidEmail("not-an-email")).toBe(false);
    expect(authErrorMessages.network_error).toBe("auth_network_error");
    expect(JSON.stringify(authErrorMessages)).not.toMatch(
      /sessionkey|sessionsignature|prelogintoken/i,
    );
  });
});
