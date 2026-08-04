export class BackendUnavailableError extends Error {
  constructor() {
    super(
      "The Wails backend is unavailable. Start Waxlight with `wails dev` or use the built desktop application.",
    );
  }
}

export async function call<T>(controller: string, method: string, ...args: unknown[]): Promise<T> {
  const namespaces = window.go;
  const callable = namespaces?.presentation?.[controller]?.[method];

  if (typeof callable !== "function") {
    throw new BackendUnavailableError();
  }

  return callable(...args) as Promise<T>;
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
