import { useQueryClient } from "@tanstack/react-query";
import {
  Boxes,
  Gamepad2,
  Globe2,
  Heart,
  LockKeyhole,
  RotateCcw,
  Search,
  ShieldCheck,
  Users,
} from "lucide-react";
import {
  memo,
  startTransition,
  useCallback,
  useDeferredValue,
  useEffect,
  useState,
  type MouseEvent,
} from "react";
import { useTranslation } from "react-i18next";

import { useToastStore } from "../../app/stores/toast";
import { useInstancesQuery } from "../../entities/instance/queries";
import { serversApi } from "../../entities/server/api";
import type { FavoriteServer, PublicServer } from "../../entities/server/model";
import { useFavoriteServersQuery, usePublicServersQuery } from "../../entities/server/queries";
import { errorMessage } from "../../shared/api/bridge";
import { FAVORITE_SERVERS_QUERY_KEY } from "../../shared/api/keys";
import { launcherApi } from "../../shared/api/launcher";
import { Button } from "../../shared/ui/button";
import { Checkbox } from "../../shared/ui/checkbox-control";
import { Empty } from "../../shared/ui/empty";
import { Modal } from "../../shared/ui/modal";
import { PageHeader } from "../../shared/ui/page-header";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../shared/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "../../shared/ui/tooltip";

type ServerTab = "favorites" | "public";

const SERVER_PAGE_SIZE = 60;

function serverKey(server: { name: string; address: string }) {
  return `${server.address}\u0000${server.name}`;
}

function canRequestServerLaunch(server: PublicServer) {
  return server.joinable || (server.accessRestricted && !server.address);
}

function PlayButton({
  blockedByWhitelist,
  disabled,
  onClick,
}: {
  blockedByWhitelist: boolean;
  disabled: boolean;
  onClick: (event: MouseEvent<HTMLButtonElement>) => void;
}) {
  const { t } = useTranslation();
  const button = (
    <Button disabled={disabled || blockedByWhitelist} onClick={onClick}>
      {t("play")}
    </Button>
  );
  if (!blockedByWhitelist) return button;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="serverDisabledPlay">{button}</span>
      </TooltipTrigger>
      <TooltipContent>{t("server_whitelist_launch_unavailable")}</TooltipContent>
    </Tooltip>
  );
}

const PublicServerCard = memo(function PublicServerCard({
  server,
  favorite,
  busy,
  onToggleFavorite,
  onPlay,
  onDetails,
}: {
  server: PublicServer;
  favorite?: FavoriteServer;
  busy: boolean;
  onToggleFavorite: (server: PublicServer, favorite?: FavoriteServer) => void;
  onPlay: (server: PublicServer, favorite?: FavoriteServer) => void;
  onDetails: (server: PublicServer) => void;
}) {
  const { t } = useTranslation();
  return (
    <article className="serverCard">
      <button
        className="serverCardDetailsButton"
        aria-label={server.name}
        onClick={() => onDetails(server)}
      />
      <div className="serverCardTopline">
        <span className="serverOnline">{t("server_players", { count: server.players })}</span>
      </div>
      <h3>{server.name}</h3>
      <p className="serverAddress">{server.address || t("server_address")}</p>
      <p className="serverDescription">{server.description || t("no_description_provided")}</p>
      <div className="serverTags">
        {server.modCount > 0 && (
          <span className="serverTag modded">{t("server_mods", { count: server.modCount })}</span>
        )}
        {server.requiresWhitelist && <span className="serverTag">{t("server_whitelist")}</span>}
        {server.accessRestricted && <span className="serverTag">{t("server_password")}</span>}
      </div>
      <ServerStats server={server} />
      <div className="serverCardActions">
        <Button
          aria-label={favorite ? t("remove") : t("add_favorite")}
          variant="ghost"
          className="serverHeartButton"
          busy={busy}
          onClick={(event) => {
            event.stopPropagation();
            onToggleFavorite(server, favorite);
          }}
        >
          <Heart className={favorite ? "saved" : ""} size={18} />
        </Button>
        <PlayButton
          blockedByWhitelist={server.requiresWhitelist}
          disabled={!canRequestServerLaunch(server)}
          onClick={(event) => {
            event.stopPropagation();
            onPlay(server, favorite);
          }}
        />
      </div>
    </article>
  );
});

function ServerStats({ server }: { server: PublicServer }) {
  const { t } = useTranslation();
  return (
    <div className="serverStats">
      <span>
        <Users size={14} /> {server.players}
      </span>
      {server.modCount > 0 && (
        <span>
          <Boxes size={14} /> {server.modCount}
        </span>
      )}
      {server.requiresWhitelist && (
        <span>
          <ShieldCheck size={14} /> {t("server_whitelist")}
        </span>
      )}
      {server.accessRestricted && (
        <span>
          <LockKeyhole size={14} /> {t("server_password")}
        </span>
      )}
    </div>
  );
}

