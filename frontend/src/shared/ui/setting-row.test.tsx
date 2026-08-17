// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";

import { SettingRow } from "./setting-row";

afterEach(() => cleanup());

it("renders the title, description and control", () => {
  render(
    <SettingRow title="Confirm deletion" description="Ask before removing items.">
      <button type="button">Toggle</button>
    </SettingRow>,
  );
  expect(screen.getByText("Confirm deletion")).toBeTruthy();
  expect(screen.getByText("Ask before removing items.")).toBeTruthy();
  expect(screen.getByRole("button", { name: "Toggle" })).toBeTruthy();
});

it("renders warning text without forcing a description", () => {
  render(
    <SettingRow title="Data folder" warning="The previous move failed.">
      <button type="button">Change</button>
    </SettingRow>,
  );
  expect(screen.getByText("The previous move failed.")).toBeTruthy();
  expect(screen.getByRole("button", { name: "Change" })).toBeTruthy();
});

it("marks disabled rows with the disabled state class", () => {
  render(
    <SettingRow title="Disabled" disabled>
      <button type="button">Control</button>
    </SettingRow>,
  );
  const row = screen.getByText("Disabled").closest(".settingRow") as HTMLElement;
  expect(row.className).toContain("settingRowDisabled");
});

it("applies the column layout for full-width controls", () => {
  render(
    <SettingRow column title="Arguments">
      <input aria-label="Arguments" />
    </SettingRow>,
  );
  const row = screen.getByLabelText("Arguments").closest(".settingRow") as HTMLElement;
  expect(row.className).toContain("settingRowColumn");
});

it("renders only the control when no text is provided", () => {
  render(
    <SettingRow>
      <button type="button">Detect</button>
    </SettingRow>,
  );
  expect(screen.getByRole("button", { name: "Detect" })).toBeTruthy();
});
