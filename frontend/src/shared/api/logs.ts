import { call } from "./bridge";

export const logsApi = {
  list: (limit?: number) => call<string[]>("LogController", "ListLogs", limit ?? 0),
  exportLogs: () => call<string>("LogController", "ExportLogs"),
  openDirectory: () => call<void>("LogController", "OpenLogsDirectory"),
};