const FavoriteServerCard = memo(function FavoriteServerCard({
  server,
  catalogServer,
  instanceName,
  onRemove,
  onPlay,
  onDetails,
}: {
  server: FavoriteServer;
  catalogServer?: PublicServer;
  instanceName?: string;
  onRemove: (id: string) => void;
  onPlay: (server: PublicServer, favorite?: FavoriteServer) => void;
  onDetails: (server: PublicServer) => void;
}) {
  const { t } = useTranslation();
  return (
    <article className="serverCard">
      <button
        className="serverCardDetailsButton"
        aria-label={server.name}
        onClick={() =>
          onDetails(
            catalogServer ?? {
              name: server.name,
              address: server.address,
              description: "",
              players: 0,
              modCount: 0,
              requiresWhitelist: false,
              accessRestricted: false,
              joinable: true,
            },
          )
        }
      />
      <div className="serverCardTopline">
        {catalogServer ? (
          <span className="serverOnline">
            {t("server_players", { count: catalogServer.players })}
          </span>
        ) : (
          <span className="serverOffline">{t("favorite")}</span>
        )}
      </div>
      <h3>{server.name}</h3>
      <p className="serverAddress">{server.address}</p>
      <p className="serverDescription">
        {catalogServer?.description || instanceName || t("no_linked_instance")}
      </p>
      {catalogServer ? (
        <ServerStats server={catalogServer} />
      ) : (
        <div className="serverStats">
          <span>
            <Gamepad2 size={14} /> {instanceName || t("no_linked_instance")}
          </span>
        </div>
      )}
      <div className="serverCardActions">
        <Button
          aria-label={t("remove")}
          variant="ghost"
          className="serverHeartButton"
          onClick={(event) => {
            event.stopPropagation();
            onRemove(server.id);
          }}
        >
          <Heart className="saved" size={18} />
        </Button>
        <PlayButton
          blockedByWhitelist={Boolean(catalogServer?.requiresWhitelist)}
          disabled={false}
          onClick={(event) => {
            event.stopPropagation();
            onPlay(
              catalogServer ?? {
                name: server.name,
                address: server.address,
                description: "",
                players: 0,
                modCount: 0,
                requiresWhitelist: false,
                accessRestricted: false,
                joinable: true,
              },
              server,
            );
          }}
        />
      </div>
    </article>
  );
});

