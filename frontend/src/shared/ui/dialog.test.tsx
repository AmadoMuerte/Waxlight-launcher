// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, expect, it, vi } from "vitest";

import i18n from "../i18n";
import { Dialog, DialogContent, DialogDescription, DialogTitle, DialogTrigger } from "./dialog";

beforeAll(() => i18n.changeLanguage("en"));

it("names the dialog and returns focus when it closes", async () => {
  render(
    <Dialog>
      <DialogTrigger>Open details</DialogTrigger>
      <DialogContent>
        <DialogTitle>Instance details</DialogTitle>
        <DialogDescription>Review the selected instance.</DialogDescription>
      </DialogContent>
    </Dialog>,
  );

  const user = userEvent.setup();
  const trigger = screen.getByRole("button", { name: "Open details" });
  await user.click(trigger);

  expect(screen.getByRole("dialog", { name: "Instance details" })).toBeTruthy();
  expect(screen.getByText("Review the selected instance.")).toBeTruthy();

  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(trigger);
});

it("does not begin closing when the dialog is locked", async () => {
  const onOpenChange = vi.fn();
  render(
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent closable={false}>
        <DialogTitle>Installing update</DialogTitle>
      </DialogContent>
    </Dialog>,
  );

  expect(screen.queryByRole("button", { name: "Close" })).toBeNull();
  await userEvent.setup().keyboard("{Escape}");

  expect(screen.getByRole("dialog", { name: "Installing update" })).toBeTruthy();
  expect(onOpenChange).not.toHaveBeenCalled();
});
