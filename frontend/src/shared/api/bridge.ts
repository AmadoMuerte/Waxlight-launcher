import { log } from "../lib/logger";

export class BackendUnavailableError extends Error {
  constructor() {
    super(
      "The Wails backend is unavailable. Start Waxlight with `wails dev` or use the built desktop application.",
    );
  }
}

export async function call<T>(controller: string, method: string, ...args: unknown[]): Promise<T> {
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

export function errorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  if (typeof error === "string") {
    return error.replace(/^[A-Z_]+:\s*/, "");
  }
  return "An unknown error occurred";
}