export function ServersPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  const { data: favorites = [] } = useFavoriteServersQuery();
  const publicServers = usePublicServersQuery();
  const { data: instances = [] } = useInstancesQuery();
  const [activeTab, setActiveTab] = useState<ServerTab>("public");
  const [busy, setBusy] = useState(false);
  const [search, setSearch] = useState("");
  const [showWhitelistServers, setShowWhitelistServers] = useState(false);
  const [visibleServerCount, setVisibleServerCount] = useState(SERVER_PAGE_SIZE);
  const [detailsServer, setDetailsServer] = useState<PublicServer>();
  const [launchServer, setLaunchServer] = useState<{
    server: PublicServer;
    favorite?: FavoriteServer;
  }>();
  const [launchInstanceID, setLaunchInstanceID] = useState("");
  const deferredSearch = useDeferredValue(search);

  const refreshFavorites = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: FAVORITE_SERVERS_QUERY_KEY });
  }, [queryClient]);

  const savePublicServer = useCallback(
    async (server: PublicServer) => {
      setBusy(true);
      try {
        await serversApi.saveFavorite({ id: "", name: server.name, address: server.address });
        await refreshFavorites();
        notify(t("server_saved_to_favorites"));
      } catch (error) {
        notify(errorMessage(error), "error");
      } finally {
        setBusy(false);
      }
    },
    [notify, refreshFavorites, t],
  );

  const removeFavorite = useCallback(
    async (id: string) => {
      try {
        await serversApi.removeFavorite(id);
        await refreshFavorites();
      } catch (error) {
        notify(errorMessage(error), "error");
      }
    },
    [notify, refreshFavorites],
  );

  const togglePublicFavorite = useCallback(
    (server: PublicServer, favorite?: FavoriteServer) => {
      if (favorite) {
        void removeFavorite(favorite.id);
        return;
      }
      void savePublicServer(server);
    },
    [removeFavorite, savePublicServer],
  );

  const launchSelectedServer = useCallback(async () => {
    if (!launchServer || !launchInstanceID) return;
    setBusy(true);
    try {
      const address = launchServer.server.address.trim();
      if (address) {
        await launcherApi.launchServer(launchInstanceID, address);
        await serversApi.saveFavorite({
          id: launchServer.favorite?.id ?? "",
          name: launchServer.server.name,
          address,
          instanceId: launchInstanceID,
        });
        await refreshFavorites();
      } else {
        await launcherApi.launch(launchInstanceID);
      }
      setLaunchServer(undefined);
      notify(t("server_opened_in_game"));
    } catch (error) {
      notify(errorMessage(error), "error");
    } finally {
      setBusy(false);
    }
  }, [launchInstanceID, launchServer, notify, refreshFavorites, t]);

  const requestPlay = useCallback(
    (server: PublicServer, favorite?: FavoriteServer) => {
      setDetailsServer(undefined);
      setLaunchServer({ server, favorite });
      setLaunchInstanceID(favorite?.instanceId ?? instances[0]?.id ?? "");
    },
    [instances],
  );

  const catalogByKey = new Map(
    (publicServers.data ?? []).map((server) => [serverKey(server), server]),
  );
  const instanceNames = new Map(instances.map((instance) => [instance.id, instance.name]));
  const favoriteByKey = new Map(favorites.map((server) => [serverKey(server), server]));
  const normalizedSearch = deferredSearch.trim().toLocaleLowerCase();
  const catalog = (publicServers.data ?? []).filter((server) => {
    if (server.requiresWhitelist && !showWhitelistServers) return false;
    return (
      !normalizedSearch ||
      `${server.name} ${server.address} ${server.description}`
        .toLocaleLowerCase()
        .includes(normalizedSearch)
    );
  });
  const featuredServer = activeTab === "public" ? catalog[0] : undefined;
  const publicGridServers = catalog.slice(featuredServer ? 1 : 0);
  const visiblePublicGridServers = publicGridServers.slice(0, visibleServerCount);

  useEffect(() => {
    setVisibleServerCount(SERVER_PAGE_SIZE);
  }, [deferredSearch, showWhitelistServers]);

  function resetFilters() {
    setSearch("");
    setShowWhitelistServers(false);
  }

  return (
    <>
      <PageHeader title={t("servers")} description={t("servers_description")} />

      <div className="serverTabs" role="tablist" aria-label={t("servers")}>
        <button
          className={activeTab === "public" ? "active" : ""}
          role="tab"
          aria-selected={activeTab === "public"}
          onClick={() => startTransition(() => setActiveTab("public"))}
        >
          <Globe2 size={19} /> {t("public_servers")}
        </button>
        <button
          className={activeTab === "favorites" ? "active" : ""}
          role="tab"
          aria-selected={activeTab === "favorites"}
          onClick={() => startTransition(() => setActiveTab("favorites"))}
        >
          <Heart size={19} fill="currentColor" /> {t("favorites")}
          <span>{favorites.length}</span>
        </button>
      </div>

      {activeTab === "public" && (
        <section className="serversToolbar">
          <label className="serversSearch">
            <Search size={20} />
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t("search_servers_placeholder")}
              aria-label={t("search_servers")}
            />
          </label>
          <Checkbox
            className="serversWhitelistFilter"
            label={t("show_whitelist_servers")}
            checked={showWhitelistServers}
            onChange={(event) => setShowWhitelistServers(event.target.checked)}
          />
          <Button variant="ghost" className="serversReset" onClick={resetFilters}>
            <RotateCcw size={16} /> {t("reset")}
          </Button>
        </section>
      )}

      {publicServers.isError && activeTab === "public" ? (
        <Empty
          icon="!"
          title={t("could_not_load_servers")}
          description={errorMessage(publicServers.error)}
          action={<Button onClick={() => void publicServers.refetch()}>{t("retry")}</Button>}
        />
      ) : activeTab === "public" && catalog.length === 0 && !publicServers.isLoading ? (
        <Empty
          icon="⌕"
          title={t("no_servers_found")}
          description={t("try_changing_server_filters")}
        />
      ) : activeTab === "favorites" && favorites.length === 0 ? (
        <Empty
          icon="☆"
          title={t("no_favorite_servers")}
          description={t("servers_description")}
          action={
            <Button onClick={() => startTransition(() => setActiveTab("public"))}>
              {t("public_servers")}
            </Button>
          }
        />
      ) : (
        <section className="serversCollection">
          {featuredServer && (
            <article className="serverFeatured">
              <button
                className="serverFeaturedDetailsButton"
                aria-label={featuredServer.name}
                onClick={() => setDetailsServer(featuredServer)}
              />
              <div className="serverFeaturedArt">
                <span>{featuredServer.name.slice(0, 1)}</span>
              </div>
              <div className="serverFeaturedContent">
                <div className="serverCardTopline">
                  <span className="serverOnline">
                    {t("server_players", { count: featuredServer.players })}
                  </span>
                </div>
                <h2>{featuredServer.name}</h2>
                <p>{featuredServer.description || t("no_description_provided")}</p>
                <div className="serverTags">
                  {featuredServer.modCount > 0 && (
                    <span className="serverTag modded">
                      {t("server_mods", { count: featuredServer.modCount })}
                    </span>
                  )}
                </div>
                <ServerStats server={featuredServer} />
              </div>
              <div className="serverFeaturedActions">
                <Button
                  variant="ghost"
                  className="serverHeartButton"
                  onClick={() =>
                    togglePublicFavorite(
                      featuredServer,
                      favoriteByKey.get(serverKey(featuredServer)),
                    )
                  }
                >
                  <Heart
                    className={favoriteByKey.has(serverKey(featuredServer)) ? "saved" : ""}
                    size={20}
                  />
                </Button>
                <PlayButton
                  blockedByWhitelist={featuredServer.requiresWhitelist}
                  disabled={!canRequestServerLaunch(featuredServer)}
                  onClick={() =>
                    requestPlay(featuredServer, favoriteByKey.get(serverKey(featuredServer)))
                  }
                />
              </div>
            </article>
          )}
          {activeTab === "public" ? (
            <>
              <div className="serversGrid" key="public-servers">
                {visiblePublicGridServers.map((server) => (
                  <PublicServerCard
                    key={`${server.name}:${server.address}`}
                    server={server}
                    favorite={favoriteByKey.get(serverKey(server))}
                    busy={busy}
                    onToggleFavorite={togglePublicFavorite}
                    onPlay={requestPlay}
                    onDetails={setDetailsServer}
                  />
                ))}
              </div>
              {visiblePublicGridServers.length < publicGridServers.length && (
                <div className="serversLoadMore">
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
            <div className="serversGrid" key="favorite-servers">
              {favorites.map((server) => (
                <FavoriteServerCard
                  key={server.id}
                  server={server}
                  catalogServer={catalogByKey.get(serverKey(server))}
                  instanceName={
                    server.instanceId ? instanceNames.get(server.instanceId) : undefined
                  }
                  onRemove={removeFavorite}
                  onPlay={requestPlay}
                  onDetails={setDetailsServer}
                />
              ))}
            </div>
          )}
        </section>
      )}

      {detailsServer && (
        <Modal
          title={detailsServer.name}
          className="serverDetailsDialog"
          onClose={() => setDetailsServer(undefined)}
        >
          <div className="modalBody serverDetails">
            <p className="serverDetailsAddress">{detailsServer.address}</p>
            <ServerStats server={detailsServer} />
            <div className="serverTags">
              {detailsServer.modCount > 0 && (
                <span className="serverTag modded">
                  {t("server_mods", { count: detailsServer.modCount })}
                </span>
              )}
              {detailsServer.requiresWhitelist && (
                <span className="serverTag">{t("server_whitelist")}</span>
              )}
              {detailsServer.accessRestricted && (
                <span className="serverTag">{t("server_password")}</span>
              )}
            </div>
            <h3>{t("full_description")}</h3>
            <p className="serverDetailsDescription">
              {detailsServer.description || t("no_description_provided")}
            </p>
          </div>
          <div className="dialogFooter">
            <Button variant="ghost" onClick={() => setDetailsServer(undefined)}>
              {t("close")}
            </Button>
            <PlayButton
              blockedByWhitelist={detailsServer.requiresWhitelist}
              disabled={!canRequestServerLaunch(detailsServer)}
              onClick={() =>
                requestPlay(detailsServer, favoriteByKey.get(serverKey(detailsServer)))
              }
            />
          </div>
        </Modal>
      )}

      {launchServer && (
        <Modal title={t("play")} onClose={() => setLaunchServer(undefined)}>
          <div className="modalBody formFields">
            <p className="serverLaunchName">{launchServer.server.name}</p>
            {launchServer.server.address && (
              <p className="serverDetailsAddress">{launchServer.server.address}</p>
            )}
            {launchServer.server.accessRestricted && (
              <p className="serverPasswordNotice">{t("server_password_enter_in_game")}</p>
            )}
            <label className="field">
              <span>{t("linked_instance")}</span>
              <Select value={launchInstanceID} onValueChange={setLaunchInstanceID}>
                <SelectTrigger>
                  <SelectValue placeholder={t("no_instances_available")} />
                </SelectTrigger>
                <SelectContent>
                  {instances.map((instance) => (
                    <SelectItem key={instance.id} value={instance.id}>
                      {instance.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </label>
          </div>
          <div className="dialogFooter">
            <Button variant="ghost" onClick={() => setLaunchServer(undefined)}>
              {t("cancel")}
            </Button>
            <Button
              disabled={!launchInstanceID}
              busy={busy}
              onClick={() => void launchSelectedServer()}
            >
              {t("play")}
            </Button>
          </div>
        </Modal>
      )}
    </>
  );
}
