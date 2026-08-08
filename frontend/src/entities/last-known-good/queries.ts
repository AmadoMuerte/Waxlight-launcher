import { useQuery } from "@tanstack/react-query";

import { LAST_KNOWN_GOOD_QUERY_KEY } from "../../shared/api/keys";
import { lastKnownGoodApi } from "./api";
import type { LastKnownGood } from "./model";

export function useLastKnownGoodQuery(instanceId: string) {
  return useQuery({
    queryKey: LAST_KNOWN_GOOD_QUERY_KEY(instanceId),
    queryFn: () => lastKnownGoodApi.get(instanceId),
    enabled: Boolean(instanceId),
  });
}

export type { LastKnownGood };
