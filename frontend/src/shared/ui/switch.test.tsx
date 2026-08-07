// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";

import { Switch } from "./switch";

afterEach(() => cleanup());

it("renders with the checked state and proper role", () => {
  render(<Switch checked label="Confirm deletion" onCheckedChange={vi.fn()} />);
  const toggle = screen.getByRole("switch", { name: "Confirm deletion" }) as HTMLButtonElement;
  expect(toggle.getAttribute("aria-checked")).toBe("true");
  expect(toggle.dataset.state).toBe("checked");
});

it("toggles through clicks", async () => {
  const onCheckedChange = vi.fn();
  render(<Switch checked={false} label="Confirm deletion" onCheckedChange={onCheckedChange} />);
  const user = userEvent.setup();
  await user.click(screen.getByRole("switch", { name: "Confirm deletion" }));
  expect(onCheckedChange).toHaveBeenCalledWith(true);
});

it("toggles through the keyboard", async () => {
  const onCheckedChange = vi.fn();
  render(<Switch checked label="Confirm deletion" onCheckedChange={onCheckedChange} />);
  const toggle = screen.getByRole("switch", { name: "Confirm deletion" });
  const user = userEvent.setup();
  toggle.focus();
  await user.keyboard("{Enter}");
  await user.keyboard(" ");
  expect(onCheckedChange).toHaveBeenCalledTimes(2);
});

it("does not toggle when disabled", async () => {
  const onCheckedChange = vi.fn();
  render(<Switch checked label="Confirm deletion" onCheckedChange={onCheckedChange} disabled />);
  const toggle = screen.getByRole("switch", { name: "Confirm deletion" }) as HTMLButtonElement;
  expect(toggle.disabled).toBe(true);
  const user = userEvent.setup();
  await user.click(toggle);
  expect(onCheckedChange).not.toHaveBeenCalled();
});
