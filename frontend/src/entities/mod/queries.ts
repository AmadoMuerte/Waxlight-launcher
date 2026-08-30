import {
  infiniteQueryOptions,
  queryOptions,
  useInfiniteQuery,
  useQuery,
} from "@tanstack/react-query";

import {
  DOWNLOADED_MODS_QUERY_KEY,
  MOD_DETAILS_QUERY_KEY,
  MOD_TAGS_QUERY_KEY,
} from "../../shared/api/keys";
import { modCatalogApi } from "./api";
import type { ModSearchQuery, ModTag, ModSummary } from "./model";

const CATALOG_STALE_TIME = 5 * 60_000;

export const DEFAULT_MOD_CATALOG_QUERY: Omit<ModSearchQuery, "page"> = {
  text: "",
  gameVersion: "",
  side: "",
  updatedAfter: undefined,
  tags: [],
  compatibleOnly: false,
  instanceId: "",
  sort: "updated",
  pageSize: 24,
};

export function downloadedModsQueryOptions() {
  return queryOptions({
    queryKey: DOWNLOADED_MODS_QUERY_KEY,
    queryFn: modCatalogApi.downloaded,
  });
}

export function modTagsQueryOptions() {
  return queryOptions({
    queryKey: MOD_TAGS_QUERY_KEY,
    queryFn: modCatalogApi.tags,
    staleTime: CATALOG_STALE_TIME,
  });
}

export function modCatalogQueryOptions(query: Omit<ModSearchQuery, "page">) {
  return infiniteQueryOptions({
    queryKey: ["mods", "search", query],
    queryFn: ({ pageParam }) => modCatalogApi.search({ ...query, page: pageParam }),
    initialPageParam: 1,
    getNextPageParam: (lastPage) => (lastPage.hasNext ? lastPage.page + 1 : undefined),
    staleTime: CATALOG_STALE_TIME,
  });
}

export function useDownloadedModsQuery() {
  return useQuery(downloadedModsQueryOptions());
}

export function useModTagsQuery(enabled: boolean) {
  return useQuery({ ...modTagsQueryOptions(), enabled });
}

export function useModDetailsQuery(modId: string, enabled = true) {
  return useQuery({
    queryKey: MOD_DETAILS_QUERY_KEY(modId),
    queryFn: () => modCatalogApi.get(modId),
    enabled,
    staleTime: CATALOG_STALE_TIME,
  });
}

export function useModCatalogQuery(query: Omit<ModSearchQuery, "page">, enabled: boolean) {
  return useInfiniteQuery({ ...modCatalogQueryOptions(query), enabled });
}

export type { ModSummary, ModTag };
