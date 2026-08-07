import { useQuery } from "@tanstack/react-query";

import { AVAILABLE_GAME_VERSIONS_QUERY_KEY, GAME_VERSIONS_QUERY_KEY } from "../../shared/api/keys";
import { versionsApi } from "./api";
import type { AvailableGameVersion, GameVersion } from "./model";

interface QueryOptions {
  refetchInterval?: number | false;
}

export function useGameVersionsQuery(options?: QueryOptions) {
  return useQuery({
    queryKey: GAME_VERSIONS_QUERY_KEY,
    queryFn: versionsApi.list,
    ...options,
  });
}

export function useAvailableGameVersionsQuery(options?: QueryOptions) {
  return useQuery({
    queryKey: AVAILABLE_GAME_VERSIONS_QUERY_KEY,
    queryFn: versionsApi.available,
    staleTime: 5 * 60_000,
    ...options,
  });
}

export type { AvailableGameVersion, GameVersion };
