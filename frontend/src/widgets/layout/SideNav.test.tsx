// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, expect, it, vi } from "vitest";

import i18n from "../../shared/i18n";
import { SideNav } from "./SideNav";

vi.mock("../../entities/operation/queries", () => ({ useOperationsQuery: () => ({ data: [] }) }));
vi.mock("../../entities/news/queries", () => ({
  useNewsQuery: () => ({ data: { unreadCount: 3 } }),
}));

afterEach(() => cleanup());

it("shows a restrained unread indicator for News", () => {
  void i18n.changeLanguage("en");
  render(
    <MemoryRouter>
      <SideNav />
    </MemoryRouter>,
  );
  expect(screen.getByText("Unread news: 3")).toBeTruthy();
  expect(screen.getByRole("link", { name: /News/ }).querySelector(".navPulse")).toBeTruthy();
});
