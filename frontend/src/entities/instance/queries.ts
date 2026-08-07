import { useQuery } from "@tanstack/react-query";

import { INSTANCES_QUERY_KEY } from "../../shared/api/keys";
import { instancesApi } from "./api";
import type { Instance } from "./model";

interface QueryOptions {
  refetchInterval?: number | false;
}

export function useInstancesQuery(options?: QueryOptions) {
  return useQuery({
    queryKey: INSTANCES_QUERY_KEY,
    queryFn: instancesApi.list,
    ...options,
  });
}

export type { Instance };
