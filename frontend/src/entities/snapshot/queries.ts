import { useQuery } from "@tanstack/react-query";

import { SNAPSHOTS_QUERY_KEY } from "../../shared/api/keys";
import { snapshotsApi } from "./api";
import type { InstanceSnapshot } from "./model";

export function useInstanceSnapshotsQuery(instanceId: string) {
  return useQuery({
    queryKey: SNAPSHOTS_QUERY_KEY(instanceId),
    queryFn: () => snapshotsApi.list(instanceId),
    enabled: Boolean(instanceId),
  });
}

export type { InstanceSnapshot };
