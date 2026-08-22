import { useQuery } from "@tanstack/react-query";

import { NEWS_QUERY_KEY } from "../../shared/api/keys";
import { newsApi } from "./api";

const NEWS_REFRESH_INTERVAL = 60 * 60_000;

export function useNewsQuery() {
  return useQuery({
    queryKey: NEWS_QUERY_KEY,
    queryFn: () => newsApi.sync(false),
    refetchInterval: NEWS_REFRESH_INTERVAL,
  });
}

export type { NewsCategory, NewsFeed, NewsItem } from "./model";
