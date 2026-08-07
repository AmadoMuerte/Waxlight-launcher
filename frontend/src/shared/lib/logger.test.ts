// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { BackendUnavailableError } from "../api/bridge";
import { logsApi } from "../api/logs";
import { flushLogs, installGlobalErrorLogging, log, setMinLevel } from "./logger";

vi.mock("../api/bridge", () => ({
  BackendUnavailableError: class MockBackendUnavailableError extends Error {},
}));

vi.mock("../api/logs", () => ({
  logsApi: { write: vi.fn() },
}));

const write = vi.mocked(logsApi.write);

async function flushAndSettle() {
  flushLogs();
  await vi.advanceTimersByTimeAsync(0);
}

describe("log", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    setMinLevel("debug");
    write.mockResolvedValue(undefined);
  });

  afterEach(() => {
    flushLogs();
    vi.clearAllMocks();
    vi.useRealTimers();
    setMinLevel("info");
  });

  it("forwards queued lines to the backend after the flush delay", async () => {
    log.info("hello");
    log.error("boom", { where: "page" });
    expect(write).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(500);

    expect(write).toHaveBeenCalledTimes(2);
    expect(write).toHaveBeenNthCalledWith(1, "info", "hello", undefined);
    expect(write).toHaveBeenNthCalledWith(2, "error", "boom", { where: "page" });
  });

  it("flushes immediately when the queue cap is reached", async () => {
    for (let index = 0; index < 50; index++) {
      log.debug(`line ${index}`);
    }
    await flushAndSettle();
    expect(write).toHaveBeenCalledTimes(50);
  });

  it("drops lines below the minimum level", async () => {
    setMinLevel("warn");
    log.debug("hidden");
    log.info("hidden");
    log.warn("shown");
    log.error("shown");
    await flushAndSettle();
    expect(write).toHaveBeenCalledTimes(2);
    expect(write.mock.calls.map(([level]) => level)).toEqual(["warn", "error"]);
  });

  it("truncates overlong messages", async () => {
    log.info("x".repeat(5000));
    await flushAndSettle();
    expect(write.mock.calls[0][1]).toHaveLength(4000);
  });

  it("falls back to the browser console when the backend is unavailable", async () => {
    write.mockRejectedValue(new BackendUnavailableError());
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    log.error("no backend");
    await flushAndSettle();

    expect(write).toHaveBeenCalledTimes(1);
    expect(consoleSpy).toHaveBeenCalledWith("[launcher] no backend", {});
  });
});

describe("installGlobalErrorLogging", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    setMinLevel("debug");
    write.mockResolvedValue(undefined);
  });

  afterEach(() => {
    flushLogs();
    vi.clearAllMocks();
    vi.useRealTimers();
    setMinLevel("info");
  });

  it("reports uncaught window errors", async () => {
    const cleanup = installGlobalErrorLogging();
    window.dispatchEvent(
      new ErrorEvent("error", { message: "oops", filename: "page.ts", lineno: 4, colno: 2 }),
    );

    await vi.advanceTimersByTimeAsync(500);

    expect(write).toHaveBeenCalledTimes(1);
    expect(write.mock.calls[0][0]).toBe("error");
    expect(write.mock.calls[0][1]).toBe("oops");
    expect(write.mock.calls[0][2]).toEqual({ source: "page.ts", line: "4", column: "2" });
    cleanup();
  });

  it("reports unhandled promise rejections with the error stack", async () => {
    const cleanup = installGlobalErrorLogging();
    const rejection = new Event("unhandledrejection");
    Object.assign(rejection, { reason: new Error("rejected") });
    window.dispatchEvent(rejection);

    await vi.advanceTimersByTimeAsync(500);

    expect(write).toHaveBeenCalledTimes(1);
    const [level, message, attrs] = write.mock.calls[0];
    expect(level).toBe("error");
    expect(message).toBe("rejected");
    expect(attrs?.stack).toContain("Error: rejected");
    cleanup();
  });

  it("stops reporting after cleanup", async () => {
    const cleanup = installGlobalErrorLogging();
    cleanup();
    window.dispatchEvent(new ErrorEvent("error", { message: "oops" }));

    await vi.advanceTimersByTimeAsync(500);

    expect(write).not.toHaveBeenCalled();
  });
});
