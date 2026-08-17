// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { beforeAll, expect, it } from "vitest";

import i18n from "../../shared/i18n";
import { UiLabPage } from "./UiLabPage";

beforeAll(() => i18n.changeLanguage("en"));

it("shows the design system and exercises the shared dialog", async () => {
  render(
    <MemoryRouter>
      <UiLabPage />
    </MemoryRouter>,
  );

  expect(screen.getByRole("heading", { level: 1, name: "Waxlight UI Lab" })).toBeTruthy();
  for (const section of [
    "Colors",
    "Typography",
    "Structural patterns",
    "Buttons",
    "Icon buttons",
    "Inputs",
    "Cards",
    "Domain patterns",
    "Page states",
  ]) {
    expect(screen.getByRole("heading", { name: section })).toBeTruthy();
  }

  const user = userEvent.setup();
  await user.click(screen.getByRole("tab", { name: "Details" }));
  expect(screen.getByText("Selected tab: details")).toBeTruthy();
  await user.click(screen.getByRole("button", { name: "List layout" }));
  expect(screen.getByRole("button", { name: "List layout" }).getAttribute("aria-pressed")).toBe(
    "true",
  );
  await user.click(screen.getByRole("button", { name: "Open shared dialog" }));
  expect(screen.getByRole("dialog", { name: "Shared dialog" })).toBeTruthy();
  await user.click(screen.getByRole("button", { name: "Close dialog" }));
  expect(screen.queryByRole("dialog")).toBeNull();
});
