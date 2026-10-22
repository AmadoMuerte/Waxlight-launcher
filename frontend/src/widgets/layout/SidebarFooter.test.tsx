import { expect, it, vi } from "bun:test";

import { fireEvent, render, screen } from "@testing-library/react";

import { SidebarFooter } from "./SidebarFooter";

const runtime = { BrowserOpenURL: vi.fn() };

void vi.mock("../../wailsjs/runtime/runtime", () => runtime);

it("opens the launcher Discord invite in the system browser", () => {
  render(<SidebarFooter />);

  fireEvent.click(screen.getByRole("link", { name: /discord/i }));

  expect(runtime.BrowserOpenURL).toHaveBeenCalledWith("https://discord.gg/CrRHvg9UVw");
});
