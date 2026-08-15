import { useQueryClient } from "@tanstack/react-query";
import type { TFunction } from "i18next";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate, useSearchParams } from "react-router";

import { useModSelectionStore } from "../../app/stores/mod-selection";
import { useToastStore } from "../../app/stores/toast";
import { useAccountsQuery } from "../../entities/account/queries";
import {
  useAvailableGameVersionsQuery,
  useGameVersionsQuery,
} from "../../entities/game-version/queries";
import { useInstancesQuery } from "../../entities/instance/queries";
import { modCatalogApi } from "../../entities/mod/api";
import type {
  DownloadedModCleanupResult,
  DownloadedMod,
  ModDetails,
  ModSearchQuery,
  ModSummary,
} from "../../entities/mod/model";
import {
  useDownloadedModsQuery,
  useModCatalogQuery,
  useModTagsQuery,
} from "../../entities/mod/queries";
import { settingsApi } from "../../entities/settings/api";
import { useSettingsQuery } from "../../entities/settings/queries";
import { BatchInstancePickerDialog } from "../../features/mods/BatchInstancePickerDialog";
import { InstancePickerDialog } from "../../features/mods/InstancePickerDialog";
import {
  gameVersionSeries,
  gameVersionSeriesOf,
  matchesGameVersionSeries,
  chooseRelease,
} from "../../features/mods/lib";
import { ModCard } from "../../features/mods/ModCard";
import { ModsFilters } from "../../features/mods/ModsFilters";
import { errorMessage } from "../../shared/api/bridge";
import {
  DOWNLOADED_MODS_QUERY_KEY,
  INSTANCES_QUERY_KEY,
  MOD_TAGS_QUERY_KEY,
} from "../../shared/api/keys";
import { Button } from "../../shared/ui/button";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { Empty } from "../../shared/ui/empty";
import { PageHeader } from "../../shared/ui/page-header";

const EMPTY_DOWNLOADED_MODS: DownloadedMod[] = [];

