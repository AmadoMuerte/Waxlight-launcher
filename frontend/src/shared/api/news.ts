import { call, callQuietly } from "./bridge";
import type { NewsFeed } from "./types";

export const newsApi = {
  sync: (force = false) => call<NewsFeed>("NewsController", "Sync", force),
  markSeen: (ids: string[]) => callQuietly<void>("NewsController", "MarkSeen", ids),
  openArticle: (url: string) => call<void>("NewsController", "OpenArticle", url),
};
