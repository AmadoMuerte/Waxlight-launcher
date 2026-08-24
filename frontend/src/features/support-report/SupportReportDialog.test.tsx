// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { useSupportReportStore } from "../../app/stores/support-report";
import { SupportReportDialog } from "./SupportReportDialog";

const api = vi.hoisted(() => ({ preview: vi.fn(), submit: vi.fn() }));
const clipboard = vi.hoisted(() => vi.fn());

vi.mock("../../shared/api/support-reports", () => ({ supportReportsApi: api }));
vi.mock("../../wailsjs/runtime/runtime", () => ({ ClipboardSetText: clipboard }));

afterEach(cleanup);
beforeEach(() => {
  vi.clearAllMocks();
  useSupportReportStore.setState({ open: true, instanceId: "" });
  api.preview.mockResolvedValue({ snapshotId: "snapshot-1", payload: '{"schemaVersion":1}' });
  api.submit.mockResolvedValue({ reportId: "WL-R-A7F31C", status: "received" });
  clipboard.mockResolvedValue(true);
});

it("validates description and previews sanitized payload", async () => {
  render(<SupportReportDialog />);
  fireEvent.click(screen.getByRole("button", { name: "View included data" }));
  expect(screen.getByText("Describe what happened before sending the report.")).toBeTruthy();
  fireEvent.change(screen.getByLabelText("What happened?"), { target: { value: "Game closes" } });
  fireEvent.click(screen.getByRole("button", { name: "View included data" }));
  expect(await screen.findByText('{"schemaVersion":1}')).toBeTruthy();
  expect(api.preview).toHaveBeenCalledWith("Game closes", "");
});

it("preserves the draft after failure and supports success and copying", async () => {
  api.submit.mockRejectedValueOnce(new Error("network unavailable"));
  render(<SupportReportDialog />);
  const field = screen.getByLabelText("What happened?");
  fireEvent.change(field, { target: { value: "Game closes" } });
  fireEvent.click(screen.getByRole("button", { name: "Send report" }));
  expect(await screen.findByText("network unavailable")).toBeTruthy();
  expect((field as HTMLTextAreaElement).value).toBe("Game closes");
  fireEvent.click(screen.getByRole("button", { name: "Send report" }));
  expect(await screen.findByText("WL-R-A7F31C")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Copy ID" }));
  await waitFor(() => expect(clipboard).toHaveBeenCalledWith("WL-R-A7F31C"));
  expect(api.submit).toHaveBeenLastCalledWith("snapshot-1");
});
