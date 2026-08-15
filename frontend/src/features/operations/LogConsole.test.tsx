// @vitest-environment jsdom

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useToastStore } from "../../app/stores/toast";
import { LogConsole } from "./LogConsole";

const api = vi.hoisted(() => ({
  list: vi.fn(),
  exportLogs: vi.fn(),
  openDirectory: vi.fn(),
  eventsOn: vi.fn(),
  copyHandler: undefined as ((event: KeyboardEvent) => boolean) | undefined,
  hasSelection: false,
  selection: "",
  clipboardWrite: vi.fn(),
}));

vi.mock("../../shared/api/logs", () => ({
  logsApi: {
    list: api.list,
    exportLogs: api.exportLogs,
    openDirectory: api.openDirectory,
  },
}));

vi.mock("@xterm/xterm", () => ({
  Terminal: class {
    write = vi.fn();
    clear = vi.fn();
    dispose = vi.fn();
    loadAddon = vi.fn();
    open = vi.fn();
    attachCustomKeyEventHandler = vi.fn((handler: (event: KeyboardEvent) => boolean) => {
      api.copyHandler = handler;
    });
    hasSelection = vi.fn(() => api.hasSelection);
    getSelection = vi.fn(() => api.selection);
  },
}));

vi.mock("@xterm/addon-fit", () => ({
  FitAddon: class {
    fit = vi.fn();
  },
}));

vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: (...args: unknown[]) => {
    api.eventsOn(...args);
    return () => undefined;
  },
}));

function renderConsole() {
  const notify = vi.fn();
  useToastStore.setState({ notify });
  render(<LogConsole />);
  return { notify };
}

describe("LogConsole", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal(
      "ResizeObserver",
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      },
    );
    api.list.mockResolvedValue([]);
    api.exportLogs.mockResolvedValue("");
    api.openDirectory.mockResolvedValue(undefined);
    api.copyHandler = undefined;
    api.hasSelection = false;
    api.selection = "";
    api.clipboardWrite.mockReset();
    vi.stubGlobal("navigator", { clipboard: { writeText: api.clipboardWrite } });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders the console header and its actions", () => {
    renderConsole();
    expect(screen.getByText("Launcher console")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Export logs" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Copy logs" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Clear console" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Open logs folder" })).toBeTruthy();
  });

  it("opens the logs directory", async () => {
    const user = userEvent.setup();
    renderConsole();

    await user.click(screen.getByRole("button", { name: "Open logs folder" }));

    await waitFor(() => expect(api.openDirectory).toHaveBeenCalledTimes(1));
  });

  it("reports directory open failures", async () => {
    const user = userEvent.setup();
    api.openDirectory.mockRejectedValue(new Error("cannot open"));
    const { notify } = renderConsole();

    await user.click(screen.getByRole("button", { name: "Open logs folder" }));

    await waitFor(() => expect(api.openDirectory).toHaveBeenCalledTimes(1));
    expect(notify).toHaveBeenCalledWith("cannot open", "error");
  });

  it("loads recent logs on mount and subscribes to new lines", async () => {
    renderConsole();
    await waitFor(() => expect(api.list).toHaveBeenCalledTimes(1));
    expect(api.eventsOn).toHaveBeenCalledWith("logs:append", expect.any(Function));
  });

  it("copies the selected terminal text with Ctrl+C", async () => {
    renderConsole();
    await waitFor(() => expect(api.copyHandler).toBeTypeOf("function"));
    api.hasSelection = true;
    api.selection = "selected log line";

    expect(api.copyHandler?.(new KeyboardEvent("keydown", { key: "c", ctrlKey: true }))).toBe(
      false,
    );
    await waitFor(() => expect(api.clipboardWrite).toHaveBeenCalledWith("selected log line"));
  });

  it("exports logs and reports the saved path", async () => {
    const user = userEvent.setup();
    api.exportLogs.mockResolvedValue("/tmp/waxlight-logs.txt");
    const { notify } = renderConsole();

    await user.click(screen.getByRole("button", { name: "Export logs" }));

    await waitFor(() => expect(api.exportLogs).toHaveBeenCalledTimes(1));
    expect(notify).toHaveBeenCalledWith("Logs saved to /tmp/waxlight-logs.txt");
  });

  it("reports export failures", async () => {
    const user = userEvent.setup();
    api.exportLogs.mockRejectedValue(new Error("dialog closed"));
    const { notify } = renderConsole();

    await user.click(screen.getByRole("button", { name: "Export logs" }));

    await waitFor(() => expect(api.exportLogs).toHaveBeenCalledTimes(1));
    expect(notify).toHaveBeenCalledWith("dialog closed", "error");
  });
});
