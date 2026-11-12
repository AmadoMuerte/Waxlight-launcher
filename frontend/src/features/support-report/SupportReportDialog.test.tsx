import { beforeEach, expect, it, vi } from "bun:test";

import { fireEvent, render, screen } from "@testing-library/react";

import { useSupportReportStore } from "../../app/stores/support-report";
import { SupportReportDialog } from "./SupportReportDialog";

const runtime = { BrowserOpenURL: vi.fn() };

void vi.mock("../../wailsjs/runtime/runtime", () => runtime);

beforeEach(() => {
  runtime.BrowserOpenURL.mockClear();
  useSupportReportStore.setState({ open: true });
});

it("points bug reports to the Discord invite", () => {
  render(<SupportReportDialog />);

  fireEvent.click(screen.getByRole("button", { name: /discord/i }));

  expect(runtime.BrowserOpenURL).toHaveBeenCalledWith("https://discord.gg/CrRHvg9UVw");
});
