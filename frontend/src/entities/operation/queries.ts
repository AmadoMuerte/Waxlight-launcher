import { useQuery } from "@tanstack/react-query";

import { OPERATIONS_QUERY_KEY } from "../../shared/api/keys";
import { operationsApi } from "./api";
import type { Operation } from "./model";

interface QueryOptions {
  refetchInterval?: number | false;
}

export function useOperationsQuery(options?: QueryOptions) {
  return useQuery({
    queryKey: OPERATIONS_QUERY_KEY,
    queryFn: operationsApi.list,
    ...options,
    refetchInterval: options?.refetchInterval
      ? (query) => (query.state.error ? false : options.refetchInterval)
      : false,
  });
}

export type { Operation };
