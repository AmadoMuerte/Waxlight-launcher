import { useQuery } from "@tanstack/react-query";

import { STATISTICS_QUERY_KEY } from "../../shared/api/keys";
import { statisticsApi } from "./api";
import type { Statistics } from "./model";

interface QueryOptions {
  refetchInterval?: number | false;
}

export function useStatisticsQuery(options?: QueryOptions) {
  return useQuery({
    queryKey: STATISTICS_QUERY_KEY,
    queryFn: statisticsApi.overview,
    ...options,
  });
}

export type { Statistics };
