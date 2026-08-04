import type { TFunction } from "i18next";
import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";

import { InstancePickerDialog } from "../features/mods/InstancePickerDialog";
import { ModCard } from "../features/mods/ModCard";
import { ModsFilters } from "../features/mods/ModsFilters";
import {
  modCatalogApi,
  type DownloadedMod,
  type GameVersion,
  type Instance,
  type ModDetails,
  type ModSearchQuery,
  type ModSummary,
} from "../shared/api";
import { errorMessage } from "../shared/api/bridge";
import { Button, Empty, PageHeader } from "../shared/ui";

type Notify = (message: string, type?: "ok" | "error") => void;

interface ModsPageProps {
  instances: Instance[];
  versions: GameVersion[];
  notify: Notify;
}

export function ModsPage({ instances, versions, notify }: ModsPageProps) {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchText, setSearchText] = useState(searchParams.get("q") ?? "");
  const [catalog, setCatalog] = useState<ModSummary[]>([]);
  const [downloaded, setDownloaded] = useState<DownloadedMod[]>([]);
  const [total, setTotal] = useState(0);
  const [hasNext, setHasNext] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState("");
  const [reloadKey, setReloadKey] = useState(0);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [installing, setInstalling] = useState<{
    details: ModDetails;
    downloaded?: DownloadedMod;
    preferredVersionId?: string;
  }>();
  const [layout, setLayout] = useState<"grid" | "list">(() =>
    readStorage("localStorage", "waxlight.mods.layout") === "list" ? "list" : "grid",
  );
  const restoreScroll = useRef(true);

  const view = searchParams.get("view") === "downloaded" ? "downloaded" : "all";
  const instanceId = searchParams.get("instanceId") ?? "";
  const contextInstance = instances.find((item) => item.id === instanceId);
  const contextVersion = contextInstance
    ? versions.find((item) => item.id === contextInstance.gameVersionId)
    : undefined;
  const compatibleOnly = contextInstance !== undefined;
  const query = useMemo<ModSearchQuery>(
    () => ({
      text: searchParams.get("q") ?? "",
      gameVersion:
        searchParams.get("gameVersion") ?? (contextVersion?.name || contextVersion?.id || ""),
      side: modSide(searchParams.get("side")),
      updatedAfter: searchParams.get("updatedAfter") ?? undefined,
      tags: searchParams.getAll("tag"),
      compatibleOnly: searchParams.get("compatible") === "1" || compatibleOnly,
      instanceId,
      sort: modSort(searchParams.get("sort")),
      page: Math.max(1, Number(searchParams.get("page") ?? "1")),
      pageSize: 24,
    }),
    [compatibleOnly, contextVersion?.id, contextVersion?.name, instanceId, searchParams],
  );
  const page = query.page;

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const current = searchParams.get("q") ?? "";
      if (searchText === current) return;
      const next = new URLSearchParams(searchParams);
      if (searchText) next.set("q", searchText);
      else next.delete("q");
      next.set("page", "1");
      setSearchParams(next, { replace: true });
    }, 400);
    return () => window.clearTimeout(timer);
  }, [searchParams, searchText, setSearchParams]);

  useEffect(() => {
    let active = true;
    async function loadMods() {
      if (view === "downloaded") {
        setLoading(true);
        try {
          const items = await modCatalogApi.downloaded();
          if (!active) return;
          setDownloaded(items ?? []);
          setError("");
          for (const item of items ?? []) {
            void checkForUpdates(item);
          }
        } catch (loadError) {
          if (active) setError(errorMessage(loadError));
        } finally {
          if (active) setLoading(false);
        }
        return;
      }

      if (page === 1) setLoading(true);
      else setLoadingMore(true);
      try {
        const result = await modCatalogApi.search(query);
        if (!active) return;
        setCatalog((items) =>
          page === 1 ? (result.items ?? []) : mergeMods(items, result.items ?? []),
        );
        setTotal(result.totalItems);
        setHasNext(result.hasNext);
        setError("");
      } catch (loadError) {
        if (active) setError(errorMessage(loadError));
      } finally {
        if (active) {
          setLoading(false);
          setLoadingMore(false);
        }
      }
    }

    async function checkForUpdates(item: DownloadedMod) {
      try {
        const updated = await modCatalogApi.checkUpdates(item.modId);
        if (active) setDownloaded(updated ?? []);
      } catch {
        // Update availability is optional.
      }
    }

    void loadMods();
    return () => {
      active = false;
    };
  }, [page, query, reloadKey, view]);

  useEffect(() => {
    let active = true;

    async function loadDownloadedState() {
      if (view !== "all") return;

      try {
        const items = (await modCatalogApi.downloaded()) ?? [];
        if (!active) return;

        setDownloaded(items);
        setCatalog((current) => synchronizeDownloadedState(current, items));
      } catch {
        // Catalog browsing remains available if the local download index cannot be read.
      }
    }

    void loadDownloadedState();

    return () => {
      active = false;
    };
  }, [reloadKey, view]);

  useEffect(() => {
    if (!loading && restoreScroll.current) {
      restoreScroll.current = false;
      const stored = readStorage("sessionStorage", `waxlight.mods.scroll:${location.search}`);
      if (stored) window.scrollTo({ top: Number(stored) });
    }
  }, [loading, location.search]);

  const filteredDownloaded = useMemo(() => {
    let result = downloaded.filter((item) => {
      const text = query.text.toLowerCase();
      if (text && !`${item.name} ${item.authorName} ${item.modId}`.toLowerCase().includes(text)) {
        return false;
      }
      if (query.side && item.side !== query.side) return false;
      if (
        query.gameVersion &&
        !item.gameVersions.some((version) =>
          version.startsWith(query.gameVersion.replace(/x$/, "")),
        )
      ) {
        return false;
      }
      return true;
    });
    const sorted = [...result];
    Array.prototype.sort.call(sorted, (left, right) => {
      if (query.sort === "name_asc") return left.name.localeCompare(right.name);
      if (query.sort === "name_desc") return right.name.localeCompare(left.name);
      return new Date(right.downloadedAt).getTime() - new Date(left.downloadedAt).getTime();
    });
    return sorted;
  }, [downloaded, query.gameVersion, query.side, query.sort, query.text]);

  function updateParams(values: Record<string, string | undefined>, replace = false) {
    const next = new URLSearchParams(searchParams);
    for (const [key, value] of Object.entries(values)) {
      if (!value) next.delete(key);
      else next.set(key, value);
    }
    setSearchParams(next, { replace });
  }

  function patchFilters(patch: Partial<ModSearchQuery>) {
    const next: Record<string, string | undefined> = { page: "1" };
    if ("gameVersion" in patch) next.gameVersion = patch.gameVersion || undefined;
    if ("side" in patch) next.side = patch.side || undefined;
    if ("updatedAfter" in patch) next.updatedAfter = patch.updatedAfter || undefined;
    if ("sort" in patch) next.sort = patch.sort;
    updateParams(next);
  }

  function clearFilters() {
    const next = new URLSearchParams(searchParams);
    ["gameVersion", "side", "updatedAfter", "tag", "compatible", "page"].forEach((key) =>
      next.delete(key),
    );
    setSearchParams(next);
  }

  async function openInstaller(modId: string, local?: DownloadedMod) {
    try {
      const details = await modCatalogApi.get(modId);
      setInstalling({
        details,
        downloaded: local,
        preferredVersionId: local?.updateAvailable ? details.versions[0]?.id : local?.versionId,
      });
    } catch (loadError) {
      notify(errorMessage(loadError), "error");
    }
  }

  function openDetails(modId: string) {
    writeStorage(
      "sessionStorage",
      `waxlight.mods.scroll:${location.search}`,
      String(window.scrollY),
    );
    void navigate(`/mods/${encodeURIComponent(modId)}?from=${encodeURIComponent(location.search)}`);
  }

  async function refreshDownloaded() {
    const items = (await modCatalogApi.downloaded()) ?? [];
    setDownloaded(items);
    setCatalog((current) => synchronizeDownloadedState(current, items));
  }

  const displayed =
    view === "all" ? catalog : filteredDownloaded.map((mod) => downloadedAsSummary(mod, t));

  return (
    <>
      <PageHeader
        eyebrow={t("mod_browser")}
        title={t("mods")}
        description={t("mods_description")}
        action={
          <div className="modsSearch">
            <span>⌕</span>
            <input
              aria-label={t("search_mods")}
              placeholder={t("search_mods_placeholder")}
              value={searchText}
              onChange={(event) => setSearchText(event.target.value)}
            />
            {searchText && (
              <button aria-label={t("clear_search")} onClick={() => setSearchText("")}>
                ×
              </button>
            )}
          </div>
        }
      />

      {contextInstance && (
        <div className="instanceContext">
          <span>{t("browsing_for")}</span>
          <strong>{contextInstance.name}</strong>
          <span>· Vintage Story {contextVersion?.name ?? contextInstance.gameVersionId}</span>
          <button onClick={() => updateParams({ instanceId: undefined, compatible: undefined })}>
            {t("clear_instance_context")}
          </button>
        </div>
      )}

      <div className="modsTabs" role="tablist">
        <button
          role="tab"
          aria-selected={view === "all"}
          className={view === "all" ? "active" : ""}
          onClick={() => updateParams({ view: "all", page: "1" })}
        >
          {t("all_mods")}
        </button>
        <button
          role="tab"
          aria-selected={view === "downloaded"}
          className={view === "downloaded" ? "active" : ""}
          onClick={() => updateParams({ view: "downloaded", page: "1" })}
        >
          {t("downloaded")} <b>{downloaded.length || ""}</b>
        </button>
      </div>

      <ModsFilters
        query={query}
        versions={versions}
        mobileOpen={filtersOpen}
        onMobileOpenChange={setFiltersOpen}
        onChange={patchFilters}
        onClear={clearFilters}
      />

      <div className="modsResultsHeader">
        <span>
          {view === "downloaded"
            ? t("downloaded_count", { count: filteredDownloaded.length })
            : t("mods_count", { count: total })}
        </span>
        <div className="viewToggle" aria-label={t("results_layout")}>
          <button
            className={layout === "grid" ? "active" : ""}
            aria-label={t("grid_view")}
            onClick={() => {
              setLayout("grid");
              writeStorage("localStorage", "waxlight.mods.layout", "grid");
            }}
          >
            ▦
          </button>
          <button
            className={layout === "list" ? "active" : ""}
            aria-label={t("list_view")}
            onClick={() => {
              setLayout("list");
              writeStorage("localStorage", "waxlight.mods.layout", "list");
            }}
          >
            ☷
          </button>
        </div>
      </div>

      {loading ? (
        <div className={`modGrid modGrid-${layout} modSkeletonGrid`} aria-label={t("loading_mods")}>
          {Array.from({ length: 8 }, (_, index) => (
            <i key={index} />
          ))}
        </div>
      ) : error ? (
        <Empty
          icon="!"
          title={t("could_not_load_mods")}
          description={error}
          action={
            <Button
              onClick={() =>
                view === "all" ? setReloadKey((value) => value + 1) : void refreshDownloaded()
              }
            >
              {t("retry")}
            </Button>
          }
        />
      ) : displayed.length === 0 ? (
        <Empty
          icon="◇"
          title={view === "downloaded" ? t("no_downloaded_mods") : t("no_mods_found")}
          description={
            view === "downloaded"
              ? t("downloaded_mods_empty_description")
              : t("try_changing_mod_filters")
          }
          action={
            view === "downloaded" ? (
              <Button onClick={() => updateParams({ view: "all" })}>{t("browse_mods")}</Button>
            ) : (
              <Button onClick={clearFilters}>{t("clear_filters")}</Button>
            )
          }
        />
      ) : (
        <div className={`modGrid modGrid-${layout}`}>
          {displayed.map((mod) => {
            const local =
              view === "downloaded"
                ? filteredDownloaded.find((item) => item.modId === mod.id)
                : undefined;
            return (
              <ModCard
                key={`${mod.id}:${local?.versionId ?? "catalog"}`}
                mod={mod}
                downloaded={local}
                layout={layout}
                onOpen={() => openDetails(mod.id)}
                onInstall={() => void openInstaller(mod.id, local)}
                onDelete={
                  local
                    ? async () => {
                        const warning =
                          local.installedInstances.length > 0
                            ? t("delete_cached_installed_mod_confirmation", {
                                count: local.installedInstances.length,
                              })
                            : t("delete_cached_mod_confirmation");
                        if (!window.confirm(warning)) return;
                        try {
                          await modCatalogApi.removeDownloaded(local.modId, local.versionId);
                          await refreshDownloaded();
                          notify(t("downloaded_mod_removed"));
                        } catch (removeError) {
                          notify(errorMessage(removeError), "error");
                        }
                      }
                    : undefined
                }
              />
            );
          })}
        </div>
      )}

      {view === "all" && hasNext && !loading && (
        <div className="loadMore">
          <Button
            variant="secondary"
            busy={loadingMore}
            onClick={() => updateParams({ page: String(page + 1) })}
          >
            {t("load_more")}
          </Button>
        </div>
      )}

      {installing && (
        <InstancePickerDialog
          mod={installing.details}
          downloaded={installing.downloaded}
          instances={instances}
          gameVersions={versions}
          preferredInstanceId={instanceId}
          preferredVersionId={installing.preferredVersionId}
          onClose={() => setInstalling(undefined)}
          onDone={async () => {
            await refreshDownloaded();
            notify(t("mod_task_completed"));
          }}
        />
      )}
    </>
  );
}

