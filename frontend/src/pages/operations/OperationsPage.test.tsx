// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useToastStore } from "../../app/stores/toast";
import type { Operation } from "../../entities/operation/model";
import { OperationsPage } from "./OperationsPage";

const api = vi.hoisted(() => ({
  list: vi.fn(),
  cancel: vi.fn(),
  remove: vi.fn(),
  clearHistory: vi.fn(),
  logsList: vi.fn(),
  logsExport: vi.fn(),
  logsOpenDirectory: vi.fn(),
}));

const settingsQuery = vi.hoisted(() => ({ useSettingsQuery: vi.fn() }));

vi.mock("../../entities/settings/queries", () => settingsQuery);

vi.mock("../../shared/api/operations", () => ({
  operationsApi: {
    list: api.list,
    cancel: api.cancel,
    remove: api.remove,
    clearHistory: api.clearHistory,
  },
}));
vi.mock("../../shared/api/logs", () => ({
  logsApi: {
    list: api.logsList,
    exportLogs: api.logsExport,
    openDirectory: api.logsOpenDirectory,
  },
}));

vi.mock("@xterm/xterm", () => ({
  Terminal: class {
    write = vi.fn();
    clear = vi.fn();
    dispose = vi.fn();
    loadAddon = vi.fn();
    open = vi.fn();
  },
}));

vi.mock("@xterm/addon-fit", () => ({
  FitAddon: class {
    fit = vi.fn();
  },
}));

const createdAt = "2026-08-03T09:00:00Z";

function operation(id: string, title: string, status: Operation["status"]): Operation {
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
  api.list.mockResolvedValue(operations);
  const notify = vi.fn();
  useToastStore.setState({ notify });
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <OperationsPage />
    </QueryClientProvider>,
  );
  return { notify };
}

async function renderPageLoaded(operations: Operation[]) {
  const result = renderPage(operations);
  await screen.findByText(operations[0].title);
  return result;
}

describe("operations history controls", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  beforeEach(() => {
    vi.clearAllMocks();
    settingsQuery.useSettingsQuery.mockReturnValue({ data: undefined });
    vi.stubGlobal(
      "ResizeObserver",
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      },
    );
    vi.stubGlobal("runtime", {
      EventsOn: () => () => undefined,
      EventsOnMultiple: () => () => undefined,
      EventsEmit: () => undefined,
    });
    api.cancel.mockResolvedValue(undefined);
    api.remove.mockResolvedValue(undefined);
    api.clearHistory.mockResolvedValue(3);
    api.logsList.mockResolvedValue([]);
    api.logsExport.mockResolvedValue("");
    api.logsOpenDirectory.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("offers cancel only for active operations and delete only for finished ones", async () => {
    await renderPageLoaded([
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
    const { notify } = await renderPageLoaded([
      operation("completed", "Completed download", "completed"),
    ]);

    await user.click(screen.getByRole("button", { name: "Delete Completed download" }));

    await user.click(await screen.findByRole("button", { name: "Delete" }));

    await waitFor(() => expect(api.remove).toHaveBeenCalledWith("completed"));
    await waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));
    expect(notify).toHaveBeenCalledWith("Operation removed from history");
  });

  it("clears finished history while retaining the backend active-operation guard", async () => {
    const user = userEvent.setup();
    const { notify } = await renderPageLoaded([
      operation("running", "Active download", "running"),
      operation("failed", "Failed download", "failed"),
    ]);

    await user.click(screen.getByRole("button", { name: "Clear history" }));

    await user.click(await screen.findByRole("button", { name: "Delete" }));

    await waitFor(() => expect(api.clearHistory).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));
    expect(notify).toHaveBeenCalledWith("3 operations removed from history");
  });

  it("renders the launcher console section alongside the history", async () => {
    await renderPageLoaded([operation("completed", "Completed download", "completed")]);
    expect(screen.getByRole("heading", { name: "Launcher console" })).toBeTruthy();
    expect(await screen.findByRole("button", { name: "Export logs" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Copy logs" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Clear console" })).toBeTruthy();
  });

  it("collapses and expands the console", async () => {
    const user = userEvent.setup();
    void renderPageLoaded([operation("completed", "Completed download", "completed")]);

    expect(screen.getByRole("button", { expanded: true })).toBeTruthy();

    await user.click(screen.getByRole("button", { expanded: true }));
    expect(screen.getByRole("button", { expanded: false })).toBeTruthy();

    await user.click(screen.getByRole("button", { expanded: false }));
    expect(screen.getByRole("button", { expanded: true })).toBeTruthy();
  });
});

describe("confirmDeletion gate", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  beforeEach(() => {
    vi.clearAllMocks();
    api.remove.mockResolvedValue(undefined);
    api.clearHistory.mockResolvedValue(3);
    api.logsList.mockResolvedValue([]);
    api.logsExport.mockResolvedValue("");
    api.logsOpenDirectory.mockResolvedValue(undefined);
    vi.stubGlobal(
      "ResizeObserver",
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      },
    );
    vi.stubGlobal("runtime", {
      EventsOn: () => () => undefined,
      EventsOnMultiple: () => () => undefined,
      EventsEmit: () => undefined,
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("removes a finished operation directly when confirmDeletion is false", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: { confirmDeletion: false } });
    const user = userEvent.setup();
    const { notify } = await renderPageLoaded([
      operation("completed", "Completed download", "completed"),
    ]);

    await user.click(screen.getByRole("button", { name: "Delete Completed download" }));

    await waitFor(() => expect(api.remove).toHaveBeenCalledWith("completed"));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(notify).toHaveBeenCalledWith("Operation removed from history");
  });

  it("shows a confirm dialog before removing when confirmDeletion is true", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: { confirmDeletion: true } });
    const user = userEvent.setup();
    const { notify } = await renderPageLoaded([
      operation("completed", "Completed download", "completed"),
    ]);

    await user.click(screen.getByRole("button", { name: "Delete Completed download" }));
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(api.remove).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(api.remove).toHaveBeenCalledWith("completed"));
    expect(notify).toHaveBeenCalledWith("Operation removed from history");
  });

  it("shows a confirm dialog when settings are still loading", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: undefined });
    const user = userEvent.setup();
    await renderPageLoaded([operation("completed", "Completed download", "completed")]);

    await user.click(screen.getByRole("button", { name: "Delete Completed download" }));
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(api.remove).not.toHaveBeenCalled();
  });

  it("clears history directly when confirmDeletion is false", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: { confirmDeletion: false } });
    const user = userEvent.setup();
    const { notify } = await renderPageLoaded([operation("failed", "Failed download", "failed")]);

    await user.click(screen.getByRole("button", { name: "Clear history" }));

    await waitFor(() => expect(api.clearHistory).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(notify).toHaveBeenCalledWith("3 operations removed from history");
  });

  it("shows a confirm dialog before clearing history when confirmDeletion is true", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: { confirmDeletion: true } });
    const user = userEvent.setup();
    await renderPageLoaded([operation("failed", "Failed download", "failed")]);

    await user.click(screen.getByRole("button", { name: "Clear history" }));
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(api.clearHistory).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(api.clearHistory).toHaveBeenCalledTimes(1));
  });
});
