// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";

import { Field } from "./field";

afterEach(() => cleanup());

it("renders the label, hint and control", () => {
  render(
    <Field label="Version ID" hint="Must match the archive folder.">
      <input />
    </Field>,
  );
  expect(screen.getByLabelText(/Version ID/)).toBeTruthy();
  expect(screen.getByText("Must match the archive folder.")).toBeTruthy();
});

it("shows the validation error instead of the hint", () => {
  render(
    <Field label="Version ID" hint="Must match the archive folder." error="Already installed.">
      <input aria-invalid="true" />
    </Field>,
  );
  expect(screen.getByText("Already installed.")).toBeTruthy();
  expect(screen.queryByText("Must match the archive folder.")).toBeNull();
});

it("renders a disabled control without an error", () => {
  render(
    <Field label="Path" hint="Unavailable right now.">
      <input disabled readOnly value="/data" />
    </Field>,
  );
  const control = screen.getByLabelText(/Path/) as HTMLInputElement;
  expect(control.disabled).toBe(true);
  expect(screen.getByText("Unavailable right now.")).toBeTruthy();
});
