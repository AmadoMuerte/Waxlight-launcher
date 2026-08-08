import { BackendUnavailableError } from "../api/bridge";
import { logsApi, type LogLevel } from "../api/logs";

const levelOrder: Record<LogLevel, number> = { debug: 0, info: 1, warn: 2, error: 3 };

const FLUSH_DELAY_MS = 500;
const MAX_QUEUE_LINES = 50;
const MAX_MESSAGE_LENGTH = 4000;
const MAX_ATTR_LENGTH = 1000;
const DEDUP_WINDOW_MS = 30_000;

interface PendingLine {
  level: LogLevel;
  message: string;
  attrs?: Record<string, string>;
}

let minLevel: LogLevel = "info";
let queue: PendingLine[] = [];
let flushTimer: ReturnType<typeof setTimeout> | null = null;

// lastLine tracks the most recent emitted line so identical repeats within
// DEDUP_WINDOW_MS are suppressed instead of flooding the launcher console.
// Polling watchers and unavailable backend calls would otherwise repeat the
// same error every few seconds.
let lastLine: { key: string; level: LogLevel; count: number; windowStart: number } | null = null;

// setMinLevel adjusts the forwarding threshold. Lines below it never reach
// the backend.
export function setMinLevel(level: LogLevel) {
  minLevel = level;
}

// resetDedupState clears the repeat-suppression state. It exists for tests so
// bursts do not leak across cases.
export function resetDedupState() {
  lastLine = null;
}

function dedupKey(level: LogLevel, message: string, attrs?: Record<string, string>): string {
  if (!attrs) {
    return `${level}|${message}`;
  }
  const pairs = Object.entries(attrs)
    .toSorted(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
    .map(([key, value]) => `${key}=${value}`);
  return `${level}|${message}|${pairs.join("&")}`;
}

function enqueue(line: PendingLine) {
  queue.push(line);
  if (queue.length >= MAX_QUEUE_LINES) {
    flushLogs();
    return;
  }
  if (flushTimer === null) {
    flushTimer = setTimeout(() => {
      flushTimer = null;
      flushLogs();
    }, FLUSH_DELAY_MS);
  }
}

// flushLogs forwards all queued lines to the backend. It is safe to call
// repeatedly; an empty queue is a no-op. When the backend is unavailable the
// batch falls back to the browser console instead of being dropped.
export function flushLogs() {
  if (flushTimer !== null) {
    clearTimeout(flushTimer);
    flushTimer = null;
  }
  if (queue.length === 0) {
    return;
  }
  const batch = queue;
  queue = [];
  void Promise.allSettled(
    batch.map(async (line) => {
      try {
        await logsApi.write(line.level, line.message.slice(0, MAX_MESSAGE_LENGTH), line.attrs);
      } catch (error) {
        if (error instanceof BackendUnavailableError) {
          fallback(line);
        }
      }
    }),
  );
}

function fallback(line: PendingLine) {
  const fn =
    console[
      line.level === "debug"
        ? "debug"
        : line.level === "info"
          ? "info"
          : line.level === "warn"
            ? "warn"
            : "error"
    ].bind(console);
  fn(`[launcher] ${line.message}`, line.attrs ?? {});
}

function truncate(value: string, max = MAX_ATTR_LENGTH): string {
  return value.length > max ? `${value.slice(0, max)}…` : value;
}

function write(level: LogLevel, message: string, attrs?: Record<string, string>) {
  if (levelOrder[level] < levelOrder[minLevel]) {
    return;
  }
  const key = dedupKey(level, message, attrs);
  const now = Date.now();
  if (lastLine !== null && lastLine.key === key && now - lastLine.windowStart < DEDUP_WINDOW_MS) {
    lastLine.count++;
    return;
  }
  if (lastLine !== null && lastLine.count > 1) {
    const suppressed = lastLine.count - 1;
    enqueue({
      level: lastLine.level,
      message: `Suppressed ${suppressed} repeated log line${suppressed === 1 ? "" : "s"}`,
    });
  }
  lastLine = { key, level, count: 1, windowStart: now };
  enqueue({ level, message, attrs });
}

export const log = {
  debug: (message: string, attrs?: Record<string, string>) => write("debug", message, attrs),
  info: (message: string, attrs?: Record<string, string>) => write("info", message, attrs),
  warn: (message: string, attrs?: Record<string, string>) => write("warn", message, attrs),
  error: (message: string, attrs?: Record<string, string>) => write("error", message, attrs),
};

// installGlobalErrorLogging forwards uncaught browser errors and unhandled
// promise rejections to the launcher log. Returns a cleanup function.
export function installGlobalErrorLogging(): () => void {
  const onError = (event: ErrorEvent) => {
    log.error(event.message || "Uncaught window error", {
      source: truncate(event.filename ?? ""),
      line: String(event.lineno),
      column: String(event.colno),
    });
  };
  const onRejection = (event: PromiseRejectionEvent) => {
    const reason = event.reason;
    const message =
      reason instanceof Error
        ? reason.message
        : typeof reason === "string"
          ? reason
          : "Unhandled promise rejection";
    const attrs: Record<string, string> | undefined =
      reason instanceof Error && reason.stack ? { stack: truncate(reason.stack) } : undefined;
    log.error(message, attrs);
  };
  window.addEventListener("error", onError);
  window.addEventListener("unhandledrejection", onRejection);
  return () => {
    window.removeEventListener("error", onError);
    window.removeEventListener("unhandledrejection", onRejection);
  };
}
