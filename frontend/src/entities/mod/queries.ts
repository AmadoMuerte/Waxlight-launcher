import {
  infiniteQueryOptions,
  queryOptions,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";

import { useToastStore } from "../../app/stores/toast";
import { errorMessage } from "../../shared/api/bridge";
import {
  DOWNLOADED_MODS_QUERY_KEY,
  FAVORITE_MOD_IDS_QUERY_KEY,
  FAVORITE_MODS_QUERY_KEY,
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

export function useFavoriteModIDsQuery() {
  return useQuery({ queryKey: FAVORITE_MOD_IDS_QUERY_KEY, queryFn: modCatalogApi.favoriteIDs });
}

export function useFavoriteModsQuery(enabled: boolean) {
  return useQuery({
    queryKey: FAVORITE_MODS_QUERY_KEY,
    queryFn: modCatalogApi.favorites,
    enabled,
    staleTime: CATALOG_STALE_TIME,
  });
}

export function useSetModFavoriteMutation() {
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  return useMutation({
    mutationFn: ({ modId, favorite }: { modId: string; favorite: boolean }) =>
      modCatalogApi.setFavorite(modId, favorite),
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: FAVORITE_MOD_IDS_QUERY_KEY }),
        queryClient.invalidateQueries({ queryKey: FAVORITE_MODS_QUERY_KEY }),
      ]),
    onError: (error) => notify(errorMessage(error), "error"),
  });
}

export type { ModSummary, ModTag };
