import { log } from "../lib/logger";

export class BackendUnavailableError extends Error {
  constructor() {
    super(
      "The Wails backend is unavailable. Start Waxlight with `wails dev` or use the built desktop application.",
    );
  }
}

async function invoke<T>(
  controller: string,
  method: string,
  args: unknown[],
  logFailures: boolean,
): Promise<T> {
  const namespaces = window.go;
  const callable = namespaces?.wails?.[controller]?.[method];

  if (typeof callable !== "function") {
    log.error("Backend call failed: backend unavailable", { controller, method });
    throw new BackendUnavailableError();
  }

  try {
    return (await callable(...args)) as T;
  } catch (error) {
    // A failing WriteLog would recurse through call() into log.error() and
    // back into WriteLog. Its own availability error is only reported when
    // the backend is gone entirely; the logger's console fallback covers it.
    if (
      logFailures &&
      !(
        error instanceof BackendUnavailableError &&
        controller === "LogController" &&
        method === "WriteLog"
      )
    ) {
      log.error(errorMessage(error), { controller, method });
    }
    throw error;
  }
}

export function call<T>(controller: string, method: string, ...args: unknown[]): Promise<T> {
  return invoke<T>(controller, method, args, true);
}

// Best-effort background requests handle their own errors and should not turn
// expected stale-data races into console errors.
export function callQuietly<T>(controller: string, method: string, ...args: unknown[]): Promise<T> {
  return invoke<T>(controller, method, args, false);
}

export function errorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  if (typeof error === "string") {
    return error.replace(/^[A-Z_]+:\s*/, "");
  }
  return "An unknown error occurred";
}

// errorCode extracts the errs code prefix (for example "FILE_PERMISSION_DENIED")
// from a backend error so pages can react to known failure classes. Returns ""
// when the error carries no code.
export function errorCode(error: unknown): string {
  const message = error instanceof Error ? error.message : typeof error === "string" ? error : "";
  return /^([A-Z][A-Z_]*):/.exec(message)?.[1] ?? "";
}
