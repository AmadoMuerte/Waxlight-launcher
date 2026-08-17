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
import { INSTANCES_QUERY_KEY, OPERATIONS_QUERY_KEY } from "../../shared/api/keys";
import { Button } from "../../shared/ui/button";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { EmptyState } from "../../shared/ui/empty";
import { ErrorState } from "../../shared/ui/error-state";
import { LoadingState } from "../../shared/ui/loading-state";
import { Page, PageContent } from "../../shared/ui/page";
import { PageHeader } from "../../shared/ui/page-header";
import { SearchInput } from "../../shared/ui/search-input";
import { Toolbar, ToolbarGroup } from "../../shared/ui/toolbar";

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
  const [query, setQuery] = useState("");
  const [modUpdates, setModUpdates] = useState<Record<string, InstanceModUpdateReport>>({});
  const instancesRef = useRef(instances);
  const checkedOnceRef = useRef(false);

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

  const visibleInstances = useMemo(
    () => instances.filter((instance) => instance.name.toLowerCase().includes(query.toLowerCase())),
    [instances, query],
  );
  const versionById = useMemo(
    () => new Map(versions.map((version) => [version.id, version])),
    [versions],
  );

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
                onOpen={handleOpen}
                onEdit={handleEdit}
                onOpenDirectory={(item) => void handleOpenDirectory(item)}
                onClone={setCloningInstance}
                onExport={setExportingInstance}
                onDelete={handleDelete}
                onLaunch={(item) => void launch(item)}
                onStop={handleStop}
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
