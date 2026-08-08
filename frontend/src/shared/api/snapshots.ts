import { call } from "./bridge";
import type { InstanceSnapshot, Operation } from "./types";

export const snapshotsApi = {
  create: (instanceId: string) =>
    call<Operation>("SnapshotController", "CreateInstanceSnapshot", instanceId),
  list: (instanceId: string) =>
    call<InstanceSnapshot[]>("SnapshotController", "ListInstanceSnapshots", instanceId),
  restore: (instanceId: string, snapshotId: string) =>
    call<void>("SnapshotController", "RestoreInstanceSnapshot", instanceId, snapshotId),
  remove: (instanceId: string, snapshotId: string) =>
    call<void>("SnapshotController", "DeleteInstanceSnapshot", instanceId, snapshotId),
};
