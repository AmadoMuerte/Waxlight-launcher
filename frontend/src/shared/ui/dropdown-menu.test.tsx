// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it } from "vitest";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "./dropdown-menu";

afterEach(() => {
  cleanup();
  delete document.documentElement.dataset.inputModality;
});

function renderMenu() {
  render(
    <DropdownMenu>
      <DropdownMenuTrigger>Actions</DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuItem>Open</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>,
  );
}

it("closes immediately without restoring pointer focus to the trigger", async () => {
  document.documentElement.dataset.inputModality = "pointer";
  const user = userEvent.setup();
  renderMenu();

  const trigger = screen.getByRole("button", { name: "Actions" });
  await user.click(trigger);
  await user.click(screen.getByRole("menuitem", { name: "Open" }));

  expect(screen.queryByRole("menu")).toBeNull();
  expect(document.activeElement).not.toBe(trigger);
});

it("restores keyboard focus to the trigger", async () => {
  document.documentElement.dataset.inputModality = "keyboard";
  const user = userEvent.setup();
  renderMenu();

  const trigger = screen.getByRole("button", { name: "Actions" });
  trigger.focus();
  await user.keyboard("{Enter}{Escape}");

  expect(document.activeElement).toBe(trigger);
});
