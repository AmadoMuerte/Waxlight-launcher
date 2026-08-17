// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import { Library } from "lucide-react";
import { MemoryRouter } from "react-router";
import { expect, it } from "vitest";

import { NavItem } from "./NavItem";

it("renders an accessible active navigation destination", () => {
  render(
    <MemoryRouter initialEntries={["/library"]}>
      <NavItem to="/library" icon={Library} label="Library" indicator />
    </MemoryRouter>,
  );

  const link = screen.getByRole("link", { name: "Library" });
  expect(link.getAttribute("aria-current")).toBe("page");
});
