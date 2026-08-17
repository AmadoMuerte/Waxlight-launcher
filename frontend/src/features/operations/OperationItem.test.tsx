// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { Operation } from "../../entities/operation/model";
import { OperationItem } from "./OperationItem";

const createdAt = "2026-08-03T09:00:00Z";

function operation(overrides: Partial<Operation> & { id: string }): Operation {
  return {
    type: "game_version_download",
    title: "Downloading Vintage Story",
    status: "running",
    progress: 0.5,
    currentBytes: 50,
    totalBytes: 100,
    bytesPerSecond: 0,
    createdAt,
    ...overrides,
  };
}

function renderItem(
  operationData: Operation,
  props: Partial<React.ComponentProps<typeof OperationItem>> = {},
) {
  return render(<OperationItem operation={operationData} {...props} />);
}

describe("operation state presentation", () => {
  afterEach(cleanup);

  it("shows queued operations with a cancel action and no progress", () => {
    renderItem(operation({ id: "q", status: "queued", progress: 0 }));

    expect(screen.getByText("Queued")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeTruthy();
    expect(screen.queryByRole("progressbar")).toBeNull();
  });

  it("shows running operations with an accessible progress bar and percent", () => {
    renderItem(operation({ id: "r", status: "running", progress: 0.63 }));

    expect(screen.getByText("Running")).toBeTruthy();
    const bar = screen.getByRole("progressbar");
    expect(bar.getAttribute("aria-valuenow")).toBe("63");
    expect(screen.getByText("63%")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeTruthy();
  });

  it("shows completed operations with a delete action and no progress", () => {
    renderItem(operation({ id: "c", status: "completed", progress: 1 }));

    expect(screen.getByText("Completed")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Delete Downloading Vintage Story" })).toBeTruthy();
    expect(screen.queryByRole("progressbar")).toBeNull();
    expect(screen.queryByRole("button", { name: "Cancel" })).toBeNull();
  });

  it("shows failed operations with the error summary", () => {
    renderItem(
      operation({
        id: "f",
        status: "failed",
        errorMessage: "The download failed because the remote server refused the connection",
      }),
    );

    expect(screen.getByText("Failed")).toBeTruthy();
    expect(screen.getByText(/remote server refused/)).toBeTruthy();
    expect(screen.getByRole("button", { name: "Delete Downloading Vintage Story" })).toBeTruthy();
  });

  it("treats cancelled as a distinct neutral state", () => {
    renderItem(operation({ id: "x", status: "cancelled" }));

    expect(screen.getByText("Cancelled")).toBeTruthy();
    expect(screen.queryByText("Failed")).toBeNull();
    expect(screen.getByRole("button", { name: "Delete Downloading Vintage Story" })).toBeTruthy();
  });

  it("renders a long title and long error text without breaking", () => {
    const longTitle = "A very long operation title that should truncate cleanly on the row";
    renderItem(
      operation({
        id: "long",
        title: longTitle,
        status: "failed",
        errorMessage:
          "A very long error summary that keeps going and going and going and going and going and going and going so the row clamps it instead of destroying the layout",
      }),
    );

    expect(screen.getByText(longTitle)).toBeTruthy();
    expect(screen.getByText(/keeps going/)).toBeTruthy();
  });
});

describe("operation actions", () => {
  afterEach(cleanup);

  it("fires cancel for active operations", async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    renderItem(operation({ id: "r", status: "running" }), { onCancel });

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("fires remove for finished operations", async () => {
    const user = userEvent.setup();
    const onRemove = vi.fn();
    renderItem(operation({ id: "c", status: "completed" }), { onRemove });

    await user.click(screen.getByRole("button", { name: "Delete Downloading Vintage Story" }));
    expect(onRemove).toHaveBeenCalledTimes(1);
  });

  it("disables actions while another action is in flight", () => {
    renderItem(operation({ id: "r", status: "running" }), { actionsDisabled: true });

    expect(screen.getByRole("button", { name: "Cancel" }).hasAttribute("disabled")).toBe(true);
  });

  it("omits cancel when the backend cannot cancel the operation", () => {
    renderItem(operation({ id: "c", status: "completed" }));
    expect(screen.queryByRole("button", { name: "Cancel" })).toBeNull();
  });
});
