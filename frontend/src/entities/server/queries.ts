import { useQuery } from "@tanstack/react-query";

import { FAVORITE_SERVERS_QUERY_KEY, PUBLIC_SERVERS_QUERY_KEY } from "../../shared/api/keys";
import { serversApi } from "./api";

export function useFavoriteServersQuery() {
  return useQuery({ queryKey: FAVORITE_SERVERS_QUERY_KEY, queryFn: serversApi.listFavorites });
}

export function usePublicServersQuery() {
  return useQuery({
    queryKey: PUBLIC_SERVERS_QUERY_KEY,
    queryFn: serversApi.listPublic,
    // The catalog is large and static enough for a launcher session. Users can
    // explicitly refresh it instead of paying for background list updates.
    staleTime: Infinity,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
}
