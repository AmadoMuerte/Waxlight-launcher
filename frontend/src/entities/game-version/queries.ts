import { useQuery } from "@tanstack/react-query";

import { GAME_VERSIONS_QUERY_KEY } from "../../shared/api/keys";
import { versionsApi } from "./api";
import type { GameVersion } from "./model";

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

export type { GameVersion };
