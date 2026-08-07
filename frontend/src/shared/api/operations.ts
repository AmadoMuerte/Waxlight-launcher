import { call } from "./bridge";
import type { Operation } from "./types";

export const operationsApi = {
  list: () => call<Operation[]>("OperationController", "ListOperations"),
  cancel: (id: string) => call<void>("OperationController", "CancelOperation", id),
  remove: (id: string) => call<void>("OperationController", "DeleteOperation", id),
  clearHistory: () => call<number>("OperationController", "ClearOperationHistory"),
};