function synchronizeDownloadedState(
  catalog: ModSummary[],
  downloaded: DownloadedMod[],
): ModSummary[] {
  const newestByModID = new Map<string, DownloadedMod>();
  for (const item of downloaded) {
    const current = newestByModID.get(item.modId);
    if (
      !current ||
      new Date(item.downloadedAt).getTime() > new Date(current.downloadedAt).getTime()
    ) {
      newestByModID.set(item.modId, item);
    }
  }

  return catalog.map((item) => {
    const local = newestByModID.get(item.id);
    return {
      ...item,
      isDownloaded: local !== undefined,
      isInstalled: (local?.installedInstances.length ?? 0) > 0,
      updateAvailable: local?.updateAvailable ?? false,
    };
  });
}

function mergeMods(current: ModSummary[], next: ModSummary[]): ModSummary[] {
  const byId = new Map(current.map((item) => [item.id, item]));
  next.forEach((item) => byId.set(item.id, item));
  return [...byId.values()];
}

function downloadedAsSummary(mod: DownloadedMod, t: TFunction): ModSummary {
  return {
    id: mod.modId,
    slug: mod.slug,
    name: mod.name,
    authorName: mod.authorName,
    summary: t("downloaded_version", { version: mod.downloadedVersion }),
    imageUrl: mod.imageUrl,
    side: mod.side,
    latestVersion: mod.latestVersion,
    gameVersions: mod.gameVersions,
    downloads: 0,
    updatedAt: mod.downloadedAt,
    tags: [],
    isDownloaded: true,
    isInstalled: mod.installedInstances.length > 0,
    updateAvailable: mod.updateAvailable,
  };
}

function readStorage(storageName: "localStorage" | "sessionStorage", key: string): string | null {
  try {
    return globalThis[storageName]?.getItem(key) ?? null;
  } catch {
    return null;
  }
}

function writeStorage(storageName: "localStorage" | "sessionStorage", key: string, value: string) {
  try {
    globalThis[storageName]?.setItem(key, value);
  } catch {
    // Preference persistence is optional in restricted webviews.
  }
}

function modSide(value: string | null): ModSearchQuery["side"] {
  switch (value) {
    case "client":
    case "server":
    case "both":
    case "unknown":
      return value;
    default:
      return "";
  }
}

function modSort(value: string | null): ModSearchQuery["sort"] {
  switch (value) {
    case "relevance":
    case "updated":
    case "newest":
    case "downloads":
    case "name_asc":
    case "name_desc":
      return value;
    default:
      return "updated";
  }
}
