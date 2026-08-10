import { call } from "./bridge";
import type { FavoriteServer, PublicServer } from "./types";

export const serversApi = {
  listPublic: () => call<PublicServer[]>("ServerController", "ListPublicServers"),
  listFavorites: () => call<FavoriteServer[]>("ServerController", "ListFavoriteServers"),
  saveFavorite: (request: { id: string; name: string; address: string; instanceId?: string }) =>
    call<FavoriteServer>("ServerController", "SaveFavoriteServer", request),
  removeFavorite: (id: string) => call<void>("ServerController", "DeleteFavoriteServer", id),
};
