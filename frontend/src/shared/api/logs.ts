import { call } from "./bridge";

export type LogLevel = "debug" | "info" | "warn" | "error";

export const logsApi = {
  list: (limit?: number) => call<string[]>("LogController", "ListLogs", limit ?? 0),
  exportLogs: () => call<string>("LogController", "ExportLogs"),
  openDirectory: () => call<void>("LogController", "OpenLogsDirectory"),
  write: (level: LogLevel, message: string, attrs?: Record<string, string>) =>
    call<void>("LogController", "WriteLog", level, message, attrs ?? {}),
};
