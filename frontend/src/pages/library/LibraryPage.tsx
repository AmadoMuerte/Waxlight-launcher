import { useQueryClient } from "@tanstack/react-query";
import { Import, Plus } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { useToastStore } from "../../app/stores/toast";
import { useAccountsQuery } from "../../entities/account/queries";
import { useGameVersionsQuery } from "../../entities/game-version/queries";
import { instancesApi, launcherApi } from "../../entities/instance/api";
import type { Instance } from "../../entities/instance/model";
import { useInstancesQuery } from "../../entities/instance/queries";
import { modsApi } from "../../entities/mod/api";
import type { InstanceModUpdateReport } from "../../entities/mod/model";
import { settingsApi } from "../../entities/settings/api";
import { useSettingsQuery } from "../../entities/settings/queries";
import { ExportInstanceModal } from "../../features/instance-package/ExportInstanceModal";
import { CloneInstanceModal } from "../../features/instance/CloneInstanceModal";
import { CreateInstanceModal } from "../../features/instances/CreateInstanceModal";
import { InstanceCard } from "../../features/instances/InstanceCard";
import { InstanceModal } from "../../features/instances/InstanceModal";
import { errorMessage } from "../../shared/api/bridge";
import { instancePackageApi } from "../../shared/api/instance-package";
import {
  INSTANCES_QUERY_KEY,
  OPERATIONS_QUERY_KEY,
  SETTINGS_QUERY_KEY,
} from "../../shared/api/keys";
import type { LibrarySort } from "../../shared/api/types";
import { Button } from "../../shared/ui/button";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { EmptyState } from "../../shared/ui/empty";
import { ErrorState } from "../../shared/ui/error-state";
import { LoadingState } from "../../shared/ui/loading-state";
import { Page, PageContent } from "../../shared/ui/page";
import { PageHeader } from "../../shared/ui/page-header";
import { SearchInput } from "../../shared/ui/search-input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../shared/ui/select";
import { Toolbar, ToolbarGroup } from "../../shared/ui/toolbar";

const nameCollator = new Intl.Collator(undefined, { sensitivity: "base" });
const versionCollator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

function parseVersion(value: string) {
  const match = value.match(/(\d+(?:\.\d+)+)(?:[- ](.+))?/);
  return match
    ? { numbers: match[1].split(".").map(Number), prerelease: match[2] ?? "" }
    : undefined;
}

function compareVersions(left: string, right: string) {
  const leftVersion = parseVersion(left);
  const rightVersion = parseVersion(right);
  if (!leftVersion || !rightVersion) return versionCollator.compare(left, right);

  const length = Math.max(leftVersion.numbers.length, rightVersion.numbers.length);
  for (let index = 0; index < length; index += 1) {
    const difference = (leftVersion.numbers[index] ?? 0) - (rightVersion.numbers[index] ?? 0);
    if (difference) return difference;
  }
  if (!leftVersion.prerelease) return rightVersion.prerelease ? 1 : 0;
  if (!rightVersion.prerelease) return -1;
  return versionCollator.compare(leftVersion.prerelease, rightVersion.prerelease);
}

function compareName(left: Instance, right: Instance) {
  return nameCollator.compare(left.name, right.name) || left.id.localeCompare(right.id);
}

function compareOptionalDates(left?: string, right?: string) {
  const leftTime = left ? Date.parse(left) : Number.NaN;
  const rightTime = right ? Date.parse(right) : Number.NaN;
  if (Number.isNaN(leftTime)) return Number.isNaN(rightTime) ? 0 : 1;
  if (Number.isNaN(rightTime)) return -1;
  return rightTime - leftTime;
}

function normalizeLibrarySort(value: string): LibrarySort {
  switch (value) {
    case "name":
    case "playtime":
    case "gameVersion":
    case "createdAt":
      return value;
    default:
      return "lastPlayed";
  }
}

