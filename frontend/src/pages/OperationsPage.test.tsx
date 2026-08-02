// @vitest-environment jsdom

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { Operation } from "../shared/api";
import { OperationsPage } from "./OperationsPage";

const api = vi.hoisted(() => ({
  cancel: vi.fn(),
  remove: vi.fn(),
  clearHistory: vi.fn(),
}));

vi.mock("../shared/api", () => ({ operationsApi: api }));

const createdAt = "2026-08-03T09:00:00Z";

function operation(
  id: string,
  title: string,
  status: Operation["status"],
): Operation {
  return {
    id,
    type: "game_version_download",
    title,
    status,
    progress: status === "completed" ? 1 : 0.5,
    currentBytes: 50,
    totalBytes: 100,
    bytesPerSecond: 0,
    createdAt,
  };
}

function renderPage(operations: Operation[]) {
  const refresh = vi.fn().mockResolvedValue(undefined);
  const notify = vi.fn();
  render(
    <OperationsPage
      operations={operations}
      refresh={refresh}
      notify={notify}
    />,
  );
  return { refresh, notify };
}

describe("operations history controls", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  beforeEach(() => {
    vi.clearAllMocks();
    api.cancel.mockResolvedValue(undefined);
    api.remove.mockResolvedValue(undefined);
    api.clearHistory.mockResolvedValue(3);
    vi.spyOn(window, "confirm").mockReturnValue(true);
  });

  it("offers cancel only for active operations and delete only for finished ones", () => {
    renderPage([
      operation("running", "Active download", "running"),
      operation("completed", "Completed download", "completed"),
      operation("failed", "Failed download", "failed"),
      operation("cancelled", "Old cancellation", "cancelled"),
    ]);

    expect(screen.getAllByRole("button", { name: "Cancel" })).toHaveLength(1);
    expect(screen.getByRole("button", { name: "Delete Completed download" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Delete Failed download" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Delete Old cancellation" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Delete Active download" })).toBeNull();
  });

  it("deletes one finished operation and refreshes the list", async () => {
    const user = userEvent.setup();
    const { refresh, notify } = renderPage([
      operation("completed", "Completed download", "completed"),
    ]);

    await user.click(
      screen.getByRole("button", { name: "Delete Completed download" }),
    );

    await waitFor(() => expect(api.remove).toHaveBeenCalledWith("completed"));
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(notify).toHaveBeenCalledWith("Operation removed from history");
  });

  it("clears finished history while retaining the backend active-operation guard", async () => {
    const user = userEvent.setup();
    const { refresh, notify } = renderPage([
      operation("running", "Active download", "running"),
      operation("failed", "Failed download", "failed"),
    ]);

    await user.click(screen.getByRole("button", { name: "Clear history" }));

    await waitFor(() => expect(api.clearHistory).toHaveBeenCalledTimes(1));
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(notify).toHaveBeenCalledWith("3 operations removed from history");
  });
});
