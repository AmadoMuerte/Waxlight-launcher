// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";

import { Button } from "./button";
import { IconButton } from "./icon-button";
import { Input } from "./input";

it("keeps a busy button named and prevents interaction", async () => {
  const onClick = vi.fn();
  render(
    <Button busy onClick={onClick}>
      Install
    </Button>,
  );

  const button = screen.getByRole("button", { name: "Install" });
  expect(button.getAttribute("aria-busy")).toBe("true");
  expect((button as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(button);
  expect(onClick).not.toHaveBeenCalled();
});

it("requires an accessible icon button name and respects disabled", async () => {
  const onClick = vi.fn();
  render(
    <IconButton aria-label="Delete item" disabled onClick={onClick}>
      ×
    </IconButton>,
  );

  const button = screen.getByRole("button", { name: "Delete item" });
  await userEvent.click(button);
  expect(onClick).not.toHaveBeenCalled();
});

it("forwards native input accessibility state", async () => {
  render(
    <label htmlFor="instance-name">
      Instance name
      <Input id="instance-name" aria-invalid="true" />
    </label>,
  );

  const input = screen.getByRole("textbox", { name: "Instance name" });
  await userEvent.type(input, "Cave base");
  expect((input as HTMLInputElement).value).toBe("Cave base");
  expect(input.getAttribute("aria-invalid")).toBe("true");
});
