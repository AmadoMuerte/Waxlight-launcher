import { call } from "./bridge";
import type { Instance } from "./types";

export const instancesApi = {
  list: () => call<Instance[]>("InstanceController", "ListInstances"),
  create: (request: {
    name: string;
    description: string;
    gameVersionId: string;
    gameClient?: "vanilla" | "optimum";
    defaultAccountId?: string;
    directory: string;
    launchArguments: string[];
  }) => call<Instance>("InstanceController", "CreateInstance", request),
  update: (request: {
    id: string;
    name: string;
    description: string;
    gameVersionId: string;
    gameClient?: "vanilla" | "optimum";
    defaultAccountId?: string;
    launchArguments: string[];
    coverSourcePath?: string;
  }) => call<Instance>("InstanceController", "UpdateInstance", request),
  remove: (id: string, deleteFiles: boolean) =>
    call<void>("InstanceController", "DeleteInstance", id, deleteFiles),
  clone: (request: { sourceId: string; name: string }) =>
    call<Instance>("InstanceController", "CloneInstance", request),
  selectCover: () => call<string>("InstanceController", "SelectInstanceCover"),
};
