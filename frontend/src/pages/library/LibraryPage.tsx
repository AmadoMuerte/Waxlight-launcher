import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import { useToastStore } from "../../app/stores/toast";
import { useAccountsQuery } from "../../entities/account/queries";
import { useGameVersionsQuery } from "../../entities/game-version/queries";
import { launcherApi } from "../../entities/instance/api";
import { useInstancesQuery } from "../../entities/instance/queries";
import { modsApi } from "../../entities/mod/api";
import type { InstanceModUpdateReport } from "../../entities/mod/model";
import { ExportInstanceModal } from "../../features/instance-package/ExportInstanceModal";
import { ImportPackageModal } from "../../features/instance-package/ImportPackageModal";
import { ImportResultModal } from "../../features/instance-package/ImportResultModal";
import { CloneInstanceModal } from "../../features/instance/CloneInstanceModal";
import { CreateInstanceModal } from "../../features/instances/CreateInstanceModal";
import { InstanceCard } from "../../features/instances/InstanceCard";
import { InstanceModal } from "../../features/instances/InstanceModal";
import { instancePackageApi, type ImportReport, type PackageInspection } from "../../shared/api";
import { errorMessage } from "../../shared/api/bridge";
import { GAME_VERSIONS_QUERY_KEY, INSTANCES_QUERY_KEY } from "../../shared/api/keys";
import { Button, Empty, PageHeader } from "../../shared/ui";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";

export function LibraryPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  const navigate = useNavigate();
  const { data: instances = [] } = useInstancesQuery();
  const { data: versions = [] } = useGameVersionsQuery();
  const { data: accounts = [] } = useAccountsQuery();
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [selectedInstance, setSelectedInstance] = useState<(typeof instances)[number]>();
  const [cloningInstance, setCloningInstance] = useState<(typeof instances)[number]>();
  const [exportingInstance, setExportingInstance] = useState<(typeof instances)[number]>();
  const [importInspection, setImportInspection] = useState<PackageInspection>();
  const [importResult, setImportResult] = useState<ImportReport>();
  const [query, setQuery] = useState("");
  const [modUpdates, setModUpdates] = useState<Record<string, InstanceModUpdateReport>>({});
  const instancesRef = useRef(instances);
  const checkedOnceRef = useRef(false);

  useEffect(() => {
    instancesRef.current = instances;
  }, [instances]);

  const checkAllUpdates = useCallback(async () => {
    const current = instancesRef.current;
    if (current.length === 0) {
      return;
    }
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
      if (entry) {
        collected[entry[0]] = entry[1];
      }
    }
    setModUpdates(collected);
  }, []);

  useEffect(() => {
    if (checkedOnceRef.current) {
      return;
    }
    checkedOnceRef.current = true;
    void checkAllUpdates();
  }, [checkAllUpdates]);

  async function startImport() {
    try {
      const path = await instancePackageApi.selectPackageFile();
      if (!path) {
        return;
      }
      const inspection = await instancePackageApi.inspect(path);
      setImportInspection(inspection);
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  const visibleInstances = useMemo(
    () => instances.filter((instance) => instance.name.toLowerCase().includes(query.toLowerCase())),
    [instances, query],
  );

  const [launchWarnConfirm, setLaunchWarnConfirm] = useState<{
    open: boolean;
    title: string;
    message?: string;
    onConfirm: () => void;
  }>({ open: false, title: "", onConfirm: () => {} });

  async function launch(instance: (typeof instances)[number]) {
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
          onConfirm: async () => {
            await launcherApi.launch(instance.id, instance.defaultAccountId);
            notify(t("started_instance", { name: instance.name }));
            await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
          },
        });
        return;
      }

      await launcherApi.launch(instance.id, instance.defaultAccountId);
      notify(t("started_instance", { name: instance.name }));
      await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  return (
    <>
      <PageHeader
        eyebrow={t("your_worlds")}
        title={t("library")}
        description={t("library_description")}
        action={
          <div className="row">
            <Button variant="secondary" onClick={() => void startImport()}>
              ⤓ {t("import_instance")}
            </Button>
            {versions.length > 0 && (
              <Button onClick={() => setCreateDialogOpen(true)}>＋ {t("new_instance")}</Button>
            )}
          </div>
        }
      />

      {instances.length > 0 && (
        <div className="toolbar">
          <div className="search">
            <span>⌕</span>
            <input
              aria-label={t("search_instances")}
              placeholder={t("find_instance_placeholder")}
              value={query}
              onChange={(event) => setQuery(event.target.value)}
            />
          </div>
          <span className="muted">{t("instances_count", { count: visibleInstances.length })}</span>
        </div>
      )}

      {visibleInstances.length === 0 ? (
        <Empty
          icon="◌"
          title={query ? t("nothing_found") : t("light_your_first_world")}
          description={
            query ? t("try_another_instance_name") : t("create_first_instance_description")
          }
          action={
            !query && (
              <Button
                onClick={
                  versions.length > 0
                    ? () => setCreateDialogOpen(true)
                    : () => navigate("/versions")
                }
              >
                {versions.length > 0 ? t("create_instance") : t("install_game_version_first")}
              </Button>
            )
          }
        />
      ) : (
        <div className="instanceGrid">
          {visibleInstances.map((instance) => (
            <InstanceCard
              key={instance.id}
              instance={instance}
              version={versions.find((version) => version.id === instance.gameVersionId)}
              updateCount={modUpdates[instance.id]?.summary.updatesAvailable ?? 0}
              onOpen={() => setSelectedInstance(instance)}
              onLaunch={() => void launch(instance)}
              onStop={async () => {
                try {
                  await launcherApi.stop(instance.id);
                  notify(t("stop_signal_sent"));
                } catch (error) {
                  notify(errorMessage(error), "error");
                }
              }}
            />
          ))}
        </div>
      )}

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
          instance={instances.find((item) => item.id === selectedInstance.id) ?? selectedInstance}
          versions={versions}
          accounts={accounts}
          onClose={() => setSelectedInstance(undefined)}
          onExport={() => setExportingInstance(selectedInstance)}
          onClone={() => setCloningInstance(selectedInstance)}
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

      {importInspection && (
        <ImportPackageModal
          inspection={importInspection}
          versions={versions}
          onClose={() => setImportInspection(undefined)}
          onDone={async (report) => {
            setImportInspection(undefined);
            setImportResult(report);
            await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
            await queryClient.invalidateQueries({ queryKey: GAME_VERSIONS_QUERY_KEY });
          }}
          onBackgroundDone={async () => {
            setImportInspection(undefined);
            await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
          }}
        />
      )}

      {importResult && (
        <ImportResultModal report={importResult} onClose={() => setImportResult(undefined)} />
      )}

      <ConfirmDialog
        open={launchWarnConfirm.open}
        title={launchWarnConfirm.title}
        message={launchWarnConfirm.message}
        onConfirm={() => {
          setLaunchWarnConfirm((s) => ({ ...s, open: false }));
          launchWarnConfirm.onConfirm();
        }}
        onCancel={() => setLaunchWarnConfirm((s) => ({ ...s, open: false }))}
      />
    </>
  );
}
