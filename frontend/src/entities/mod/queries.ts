import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import {
  DOWNLOADED_MODS_QUERY_KEY,
  MOD_DETAILS_QUERY_KEY,
  MOD_TAGS_QUERY_KEY,
} from "../../shared/api/keys";
import { modCatalogApi } from "./api";
import type { ModSearchQuery, ModTag, ModSummary } from "./model";

export function useDownloadedModsQuery() {
  return useQuery({
    queryKey: DOWNLOADED_MODS_QUERY_KEY,
    queryFn: modCatalogApi.downloaded,
  });
}

export function useModTagsQuery(enabled: boolean) {
  return useQuery({
    queryKey: MOD_TAGS_QUERY_KEY,
    queryFn: modCatalogApi.tags,
    enabled,
  });
}

export function useModDetailsQuery(modId: string, enabled = true) {
  return useQuery({
    queryKey: MOD_DETAILS_QUERY_KEY(modId),
    queryFn: () => modCatalogApi.get(modId),
    enabled,
  });
}

export function useModCatalogQuery(query: Omit<ModSearchQuery, "page">, enabled: boolean) {
  return useInfiniteQuery({
    queryKey: ["mods", "search", query],
    queryFn: ({ pageParam }) => modCatalogApi.search({ ...query, page: pageParam }),
    initialPageParam: 1,
    getNextPageParam: (lastPage) => (lastPage.hasNext ? lastPage.page + 1 : undefined),
    enabled,
  });
}

export type { ModSummary, ModTag };