export function ModsPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  const { data: settings } = useSettingsQuery();
  const { data: instances = [] } = useInstancesQuery();
  const { data: accounts = [] } = useAccountsQuery();
  const { data: versions = [] } = useGameVersionsQuery();
  const { data: availableVersions = [] } = useAvailableGameVersionsQuery();
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchText, setSearchText] = useState(searchParams.get("q") ?? "");
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [installing, setInstalling] = useState<{
    details: ModDetails;
    downloaded?: DownloadedMod;
    preferredVersionId?: string;
  }>();
  const [openingModId, setOpeningModId] = useState("");
  const selectedModIds = useModSelectionStore((state) => state.selectedModIds);
  const setSelectedMod = useModSelectionStore((state) => state.setSelected);
  const clearSelectedMods = useModSelectionStore((state) => state.clear);
  const [batchMods, setBatchMods] =
    useState<
      { details: ModDetails; release: ModDetails["versions"][number]; downloaded?: DownloadedMod }[]
    >();
  const [openingBatch, setOpeningBatch] = useState(false);
  const [layout, setLayout] = useState<"grid" | "list">(() =>
    readStorage("localStorage", "waxlight.mods.layout") === "list" ? "list" : "grid",
  );
  const restoreScroll = useRef(true);
  const [deleteConfirm, setDeleteConfirm] = useState<{
    open: boolean;
    title: string;
    message?: string;
    onConfirm: () => void;
  }>({ open: false, title: "", onConfirm: () => {} });
  const [cleanupPreview, setCleanupPreview] = useState<DownloadedModCleanupResult>();
  const [cleaning, setCleaning] = useState(false);

  const view = searchParams.get("view") === "downloaded" ? "downloaded" : "all";
  const instanceId = searchParams.get("instanceId") ?? "";
  const contextInstance = instances.find((item) => item.id === instanceId);
  const contextVersion = contextInstance
    ? versions.find((item) => item.id === contextInstance.gameVersionId)
    : undefined;
  const compatibleOnly = contextInstance !== undefined;
  const versionSeries = useMemo(() => {
    const series = gameVersionSeries(availableVersions);
    return series.length > 0 ? series : gameVersionSeries(versions);
  }, [availableVersions, versions]);
  const query = useMemo<Omit<ModSearchQuery, "page">>(
    () => ({
      text: searchParams.get("q") ?? "",
      gameVersion:
        searchParams.get("gameVersion") ??
        (contextInstance ? gameVersionSeriesOf(contextInstance.gameVersionId) : ""),
      side: modSide(searchParams.get("side")),
      updatedAfter: searchParams.get("updatedAfter") ?? undefined,
      tags: searchParams.getAll("tag"),
      compatibleOnly: searchParams.get("compatible") === "1" || compatibleOnly,
      instanceId,
      sort: modSort(searchParams.get("sort")),
      pageSize: 24,
    }),
    [compatibleOnly, contextInstance, instanceId, searchParams],
  );

  const searchQuery = useModCatalogQuery(query, view === "all");

  const downloadedQuery = useDownloadedModsQuery();
  const downloaded = downloadedQuery.data ?? EMPTY_DOWNLOADED_MODS;

  const tagsQuery = useModTagsQuery(true);
  const tags = tagsQuery.data ?? [];

  const catalog = useMemo(
    () =>
      searchQuery.data?.pages.reduce<ModSummary[]>(
        (all, page) => mergeMods(all, page.items ?? []),
        [],
      ) ?? [],
    [searchQuery.data],
  );
  const total = searchQuery.data?.pages[0]?.totalItems ?? 0;
  const hasNext = searchQuery.hasNextPage;
  const loading = view === "all" ? searchQuery.isPending : downloadedQuery.isPending;
  const loadingMore = searchQuery.isFetchingNextPage;
  const error =
    view === "all"
      ? searchQuery.error
        ? errorMessage(searchQuery.error)
        : ""
      : downloadedQuery.error
        ? errorMessage(downloadedQuery.error)
        : "";

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const current = searchParams.get("q") ?? "";
      if (searchText === current) return;
      const next = new URLSearchParams(searchParams);
      if (searchText) next.set("q", searchText);
      else next.delete("q");
      setSearchParams(next, { replace: true });
    }, 400);
    return () => window.clearTimeout(timer);
  }, [searchParams, searchText, setSearchParams]);

  const checkedModIDs = useRef(new Set<string>());

  useEffect(() => {
    checkedModIDs.current.clear();
  }, [view]);

  useEffect(() => {
    if (view !== "downloaded") return;
    for (const item of downloadedQuery.data ?? []) {
      if (checkedModIDs.current.has(item.modId)) continue;
      checkedModIDs.current.add(item.modId);
      void modCatalogApi
        .checkUpdates(item.modId)
        .then((updated) => {
          return queryClient.setQueryData(DOWNLOADED_MODS_QUERY_KEY, updated ?? []);
        })
        .catch(() => {
          // Update availability is optional.
        });
    }
  }, [downloadedQuery.data, queryClient, view]);

  useEffect(() => {
    if (!loading && restoreScroll.current) {
      restoreScroll.current = false;
      const stored = readStorage("sessionStorage", `waxlight.mods.scroll:${location.search}`);
      if (stored) window.scrollTo({ top: Number(stored) });
    }
  }, [loading, location.search]);

  const syncedCatalog = useMemo(
    () => synchronizeDownloadedState(catalog, downloaded),
    [catalog, downloaded],
  );

  const filteredDownloaded = useMemo(() => {
    let result = downloaded.filter((item) => {
      const text = query.text.toLowerCase();
      if (text && !`${item.name} ${item.authorName} ${item.modId}`.toLowerCase().includes(text)) {
        return false;
      }
      if (query.side && item.side !== query.side) return false;
      if (
        query.gameVersion &&
        !item.gameVersions.some((version) => matchesGameVersionSeries(version, query.gameVersion))
      ) {
        return false;
      }
      if (query.tags.length > 0 && !query.tags.every((tag) => item.tags?.includes(tag)))
        return false;
      return true;
    });
    const sorted = [...result];
    Array.prototype.sort.call(sorted, (left, right) => {
      if (query.sort === "name_asc") return left.name.localeCompare(right.name);
      if (query.sort === "name_desc") return right.name.localeCompare(left.name);
      return new Date(right.downloadedAt).getTime() - new Date(left.downloadedAt).getTime();
    });
    return sorted;
  }, [downloaded, query.gameVersion, query.side, query.sort, query.tags, query.text]);

  function updateParams(values: Record<string, string | undefined>, replace = false) {
    const next = new URLSearchParams(searchParams);
    for (const [key, value] of Object.entries(values)) {
      if (!value) next.delete(key);
      else next.set(key, value);
    }
    setSearchParams(next, { replace });
  }

  function patchFilters(patch: Partial<ModSearchQuery>) {
    if ("tags" in patch) {
      const next = new URLSearchParams(searchParams);
      next.delete("tag");
      for (const tag of patch.tags ?? []) next.append("tag", tag);
      setSearchParams(next);
      return;
    }
    const next: Record<string, string | undefined> = {};
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

  function retry() {
    if (view === "all") {
      void searchQuery.refetch();
      void queryClient.invalidateQueries({ queryKey: MOD_TAGS_QUERY_KEY });
    } else {
      void downloadedQuery.refetch();
    }
  }

  const toggleSelectedMod = useCallback(
    (modId: string, selected: boolean) => {
      setSelectedMod(modId, selected);
    },
    [setSelectedMod],
  );

  const downloadedByModId = useMemo(
    () => new Map(downloaded.map((item) => [item.modId, item])),
    [downloaded],
  );

  const openBatchInstaller = useCallback(async () => {
    setOpeningBatch(true);
    try {
      const details = await Promise.all(selectedModIds.map((modId) => modCatalogApi.get(modId)));
      const selected = details.flatMap((item) => {
        const release = chooseRelease(item.versions);
        return release
          ? [{ details: item, release, downloaded: downloadedByModId.get(item.id) }]
          : [];
      });
      if (selected.length === 0) {
        notify(t("no_downloadable_mod_version"), "error");
        return;
      }
      setBatchMods(selected);
    } catch (loadError) {
      notify(errorMessage(loadError), "error");
    } finally {
      setOpeningBatch(false);
    }
  }, [downloadedByModId, notify, selectedModIds, t]);

  async function previewUnusedDownloadedMods() {
    try {
      const preview = await modCatalogApi.previewUnusedDownloaded();
      if (preview.removedCount === 0) {
        notify(t("no_unused_downloaded_mods"));
        return;
      }
      setCleanupPreview(preview);
    } catch (previewError) {
      notify(errorMessage(previewError), "error");
    }
  }

  async function removeUnusedDownloadedMods() {
    setCleaning(true);
    try {
      const result = await modCatalogApi.removeUnusedDownloaded();
      setCleanupPreview(undefined);
      await queryClient.invalidateQueries({ queryKey: DOWNLOADED_MODS_QUERY_KEY });
      notify(t("unused_downloaded_mods_removed", { count: result.removedCount }));
    } catch (cleanupError) {
      notify(errorMessage(cleanupError), "error");
    } finally {
      setCleaning(false);
    }
  }

  const localByModId = useMemo(
    () => new Map(filteredDownloaded.map((item) => [item.modId, item])),
    [filteredDownloaded],
  );

  const openInstaller = useCallback(
    async (modId: string, local?: DownloadedMod) => {
      setOpeningModId(modId);
      try {
        const details = await modCatalogApi.get(modId);
        const localDownloaded = local ?? localByModId.get(modId);
        setInstalling({
          details,
          downloaded: localDownloaded,
          preferredVersionId: localDownloaded?.updateAvailable
            ? details.versions[0]?.id
            : localDownloaded?.versionId,
        });
      } catch (loadError) {
        notify(errorMessage(loadError), "error");
      } finally {
        setOpeningModId("");
      }
    },
    [localByModId, notify],
  );

  const openDetails = useCallback(
    (modId: string) => {
      writeStorage(
        "sessionStorage",
        `waxlight.mods.scroll:${location.search}`,
        String(window.scrollY),
      );
      void navigate(
        `/mods/${encodeURIComponent(modId)}?from=${encodeURIComponent(location.search)}`,
      );
    },
    [location.search, navigate],
  );

  const handleOpen = useCallback((modId: string) => openDetails(modId), [openDetails]);

  const handleInstall = useCallback(
    (modId: string, local?: DownloadedMod) => {
      void openInstaller(modId, local);
    },
    [openInstaller],
  );

  const handleDelete = useCallback(
    async (local: DownloadedMod) => {
      const warning =
        local.installedInstances.length > 0
          ? t("delete_cached_installed_mod_confirmation", {
              count: local.installedInstances.length,
            })
          : t("delete_cached_mod_confirmation");
      if (settings?.confirmDeletion === false) {
        try {
          await modCatalogApi.removeDownloaded(local.modId, local.versionId);
          await queryClient.invalidateQueries({ queryKey: DOWNLOADED_MODS_QUERY_KEY });
          notify(t("downloaded_mod_removed"));
        } catch (removeError) {
          notify(errorMessage(removeError), "error");
        }
        return;
      }
      setDeleteConfirm({
        open: true,
        title: warning,
        onConfirm: async () => {
          try {
            await modCatalogApi.removeDownloaded(local.modId, local.versionId);
            await queryClient.invalidateQueries({ queryKey: DOWNLOADED_MODS_QUERY_KEY });
            notify(t("downloaded_mod_removed"));
          } catch (removeError) {
            notify(errorMessage(removeError), "error");
          }
        },
      });
    },
    [notify, queryClient, settings, t],
  );

  async function uploadMods() {
    let paths: string[];
    try {
      paths = (await settingsApi.selectModFiles()) ?? [];
    } catch (pickError) {
      notify(errorMessage(pickError), "error");
      return;
    }
    if (paths.length === 0) return;
    try {
      const result = await modCatalogApi.uploadMods(paths);
      const linked = result.linked ?? [];
      const notMatched = result.notMatched ?? [];
      const skipped = result.skipped ?? [];
      const failed = result.failed ?? [];
      await queryClient.invalidateQueries({ queryKey: DOWNLOADED_MODS_QUERY_KEY });
      const counts: string[] = [];
      if (linked.length > 0) {
        counts.push(t("mods_linked_count", { count: linked.length }));
      }
      if (notMatched.length > 0) {
        counts.push(t("mods_not_matched_count", { count: notMatched.length }));
      }
      if (skipped.length > 0) {
        counts.push(t("mods_upload_skipped_count", { count: skipped.length }));
      }
      if (failed.length > 0) {
        counts.push(t("mods_link_failed_count", { count: failed.length }));
      }
      notify(counts.join(" · ") || t("mod_uploaded_none"));
    } catch (uploadError) {
      notify(errorMessage(uploadError), "error");
    }
  }

  const displayed =
    view === "all" ? syncedCatalog : filteredDownloaded.map((mod) => downloadedAsSummary(mod, t));

  return (
    <>
      <PageHeader
        eyebrow={t("mod_browser")}
        title={t("mods")}
        description={t("mods_description")}
        action={
          <div className="modsHeaderActions">
            {view === "downloaded" && (
              <Button variant="danger" onClick={() => void previewUnusedDownloadedMods()}>
                {t("remove_unused_downloaded_mods")}
              </Button>
            )}
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
            {view === "downloaded" && (
              <Button variant="secondary" onClick={() => void uploadMods()}>
                <span aria-hidden="true">＋</span> {t("upload_mods")}
              </Button>
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

      {selectedModIds.length > 0 && (
        <div className="selectedModsBar">
          <strong>{t("selected_mods_count", { count: selectedModIds.length })}</strong>
          <Button variant="ghost" onClick={clearSelectedMods}>
            {t("cancel")}
          </Button>
          <Button busy={openingBatch} onClick={() => void openBatchInstaller()}>
            {t("add_mods_or_create_instance")}
          </Button>
        </div>
      )}

      <div className="modsTabs" role="tablist">
        <button
          role="tab"
          aria-selected={view === "all"}
          className={view === "all" ? "active" : ""}
          onClick={() => updateParams({ view: "all" })}
        >
          {t("all_mods")}
        </button>
        <button
          role="tab"
          aria-selected={view === "downloaded"}
          className={view === "downloaded" ? "active" : ""}
          onClick={() => updateParams({ view: "downloaded" })}
        >
          {t("downloaded")} <b>{downloaded.length || ""}</b>
        </button>
      </div>

      <ModsFilters
        query={query}
        series={versionSeries}
        tags={tags}
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
          action={<Button onClick={retry}>{t("retry")}</Button>}
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
              <div className="row">
                <Button onClick={() => updateParams({ view: "all" })}>{t("browse_mods")}</Button>
                <Button variant="secondary" onClick={() => void uploadMods()}>
                  <span aria-hidden="true">＋</span> {t("upload_mods")}
                </Button>
              </div>
            ) : (
              <Button onClick={clearFilters}>{t("clear_filters")}</Button>
            )
          }
        />
      ) : (
        <div className={`modGrid modGrid-${layout}`}>
          {displayed.map((mod) => {
            const local = view === "downloaded" ? localByModId.get(mod.id) : undefined;
            return (
              <ModCard
                key={`${mod.id}:${local?.versionId ?? "catalog"}`}
                mod={mod}
                downloaded={local}
                layout={layout}
                onOpen={handleOpen}
                onInstall={handleInstall}
                selected={selectedModIds.includes(mod.id)}
                onSelectedChange={toggleSelectedMod}
                installBusy={openingModId === mod.id}
                onDelete={local ? handleDelete : undefined}
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
            onClick={() => void searchQuery.fetchNextPage()}
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
            await queryClient.invalidateQueries({ queryKey: DOWNLOADED_MODS_QUERY_KEY });
            notify(t("mod_task_completed"));
          }}
        />
      )}

      {batchMods && (
        <BatchInstancePickerDialog
          mods={batchMods}
          instances={instances}
          gameVersions={versions}
          accounts={accounts}
          onClose={() => setBatchMods(undefined)}
          onCreated={async () => {
            await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
            notify(t("instance_created"));
          }}
          onDone={async () => {
            await Promise.all([
              queryClient.invalidateQueries({ queryKey: DOWNLOADED_MODS_QUERY_KEY }),
              queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY }),
            ]);
            clearSelectedMods();
            notify(t("mod_task_completed"));
          }}
        />
      )}

      <ConfirmDialog
        open={deleteConfirm.open}
        title={deleteConfirm.title}
        message={deleteConfirm.message}
        destructive
        onConfirm={() => {
          setDeleteConfirm((s) => ({ ...s, open: false }));
          deleteConfirm.onConfirm();
        }}
        onCancel={() => setDeleteConfirm((s) => ({ ...s, open: false }))}
      />

      <ConfirmDialog
        open={cleanupPreview !== undefined}
        title={t("remove_unused_downloaded_mods")}
        message={t("unused_downloaded_mods_confirm", { count: cleanupPreview?.removedCount ?? 0 })}
        warningMessage={t("unused_downloaded_mods_warning")}
        confirmLabel={t("remove")}
        destructive
        loading={cleaning}
        onConfirm={() => void removeUnusedDownloadedMods()}
        onCancel={() => setCleanupPreview(undefined)}
      />
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
