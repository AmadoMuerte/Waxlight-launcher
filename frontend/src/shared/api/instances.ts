import { call } from "./bridge";
import type { ExistingDataImportRequest, Instance, MigrationCandidate, Operation } from "./types";

export const instancesApi = {
  list: () => call<Instance[]>("InstanceController", "ListInstances"),
  get: (id: string) => call<Instance>("InstanceController", "GetInstance", id),
  detectExistingData: () =>
    call<MigrationCandidate[]>("InstanceController", "DetectExistingVintageStoryData"),
  inspectExistingData: (path: string) =>
    call<MigrationCandidate>("InstanceController", "InspectExistingVintageStoryData", path),
  importExistingData: (request: ExistingDataImportRequest) =>
    call<Operation>("InstanceController", "StartExistingDataImport", request),
  create: (request: {
    name: string;
    description: string;
    gameVersionId: string;
    gameClient?: "vanilla" | "optimum";
    defaultAccountId?: string;
    directory: string;
    launchArguments: string[];
    environmentVariables?: Record<string, string>;
  }) => call<Instance>("InstanceController", "CreateInstance", request),
  update: (request: {
    id: string;
    name: string;
    description: string;
    gameVersionId: string;
    gameClient?: "vanilla" | "optimum";
    defaultAccountId?: string;
    launchArguments: string[];
    environmentVariables?: Record<string, string>;
    coverSourcePath?: string;
  }) => call<Instance>("InstanceController", "UpdateInstance", request),
  setPinned: (id: string, pinned: boolean) =>
    call<Instance>("InstanceController", "SetInstancePinned", id, pinned),
  remove: (id: string, deleteFiles: boolean) =>
    call<void>("InstanceController", "DeleteInstance", id, deleteFiles),
  clone: (request: { sourceId: string; name: string }) =>
    call<Instance>("InstanceController", "CloneInstance", request),
  selectCover: () => call<string>("InstanceController", "SelectInstanceCover"),
};
