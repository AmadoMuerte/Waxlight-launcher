import { useQueryClient } from "@tanstack/react-query";
import { Globe2, Heart, RefreshCw } from "lucide-react";
import { startTransition, useCallback, useDeferredValue, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router";

import { useToastStore } from "../../app/stores/toast";
import { useGameVersionsQuery } from "../../entities/game-version/queries";
import { useInstancesQuery } from "../../entities/instance/queries";
import { serversApi } from "../../entities/server/api";
import type { FavoriteServer, PublicServer } from "../../entities/server/model";
import {
  useFavoriteServersQuery,
  usePublicServerDetailsQuery,
  usePublicServersQuery,
} from "../../entities/server/queries";
import { serverKey } from "../../features/servers/lib";
import { ServerCard } from "../../features/servers/ServerCard";
import { ServerDetailsDialog } from "../../features/servers/ServerDetailsDialog";
import { ServerJoinDialog } from "../../features/servers/ServerJoinDialog";
import { errorMessage } from "../../shared/api/bridge";
import { FAVORITE_SERVERS_QUERY_KEY } from "../../shared/api/keys";
import { launcherApi } from "../../shared/api/launcher";
import { normalizeServerAddress, serverShareURL } from "../../shared/lib/waxlight-links";
import { Button } from "../../shared/ui/button";
import { Checkbox } from "../../shared/ui/checkbox-control";
import { EmptyState } from "../../shared/ui/empty";
import { ErrorState } from "../../shared/ui/error-state";
import { IconButton } from "../../shared/ui/icon-button";
import { LoadingState } from "../../shared/ui/loading-state";
import { Page, PageContent } from "../../shared/ui/page";
import { PageHeader } from "../../shared/ui/page-header";
import { SearchInput } from "../../shared/ui/search-input";
import { Tabs } from "../../shared/ui/tabs";
import { Toolbar, ToolbarGroup } from "../../shared/ui/toolbar";
import { ClipboardSetText } from "../../wailsjs/runtime/runtime";

type ServerTab = "favorites" | "public";

const SERVER_PAGE_SIZE = 60;

function favoriteAsPublicServer(favorite: FavoriteServer): PublicServer {
  return {
    name: favorite.name,
    address: favorite.address,
    description: "",
    players: 0,
    modCount: 0,
    requiresWhitelist: false,
    accessRestricted: false,
    joinable: true,
  };
}

export function ServersPage() {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  const favoriteServers = useFavoriteServersQuery();
  const { data: favorites = [] } = favoriteServers;
  const publicServers = usePublicServersQuery();
  const { data: instances = [] } = useInstancesQuery();
  const { data: versions = [] } = useGameVersionsQuery();
  const [activeTab, setActiveTab] = useState<ServerTab>("public");
  const [search, setSearch] = useState("");
  const [showWhitelistServers, setShowWhitelistServers] = useState(false);
  const [visibleServerCount, setVisibleServerCount] = useState(SERVER_PAGE_SIZE);
  const consumedDeepLink = useRef("");
  const [detailsServer, setDetailsServer] = useState<PublicServer>();
  const [joinTarget, setJoinTarget] = useState<{
    server: PublicServer;
    favorite?: FavoriteServer;
  }>();
  const [joiningKey, setJoiningKey] = useState<string | null>(null);
  const [favoriteBusyKey, setFavoriteBusyKey] = useState<string | null>(null);
  const deferredSearch = useDeferredValue(search);
  const publicServerDetails = usePublicServerDetailsQuery(detailsServer?.id);

  const refreshFavorites = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: FAVORITE_SERVERS_QUERY_KEY });
  }, [queryClient]);

  const savePublicServer = useCallback(
    async (server: PublicServer) => {
      setFavoriteBusyKey(serverKey(server));
      try {
        await serversApi.saveFavorite({ id: "", name: server.name, address: server.address });
        await refreshFavorites();
        notify(t("server_saved_to_favorites"));
      } catch (error) {
        notify(errorMessage(error), "error");
      } finally {
        setFavoriteBusyKey(null);
      }
    },
    [notify, refreshFavorites, t],
  );

  const removeFavorite = useCallback(
    async (server: PublicServer, favorite: FavoriteServer) => {
      setFavoriteBusyKey(serverKey(server));
      try {
        await serversApi.removeFavorite(favorite.id);
        await refreshFavorites();
      } catch (error) {
        notify(errorMessage(error), "error");
      } finally {
        setFavoriteBusyKey(null);
      }
    },
    [notify, refreshFavorites],
  );

  const toggleFavorite = useCallback(
    (server: PublicServer, favorite?: FavoriteServer) => {
      if (favorite) {
        void removeFavorite(server, favorite);
        return;
      }
      void savePublicServer(server);
    },
    [removeFavorite, savePublicServer],
  );

  const launchWithInstance = useCallback(
    async (server: PublicServer, favorite: FavoriteServer | undefined, instanceId: string) => {
      setJoiningKey(serverKey(server));
      try {
        const address = server.address.trim();
        if (address) {
          await launcherApi.launchServer(instanceId, address);
          await serversApi.saveFavorite({
            id: favorite?.id ?? "",
            name: server.name,
            address,
            instanceId,
          });
          await refreshFavorites();
        } else {
          await launcherApi.launch(instanceId);
        }
        setJoinTarget(undefined);
        setDetailsServer(undefined);
        notify(t("server_opened_in_game"));
      } catch (error) {
        notify(errorMessage(error), "error");
      } finally {
        setJoiningKey(null);
      }
    },
    [notify, refreshFavorites, t],
  );

  const requestPlay = useCallback((server: PublicServer, favorite?: FavoriteServer) => {
    setDetailsServer(undefined);
    setJoinTarget({ server, favorite });
  }, []);

  const catalogByKey = new Map(
    (publicServers.data ?? []).map((server) => [serverKey(server), server]),
  );
  const favoriteByKey = new Map(favorites.map((server) => [serverKey(server), server]));
  const publicServerKeyCounts = new Map<string, number>();
  const instanceById = new Map(instances.map((instance) => [instance.id, instance]));
  const normalizedSearch = deferredSearch.trim().toLocaleLowerCase();
  const hasActivePublicFilters = search.trim() !== "" || showWhitelistServers;
  const catalog = (publicServers.data ?? []).filter((server) => {
    if (server.requiresWhitelist && !showWhitelistServers) return false;
    return (
      !normalizedSearch ||
      `${server.name} ${server.address} ${server.description}`
        .toLocaleLowerCase()
        .includes(normalizedSearch)
    );
  });
  const visiblePublicServers = catalog.slice(0, visibleServerCount);

  const detailsFavorite = detailsServer ? favoriteByKey.get(serverKey(detailsServer)) : undefined;
  const detailedServer = publicServerDetails.data ?? detailsServer;
  const detailsPreferredInstance = detailsFavorite?.instanceId
    ? instanceById.get(detailsFavorite.instanceId)
    : undefined;

  useEffect(() => {
    const address = normalizeServerAddress(
      location.state && typeof location.state === "object"
        ? Reflect.get(location.state, "deepLinkAddress")
        : undefined,
    );
    if (!address) {
      consumedDeepLink.current = "";
      return;
    }
    if (
      consumedDeepLink.current === address ||
      publicServers.isLoading ||
      favoriteServers.isLoading
    ) {
      return;
    }
    consumedDeepLink.current = address;

    const publicServer = (publicServers.data ?? []).find(
      (server) => normalizeServerAddress(server.address) === address,
    );
    const favorite = favorites.find((server) => normalizeServerAddress(server.address) === address);
    void navigate("/servers", { replace: true, state: null });
    setActiveTab("public");
    setDetailsServer(
      publicServer ??
        (favorite
          ? favoriteAsPublicServer(favorite)
          : {
              name: t("server"),
              address,
              description: t("server_not_in_catalog"),
              players: 0,
              modCount: 0,
              requiresWhitelist: false,
              accessRestricted: false,
              joinable: true,
            }),
    );
  }, [
    favoriteServers.isLoading,
    favorites,
    location.state,
    navigate,
    publicServers.data,
    publicServers.isLoading,
    t,
  ]);

  useEffect(() => {
    setVisibleServerCount(SERVER_PAGE_SIZE);
  }, [deferredSearch, showWhitelistServers]);

  function resetFilters() {
    setSearch("");
    setShowWhitelistServers(false);
  }

  async function copyWaxlightLink(address: string) {
    const url = serverShareURL(address);
    if (!url) {
      notify(t("invalid_waxlight_link"), "error");
      return;
    }
    try {
      if (!(await ClipboardSetText(url))) throw new Error("clipboard unavailable");
      notify(t("server_link_copied"));
    } catch {
      notify(t("waxlight_link_copy_failed"), "error");
    }
  }

  async function copyAddress(address: string) {
    if (!address) return;
    try {
      if (!(await ClipboardSetText(address))) throw new Error("clipboard unavailable");
      notify(t("server_address_copied"));
    } catch {
      notify(t("waxlight_link_copy_failed"), "error");
    }
  }

  return (
    <Page>
      <PageHeader
        eyebrow={t("server_browser")}
        title={t("servers")}
        description={t("servers_description")}
      />

      <PageContent>
        <Tabs
          label={t("servers")}
          value={activeTab}
          options={[
            {
              value: "public",
              tabId: "servers-public-tab",
              panelId: "servers-results-panel",
              label: (
                <span className="inline-flex items-center gap-2">
                  <Globe2 size={16} aria-hidden="true" /> {t("public_servers")}
                </span>
              ),
            },
            {
              value: "favorites",
              tabId: "servers-favorites-tab",
              panelId: "servers-results-panel",
              label: (
                <span className="inline-flex items-center gap-2">
                  <Heart size={16} aria-hidden="true" /> {t("favorites")}
                  <span className="text-xs text-text-muted">{favorites.length}</span>
                </span>
              ),
            },
          ]}
          onValueChange={(value) => startTransition(() => setActiveTab(value))}
        />

        <div
          id="servers-results-panel"
          className="flex flex-col gap-5"
          role="tabpanel"
          aria-labelledby={activeTab === "public" ? "servers-public-tab" : "servers-favorites-tab"}
        >
          {activeTab === "public" && (
            <Toolbar className="flex-wrap gap-3">
              <ToolbarGroup className="min-w-[240px] flex-1">
                <SearchInput
                  wrapperClassName="w-full max-w-md"
                  aria-label={t("search_servers")}
                  placeholder={t("search_servers_placeholder")}
                  value={search}
                  onValueChange={setSearch}
                />
              </ToolbarGroup>
              <ToolbarGroup align="end">
                <Checkbox
                  label={t("show_whitelist_servers")}
                  checked={showWhitelistServers}
                  onChange={(event) => setShowWhitelistServers(event.target.checked)}
                />
                <IconButton
                  aria-label={t("refresh")}
                  disabled={publicServers.isFetching}
                  onClick={() => void publicServers.refetch()}
                >
                  <RefreshCw size={16} aria-hidden="true" />
                </IconButton>
              </ToolbarGroup>
            </Toolbar>
          )}

          {publicServers.isLoading && activeTab === "public" ? (
            <LoadingState>{t("loading_servers")}</LoadingState>
          ) : favoriteServers.isLoading && activeTab === "favorites" ? (
            <LoadingState>{t("loading_servers")}</LoadingState>
          ) : publicServers.isError && activeTab === "public" ? (
            <ErrorState
              title={t("could_not_load_servers")}
              description={errorMessage(publicServers.error)}
              action={<Button onClick={() => void publicServers.refetch()}>{t("retry")}</Button>}
            />
          ) : favoriteServers.isError && activeTab === "favorites" ? (
            <ErrorState
              title={t("could_not_load_servers")}
              description={errorMessage(favoriteServers.error)}
              action={<Button onClick={() => void favoriteServers.refetch()}>{t("retry")}</Button>}
            />
          ) : activeTab === "public" && catalog.length === 0 ? (
            <EmptyState
              title={t("no_servers_found")}
              description={t("try_changing_server_filters")}
              action={
                hasActivePublicFilters ? (
                  <Button variant="secondary" onClick={resetFilters}>
                    {t("reset")}
                  </Button>
                ) : undefined
              }
            />
          ) : activeTab === "favorites" && favorites.length === 0 ? (
            <EmptyState
              icon={<Heart size={24} aria-hidden="true" />}
              title={t("no_favorite_servers")}
              description={t("favorites_empty_description")}
              action={
                <Button onClick={() => startTransition(() => setActiveTab("public"))}>
                  {t("public_servers")}
                </Button>
              }
            />
          ) : (
            <section>
              {activeTab === "public" ? (
                <>
                  <div className="grid grid-cols-[repeat(auto-fill,minmax(min(260px,100%),1fr))] gap-4">
                    {visiblePublicServers.map((server) => {
                      const key = serverKey(server);
                      const occurrence = publicServerKeyCounts.get(key) ?? 0;
                      publicServerKeyCounts.set(key, occurrence + 1);
                      const favorite = favoriteByKey.get(key);
                      const preferredInstance = favorite?.instanceId
                        ? instanceById.get(favorite.instanceId)
                        : undefined;
                      return (
                        <ServerCard
                          key={`${key}:${occurrence}`}
                          server={server}
                          favorite={favorite}
                          preferredInstance={preferredInstance}
                          busy={joiningKey === key}
                          favoriteBusy={favoriteBusyKey === key}
                          onJoin={requestPlay}
                          onToggleFavorite={toggleFavorite}
                          onDetails={setDetailsServer}
                          onCopyAddress={(address) => void copyAddress(address)}
                          onCopyLink={(address) => void copyWaxlightLink(address)}
                        />
                      );
                    })}
                  </div>
                  {visiblePublicServers.length < catalog.length && (
                    <div className="flex justify-center pt-4">
                      <Button
                        variant="secondary"
                        onClick={() => setVisibleServerCount((count) => count + SERVER_PAGE_SIZE)}
                      >
                        {t("load_more")}
                      </Button>
                    </div>
                  )}
                </>
              ) : (
                <div className="grid grid-cols-[repeat(auto-fill,minmax(min(260px,100%),1fr))] gap-4">
                  {favorites.map((favorite) => {
                    const catalogServer = catalogByKey.get(serverKey(favorite));
                    const server = catalogServer ?? favoriteAsPublicServer(favorite);
                    const preferredInstance = favorite.instanceId
                      ? instanceById.get(favorite.instanceId)
                      : undefined;
                    return (
                      <ServerCard
                        key={favorite.id}
                        server={server}
                        favorite={favorite}
                        preferredInstance={preferredInstance}
                        favoriteBusy={favoriteBusyKey === serverKey(favorite)}
                        onJoin={requestPlay}
                        onToggleFavorite={toggleFavorite}
                        onDetails={setDetailsServer}
                        onCopyAddress={(address) => void copyAddress(address)}
                        onCopyLink={(address) => void copyWaxlightLink(address)}
                      />
                    );
                  })}
                </div>
              )}
            </section>
          )}
        </div>
      </PageContent>

      {detailedServer && (
        <ServerDetailsDialog
          server={detailedServer}
          favorite={detailsFavorite}
          preferredInstance={detailsPreferredInstance}
          detailsLoading={publicServerDetails.isLoading}
          detailsError={
            publicServerDetails.isError ? errorMessage(publicServerDetails.error) : undefined
          }
          onRetry={() => void publicServerDetails.refetch()}
          favoriteBusy={detailsFavorite ? favoriteBusyKey === serverKey(detailedServer) : false}
          onToggleFavorite={() => toggleFavorite(detailedServer, detailsFavorite)}
          onClose={() => setDetailsServer(undefined)}
          onCopyAddress={() => void copyAddress(detailedServer.address)}
          onCopyLink={() => void copyWaxlightLink(detailedServer.address)}
          onJoin={() => requestPlay(detailedServer, detailsFavorite)}
        />
      )}

      {joinTarget && (
        <ServerJoinDialog
          server={joinTarget.server}
          favorite={joinTarget.favorite}
          instances={instances}
          versions={versions}
          busy={joiningKey !== null}
          onClose={() => setJoinTarget(undefined)}
          onConfirm={(instanceId) =>
            void launchWithInstance(joinTarget.server, joinTarget.favorite, instanceId)
          }
        />
      )}
    </Page>
  );
}
