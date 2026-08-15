import { useQuery } from "@tanstack/react-query";

import { OPTIMUM_STATUS_QUERY_KEY, SETTINGS_QUERY_KEY } from "../../shared/api/keys";
import { settingsApi } from "./api";
import type { Settings } from "./model";

interface QueryOptions {
  refetchInterval?: number | false;
}

export function useSettingsQuery(options?: QueryOptions) {
  return useQuery({
    queryKey: SETTINGS_QUERY_KEY,
    queryFn: settingsApi.get,
    staleTime: Number.POSITIVE_INFINITY,
    ...options,
  });
}

export function useOptimumStatusQuery() {
  return useQuery({
    queryKey: OPTIMUM_STATUS_QUERY_KEY,
    queryFn: settingsApi.getOptimumStatus,
  });
}

export type { Settings };
