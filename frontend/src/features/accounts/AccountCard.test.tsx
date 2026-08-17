// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";

import type { Account } from "../../entities/account/model";
import { AccountCard } from "./AccountCard";

const account: Account = {
  id: "first",
  username: "Waxlighter",
  displayName: "Waxlighter",
  email: "player@example.com",
  status: "valid",
  isDefault: true,
};

const handlers = {
  onSelect: vi.fn(),
  onSignInAgain: vi.fn(),
  onValidate: vi.fn(),
  onRemove: vi.fn(),
};

afterEach(() => cleanup());

it("marks the active account and hides the select action", () => {
  render(<AccountCard account={account} {...handlers} />);
  expect(screen.getByText("★ Selected")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Select" })).toBeNull();
  expect(screen.getByRole("button", { name: "Validate" })).toBeTruthy();
  expect(screen.getByRole("button", { name: "Remove from launcher" })).toBeTruthy();
});

it("offers select for inactive accounts and signs in again for expired ones", async () => {
  const user = userEvent.setup();
  const { rerender } = render(
    <AccountCard account={{ ...account, isDefault: false }} {...handlers} />,
  );
  await user.click(screen.getByRole("button", { name: "Select" }));
  expect(handlers.onSelect).toHaveBeenCalledTimes(1);
  expect(screen.getByRole("button", { name: "Validate" })).toBeTruthy();

  rerender(
    <AccountCard
      account={{ ...account, isDefault: false, status: "needs_reauth" }}
      {...handlers}
    />,
  );
  await user.click(screen.getByRole("button", { name: "Sign in again" }));
  expect(handlers.onSignInAgain).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole("button", { name: "Validate" })).toBeNull();
});

it("truncates long display names without hiding the actions", () => {
  render(
    <AccountCard
      account={{
        ...account,
        id: "long",
        displayName: "A very long translated display name that must never break card actions",
        isDefault: false,
      }}
      {...handlers}
    />,
  );
  const name = screen.getByText(
    "A very long translated display name that must never break card actions",
  );
  expect(name).toBeTruthy();
  expect(screen.getByRole("button", { name: "Select" })).toBeTruthy();
  expect(screen.getByRole("button", { name: "Remove from launcher" })).toBeTruthy();
});

it("disables actions while busy", () => {
  render(<AccountCard account={{ ...account, isDefault: false }} busy {...handlers} />);
  const select = screen.getByRole("button", { name: "Select" }) as HTMLButtonElement;
  const remove = screen.getByRole("button", { name: "Remove from launcher" }) as HTMLButtonElement;
  expect(select.disabled).toBe(true);
  expect(remove.disabled).toBe(true);
});

it("shows the account validity status", () => {
  render(
    <AccountCard account={{ ...account, isDefault: false, status: "expired" }} {...handlers} />,
  );
  expect(screen.getByText("Expired")).toBeTruthy();
});
