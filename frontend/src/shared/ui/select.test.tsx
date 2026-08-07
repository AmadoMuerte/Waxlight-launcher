// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";

import { Field } from "@/shared/ui/field";
import { Modal } from "@/shared/ui/modal";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./select";

it("closes an open select before closing its modal", async () => {
  const onClose = vi.fn();
  render(
    <Modal title="Example" onClose={onClose}>
      <Field label="Value">
        <Select value="one">
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="one">One</SelectItem>
            <SelectItem value="two">Two</SelectItem>
          </SelectContent>
        </Select>
      </Field>
    </Modal>,
  );

  const user = userEvent.setup();
  await user.click(screen.getByRole("combobox", { name: "Value" }));
  expect(screen.getByRole("option", { name: "One" })).toBeTruthy();
  await user.keyboard("{Escape}");

  expect(screen.queryByRole("option", { name: "One" })).toBeNull();
  expect(onClose).not.toHaveBeenCalled();

  await user.keyboard("{Escape}");
  expect(onClose).toHaveBeenCalledOnce();
});