export function LibraryPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  const navigate = useNavigate();
  const instancesQuery = useInstancesQuery();
  const { data: instances = [] } = instancesQuery;
  const { data: versions = [] } = useGameVersionsQuery();
  const { data: accounts = [] } = useAccountsQuery();
  const { data: settings } = useSettingsQuery();
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [selectedInstance, setSelectedInstance] = useState<Instance>();
  const [selectedTab, setSelectedTab] = useState<"overview" | "settings">("overview");
  const [cloningInstance, setCloningInstance] = useState<Instance>();
  const [exportingInstance, setExportingInstance] = useState<Instance>();
  const [deletingInstance, setDeletingInstance] = useState<Instance>();
  const [deleteBusy, setDeleteBusy] = useState(false);
  const [busyInstanceIDs, setBusyInstanceIDs] = useState<Set<string>>(new Set());
  const [pinningInstanceIDs, setPinningInstanceIDs] = useState<Set<string>>(new Set());
  const [query, setQuery] = useState("");
  const [modUpdates, setModUpdates] = useState<Record<string, InstanceModUpdateReport>>({});
  const instancesRef = useRef(instances);
  const checkedOnceRef = useRef(false);
  const sortRevisionRef = useRef(0);
  const sortSaveQueueRef = useRef(Promise.resolve());

  useEffect(() => {
    instancesRef.current = instances;
  }, [instances]);

  const checkAllUpdates = useCallback(async () => {
    const current = instancesRef.current;
    if (current.length === 0) return;

    const entries = await Promise.all(
      current.map(async (instance) => {
        try {
          const report = await modsApi.checkInstanceUpdates(instance.id);
          return [instance.id, report] as const;
        } catch {
          return undefined;
        }
      }),
    );
    const collected: Record<string, InstanceModUpdateReport> = {};
    for (const entry of entries) {
      if (entry) collected[entry[0]] = entry[1];
    }
    setModUpdates(collected);
  }, []);

  const handleModUpdatesChanged = useCallback(
    (instanceID: string, report: InstanceModUpdateReport) =>
      setModUpdates((current) => ({ ...current, [instanceID]: report })),
    [],
  );

  useEffect(() => {
    if (checkedOnceRef.current) return;
    checkedOnceRef.current = true;
    void checkAllUpdates();
  }, [checkAllUpdates]);

  async function startImport() {
    try {
      const path = await instancePackageApi.selectPackageFile();
      if (!path) return;
      await instancePackageApi.import({
        packagePath: path,
        name: "",
        description: "",
        directory: "",
        gameVersionId: "",
        installVersion: true,
        allowIncompatible: false,
        skipUnavailable: true,
      });
      await queryClient.invalidateQueries({ queryKey: OPERATIONS_QUERY_KEY });
      void navigate("/operations");
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  const versionById = useMemo(
    () => new Map(versions.map((version) => [version.id, version])),
    [versions],
  );
  const librarySort = settings?.librarySort ?? "lastPlayed";
  const visibleInstances = useMemo(() => {
    const normalizedQuery = query.toLocaleLowerCase();
    return instances
      .filter((instance) => instance.name.toLocaleLowerCase().includes(normalizedQuery))
      .toSorted((left, right) => {
        if (left.isPinned !== right.isPinned) return left.isPinned ? -1 : 1;

        let result = 0;
        if (librarySort === "lastPlayed") {
          result = compareOptionalDates(left.lastPlayedAt, right.lastPlayedAt);
        } else if (librarySort === "playtime") {
          result = right.playtimeSeconds - left.playtimeSeconds;
        } else if (librarySort === "gameVersion") {
          const leftVersion = versionById.get(left.gameVersionId)?.name ?? left.gameVersionId;
          const rightVersion = versionById.get(right.gameVersionId)?.name ?? right.gameVersionId;
          if (!leftVersion) result = rightVersion ? 1 : 0;
          else if (!rightVersion) result = -1;
          else result = compareVersions(rightVersion, leftVersion);
        } else if (librarySort === "createdAt") {
          result = compareOptionalDates(left.createdAt, right.createdAt);
        }
        return result || compareName(left, right);
      });
  }, [instances, librarySort, query, versionById]);

  const [launchWarnConfirm, setLaunchWarnConfirm] = useState<{
    open: boolean;
    title: string;
    message?: string;
    onConfirm: () => void;
  }>({ open: false, title: "", onConfirm: () => {} });

  const setInstanceBusy = useCallback((instanceID: string, busy: boolean) => {
    setBusyInstanceIDs((current) => {
      const next = new Set(current);
      if (busy) next.add(instanceID);
      else next.delete(instanceID);
      return next;
    });
  }, []);

  const handleTogglePin = useCallback(
    async (instance: Instance) => {
      const isPinned = !instance.isPinned;
      setPinningInstanceIDs((current) => new Set(current).add(instance.id));
      queryClient.setQueryData<Instance[]>(INSTANCES_QUERY_KEY, (current = []) =>
        current.map((item) => (item.id === instance.id ? { ...item, isPinned } : item)),
      );
      try {
        await instancesApi.setPinned(instance.id, isPinned);
      } catch (error) {
        queryClient.setQueryData<Instance[]>(INSTANCES_QUERY_KEY, (current = []) =>
          current.map((item) =>
            item.id === instance.id ? { ...item, isPinned: instance.isPinned } : item,
          ),
        );
        notify(errorMessage(error), "error");
      } finally {
        setPinningInstanceIDs((current) => {
          const next = new Set(current);
          next.delete(instance.id);
          return next;
        });
      }
    },
    [notify, queryClient],
  );

  const handleSortChange = useCallback(
    (nextSort: LibrarySort) => {
      if (!settings) return;
      const next = { ...settings, librarySort: nextSort };
      const revision = ++sortRevisionRef.current;
      queryClient.setQueryData(SETTINGS_QUERY_KEY, next);
      const previousSave = sortSaveQueueRef.current;
      sortSaveQueueRef.current = (async () => {
        await previousSave;
        try {
          const saved = await settingsApi.setLibrarySort(nextSort);
          if (revision === sortRevisionRef.current) {
            queryClient.setQueryData(SETTINGS_QUERY_KEY, saved);
          }
        } catch (error) {
          if (revision === sortRevisionRef.current) {
            await queryClient.invalidateQueries({ queryKey: SETTINGS_QUERY_KEY });
            notify(errorMessage(error), "error");
          }
        }
      })();
    },
    [notify, queryClient, settings],
  );

  const startValidatedInstance = useCallback(
    async (instance: Instance) => {
      setInstanceBusy(instance.id, true);
      try {
        await launcherApi.launch(instance.id, instance.defaultAccountId);
        notify(t("started_instance", { name: instance.name }));
        await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
      } catch (error) {
        notify(errorMessage(error), "error");
      } finally {
        setInstanceBusy(instance.id, false);
      }
    },
    [notify, queryClient, setInstanceBusy, t],
  );

  const launch = useCallback(
    async (instance: Instance) => {
      setInstanceBusy(instance.id, true);
      try {
        const validation = await launcherApi.validate(instance.id, instance.defaultAccountId);
        const issues = validation?.issues ?? [];
        const warnings = validation?.warnings ?? [];

        if (!validation?.valid) {
          throw new Error(issues.join(". ") || t("instance_cannot_launch"));
        }
        if (warnings.length > 0) {
          setLaunchWarnConfirm({
            open: true,
            title: t("launch_anyway"),
            message: warnings.join("\n"),
            onConfirm: () => void startValidatedInstance(instance),
          });
          return;
        }
        await startValidatedInstance(instance);
      } catch (error) {
        notify(errorMessage(error), "error");
      } finally {
        setInstanceBusy(instance.id, false);
      }
    },
    [notify, setInstanceBusy, startValidatedInstance, t],
  );

  const handleOpen = useCallback((instance: Instance) => {
    setSelectedTab("overview");
    setSelectedInstance(instance);
  }, []);

  const handleEdit = useCallback((instance: Instance) => {
    setSelectedTab("settings");
    setSelectedInstance(instance);
  }, []);

  const handleOpenDirectory = useCallback(
    async (instance: Instance) => {
      try {
        await settingsApi.openDirectory(instance.directory);
      } catch (error) {
        notify(errorMessage(error), "error");
      }
    },
    [notify],
  );

  const removeInstance = useCallback(
    async (instance: Instance) => {
      setDeleteBusy(true);
      setInstanceBusy(instance.id, true);
      try {
        await instancesApi.remove(instance.id, true);
        setDeletingInstance(undefined);
        setSelectedInstance(undefined);
        await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
        notify(t("instance_deleted"));
      } catch (error) {
        notify(errorMessage(error), "error");
      } finally {
        setDeleteBusy(false);
        setInstanceBusy(instance.id, false);
      }
    },
    [notify, queryClient, setInstanceBusy, t],
  );

  const handleDelete = useCallback(
    (instance: Instance) => {
      if (settings?.confirmDeletion === false) {
        void removeInstance(instance);
        return;
      }
      setDeletingInstance(instance);
    },
    [removeInstance, settings?.confirmDeletion],
  );

  const handleStop = useCallback(
    async (instance: Instance) => {
      setInstanceBusy(instance.id, true);
      try {
        await launcherApi.stop(instance.id);
        notify(t("stop_signal_sent"));
      } catch (error) {
        notify(errorMessage(error), "error");
      } finally {
        setInstanceBusy(instance.id, false);
      }
    },
    [notify, setInstanceBusy, t],
  );

  return (
    <Page>
      <PageHeader
        eyebrow={t("your_worlds")}
        title={t("library")}
        description={t("library_description")}
      />

      <PageContent>
        <Toolbar>
          <ToolbarGroup className="min-w-[240px] flex-1">
            <SearchInput
              wrapperClassName="w-full max-w-sm"
              aria-label={t("search_instances")}
              placeholder={t("find_instance_placeholder")}
              value={query}
              disabled={instances.length === 0}
              onValueChange={setQuery}
            />
            {instances.length > 0 && (
              <span className="text-xs whitespace-nowrap text-text-muted">
                {t("instances_count", { count: visibleInstances.length })}
              </span>
            )}
          </ToolbarGroup>
          <ToolbarGroup align="end">
            <Select
              value={librarySort}
              disabled={!settings || instances.length === 0}
              onValueChange={(value) => handleSortChange(normalizeLibrarySort(value))}
            >
              <SelectTrigger className="w-[170px]" aria-label={t("sort_by")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="lastPlayed">{t("sort_last_played")}</SelectItem>
                <SelectItem value="name">{t("sort_name")}</SelectItem>
                <SelectItem value="playtime">{t("sort_playtime")}</SelectItem>
                <SelectItem value="gameVersion">{t("sort_game_version")}</SelectItem>
                <SelectItem value="createdAt">{t("sort_created_at")}</SelectItem>
              </SelectContent>
            </Select>
            <Button variant="secondary" onClick={() => void startImport()}>
              <Import size={16} aria-hidden="true" />
              {t("import_instance")}
            </Button>
            {versions.length > 0 && (
              <Button onClick={() => setCreateDialogOpen(true)}>
                <Plus size={16} aria-hidden="true" />
                {t("new_instance")}
              </Button>
            )}
          </ToolbarGroup>
        </Toolbar>

        {instancesQuery.isPending ? (
          <LoadingState />
        ) : instancesQuery.error && instances.length === 0 ? (
          <ErrorState
            title={t("could_not_connect_to_core")}
            description={errorMessage(instancesQuery.error)}
            action={
              <Button variant="secondary" onClick={() => void instancesQuery.refetch()}>
                {t("retry")}
              </Button>
            }
          />
        ) : visibleInstances.length === 0 ? (
          <EmptyState
            title={query ? t("nothing_found") : t("light_your_first_world")}
            description={
              query ? t("try_another_instance_name") : t("create_first_instance_description")
            }
            action={
              !query && (
                <div className="flex flex-wrap justify-center gap-2">
                  <Button
                    onClick={
                      versions.length > 0
                        ? () => setCreateDialogOpen(true)
                        : () => navigate("/versions")
                    }
                  >
                    {versions.length > 0 ? t("create_instance") : t("install_game_version_first")}
                  </Button>
                  <Button variant="secondary" onClick={() => void startImport()}>
                    {t("import_instance")}
                  </Button>
                </div>
              )
            }
          />
        ) : (
          <div className="grid grid-cols-[repeat(auto-fill,minmax(min(300px,100%),1fr))] gap-4">
            {visibleInstances.map((instance) => (
              <InstanceCard
                key={instance.id}
                instance={instance}
                version={versionById.get(instance.gameVersionId)}
                updateCount={modUpdates[instance.id]?.summary.updatesAvailable ?? 0}
                busy={busyInstanceIDs.has(instance.id)}
                pinBusy={pinningInstanceIDs.has(instance.id)}
                onOpen={handleOpen}
                onEdit={handleEdit}
                onOpenDirectory={(item) => void handleOpenDirectory(item)}
                onClone={setCloningInstance}
                onExport={setExportingInstance}
                onDelete={handleDelete}
                onLaunch={(item) => void launch(item)}
                onStop={handleStop}
                onTogglePin={(item) => void handleTogglePin(item)}
              />
            ))}
          </div>
        )}
      </PageContent>

      {createDialogOpen && (
        <CreateInstanceModal
          versions={versions}
          accounts={accounts}
          onClose={() => setCreateDialogOpen(false)}
          onDone={async () => {
            setCreateDialogOpen(false);
            await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
            notify(t("instance_created"));
          }}
        />
      )}

      {selectedInstance && (
        <InstanceModal
          key={`${selectedInstance.id}:${selectedTab}`}
          instance={instances.find((item) => item.id === selectedInstance.id) ?? selectedInstance}
          initialTab={selectedTab}
          versions={versions}
          accounts={accounts}
          onClose={() => setSelectedInstance(undefined)}
          onExport={() => setExportingInstance(selectedInstance)}
          onClone={() => setCloningInstance(selectedInstance)}
          onModUpdatesChanged={handleModUpdatesChanged}
        />
      )}

      {cloningInstance && (
        <CloneInstanceModal
          instance={instances.find((item) => item.id === cloningInstance.id) ?? cloningInstance}
          onClose={() => setCloningInstance(undefined)}
          onDone={async () => {
            setCloningInstance(undefined);
            setSelectedInstance(undefined);
            await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
          }}
        />
      )}

      {exportingInstance && (
        <ExportInstanceModal
          instance={exportingInstance}
          onClose={() => setExportingInstance(undefined)}
          onDone={async () => {
            setExportingInstance(undefined);
            await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
          }}
        />
      )}

      <ConfirmDialog
        open={launchWarnConfirm.open}
        title={launchWarnConfirm.title}
        message={launchWarnConfirm.message}
        onConfirm={() => {
          setLaunchWarnConfirm((state) => ({ ...state, open: false }));
          launchWarnConfirm.onConfirm();
        }}
        onCancel={() => setLaunchWarnConfirm((state) => ({ ...state, open: false }))}
      />

      <ConfirmDialog
        open={Boolean(deletingInstance)}
        title={
          deletingInstance ? t("delete_instance_confirmation", { name: deletingInstance.name }) : ""
        }
        destructive
        loading={deleteBusy}
        onConfirm={() => {
          if (deletingInstance) void removeInstance(deletingInstance);
        }}
        onCancel={() => setDeletingInstance(undefined)}
      />
    </Page>
  );
}
