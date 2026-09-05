import { Check, Download } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui/select";

import { errorMessage } from "../../shared/api/bridge";
import { modCatalogApi } from "../../shared/api/mod-catalog";
import type {
  DownloadedMod,
  GameVersion,
  Instance,
  ModDetails,
  ModInstallResult,
  ModTaskProgress,
} from "../../shared/api/types";
import { Button } from "../../shared/ui/button";
import { Checkbox } from "../../shared/ui/checkbox-control";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { Empty } from "../../shared/ui/empty";
import { Field } from "../../shared/ui/field";
import { Modal } from "../../shared/ui/modal";
import { Progress } from "../../shared/ui/progress";
import { SearchInput } from "../../shared/ui/search-input";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  chooseRelease,
  compatibilityFor,
  compatibilityLabel,
  formatBytes,
  formatGameVersions,
  instanceGameVersion,
  releaseTypeLabel,
} from "./lib";

interface DownloadedDependency {
  modId: string;
  name: string;
  version: string;
}

interface ModDownloadsChangedEvent {
  taskId: string;
  modId: string;
  downloadedDependencies?: DownloadedDependency[];
}

interface InstancePickerDialogProps {
  mod: ModDetails;
  downloaded?: DownloadedMod;
  instances: Instance[];
  gameVersions: GameVersion[];
  preferredInstanceId?: string;
  preferredVersionId?: string;
  onClose: () => void;
  onDone: () => Promise<void>;
}

const compatibilityColor: Record<string, string> = {
  compatible: "text-success",
  possibly_compatible: "text-warning",
  incompatible: "text-danger",
  unknown: "text-text-muted",
};

export function InstancePickerDialog({
  mod,
  downloaded,
  instances,
  gameVersions,
  preferredInstanceId,
  preferredVersionId,
  onClose,
  onDone,
}: InstancePickerDialogProps) {
  const { t } = useTranslation();
  const preferredInstance = instances.find((item) => item.id === preferredInstanceId);
  const preferredVersion = preferredInstance
    ? instanceGameVersion(preferredInstance, gameVersions)
    : undefined;
  const initialRelease = preferredVersionId
    ? mod.versions.find((release) => release.id === preferredVersionId)
    : downloaded
      ? mod.versions.find((release) => release.id === downloaded.versionId)
      : chooseRelease(mod.versions, preferredVersion);
  const [releaseId, setReleaseId] = useState(initialRelease?.id ?? "");
  const [selected, setSelected] = useState<string[]>(
    preferredInstanceId ? [preferredInstanceId] : [],
  );
  const [instanceQuery, setInstanceQuery] = useState("");
  const [showIncompatible, setShowIncompatible] = useState(false);
  const [phase, setPhase] = useState<"select" | "progress" | "result">("select");
  const [progress, setProgress] = useState<ModTaskProgress>();
  const [result, setResult] = useState<ModInstallResult>();
  const [downloadedDependencies, setDownloadedDependencies] = useState<DownloadedDependency[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const taskId = useRef("");
  const taskStarted = useRef(false);
  const [incompatConfirm, setIncompatConfirm] = useState<{
    open: boolean;
    title: string;
    message?: string;
    warningMessage?: string;
    onConfirm: () => void;
  }>({ open: false, title: "", onConfirm: () => {} });

  const release = mod.versions.find((item) => item.id === releaseId);
  const installedById = useMemo(() => {
    const map = new Map<string, { instanceName: string; version: string }>();
    for (const item of downloaded?.installedInstances ?? []) {
      map.set(item.instanceId, item);
    }
    return map;
  }, [downloaded]);
  const visibleInstances = useMemo(() => {
    if (!release) return [];
    return instances.filter((instance) => {
      const matches = instance.name.toLowerCase().includes(instanceQuery.toLowerCase());
      const compatibility = compatibilityFor(instance, gameVersions, release);
      return matches && (showIncompatible || compatibility !== "incompatible");
    });
  }, [gameVersions, instanceQuery, instances, release, showIncompatible]);

  // Instances that already have this mod installed are shown as installed and
  // cannot be selected again.
  useEffect(() => {
    setSelected((items) => items.filter((id) => !installedById.has(id)));
  }, [installedById]);

  useEffect(() => {
    let unsubscribeProgress = () => {};
    let unsubscribeDownloadsChanged = () => {};
    try {
      unsubscribeProgress = EventsOn("mods:task-progress", (event: ModTaskProgress) => {
        if (!taskStarted.current || event.modId !== mod.id) return;
        if (taskId.current && event.taskId !== taskId.current) return;
        taskId.current = event.taskId;
        setProgress(event);
      });
      unsubscribeDownloadsChanged = EventsOn(
        "mods:downloads-changed",
        (event: ModDownloadsChangedEvent) => {
          if (!taskStarted.current || event.modId !== mod.id) return;
          if (taskId.current && event.taskId !== taskId.current) return;
          taskId.current = event.taskId;
          setDownloadedDependencies(event.downloadedDependencies ?? []);
        },
      );
    } catch {
      // The runtime is not present in browser-only tests.
    }
    return () => {
      unsubscribeProgress();
      unsubscribeDownloadsChanged();
    };
  }, [mod.id]);

  async function start(downloadOnly: boolean) {
    if (!release) {
      setError(t("no_downloadable_mod_version"));
      return;
    }
    if (!downloadOnly && selected.length === 0) {
      setError(t("select_instance_or_download_only"));
      return;
    }
    const hasIncompatible = selected.some((id) => {
      const instance = instances.find((item) => item.id === id);
      return instance && compatibilityFor(instance, gameVersions, release) === "incompatible";
    });
    if (hasIncompatible) {
      setIncompatConfirm({
        open: true,
        title: t("unsupported_mod_warning"),
        warningMessage: t("unsupported_mod_warning_detail"),
        onConfirm: async () => {
          taskStarted.current = true;
          taskId.current = "";
          setDownloadedDependencies([]);
          setProgress(undefined);
          setBusy(true);
          setError("");
          setPhase("progress");
          try {
            const response = downloaded
              ? await modCatalogApi.installDownloaded({
                  modId: downloaded.modId,
                  versionId: release.id,
                  instanceIds: selected,
                  allowIncompatible: true,
                })
              : await modCatalogApi.download({
                  modId: mod.id,
                  versionId: release.id,
                  instanceIds: selected,
                  downloadOnly: false,
                  allowIncompatible: true,
                });
            taskId.current = response.taskId;
            setResult(response);
            setPhase("result");
            await onDone();
          } catch (startError) {
            taskStarted.current = false;
            setError(errorMessage(startError));
            setPhase("select");
          } finally {
            setBusy(false);
          }
        },
      });
      return;
    }
    taskStarted.current = true;
    taskId.current = "";
    setDownloadedDependencies([]);
    setProgress(undefined);
    setBusy(true);
    setError("");
    setPhase("progress");
    try {
      const response = downloaded
        ? await modCatalogApi.installDownloaded({
            modId: downloaded.modId,
            versionId: release.id,
            instanceIds: downloadOnly ? [] : selected,
            allowIncompatible: hasIncompatible,
          })
        : await modCatalogApi.download({
            modId: mod.id,
            versionId: release.id,
            instanceIds: downloadOnly ? [] : selected,
            downloadOnly,
            allowIncompatible: hasIncompatible,
          });
      taskId.current = response.taskId;
      setResult(response);
      setPhase("result");
      await onDone();
    } catch (startError) {
      taskStarted.current = false;
      setError(errorMessage(startError));
      setPhase("select");
    } finally {
      setBusy(false);
    }
  }

  async function cancel() {
    const id = taskId.current;
    const finished = progress?.phase === "complete" || progress?.phase === "failed";
    // A finished task has no cancelable backend state; asking the backend to
    // cancel it would surface a harmless "Mod task not found" error.
    if (taskStarted.current && phase !== "result" && !finished && id) {
      try {
        await modCatalogApi.cancelTask(id);
      } catch {
        // The task may have completed between the click and cancellation.
      }
    }
    onClose();
  }

  return (
    <Modal
      title={t(downloaded ? "install_named_mod" : "download_named_mod", { name: mod.name })}
      onClose={() => void cancel()}
    >
      {phase === "select" && (
        <div className="flex min-h-0 flex-col gap-4 p-6">
          <p className="muted">{t("select_one_or_more_instances")}</p>
          <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
            <Field label={t("mod_version")}>
              <Select value={releaseId} onValueChange={setReleaseId}>
                <SelectTrigger>
                  <SelectValue placeholder={t("mod_version")} />
                </SelectTrigger>
                <SelectContent>
                  {mod.versions.map((item) => (
                    <SelectItem key={item.id} value={item.id}>
                      {item.version} · {releaseTypeLabel(item.releaseType)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            <div className="pb-1 text-right text-[length:var(--fs-body)] leading-5 text-text-secondary">
              <span>
                {release && release.gameVersions.length > 0
                  ? formatGameVersions(release.gameVersions)
                  : t("compatibility_unknown")}
              </span>
              <small className="block text-xs text-text-muted">
                {formatBytes(release?.fileSize ?? 0)}
              </small>
            </div>
          </div>

          {instances.length === 0 ? (
            <Empty
              title={t("no_instances_available")}
              description={t("create_instance_before_mods")}
            />
          ) : (
            <div className="flex min-h-0 flex-col gap-3">
              <SearchInput
                aria-label={t("search_instances")}
                placeholder={t("search_instances_placeholder")}
                value={instanceQuery}
                onValueChange={setInstanceQuery}
              />
              <div className="max-h-72 overflow-y-auto rounded-lg border border-border-subtle bg-surface-input">
                <div className="divide-y divide-border-subtle">
                  {visibleInstances.map((instance) => {
                    const compatibility = release
                      ? compatibilityFor(instance, gameVersions, release)
                      : "unknown";
                    const installed = installedById.get(instance.id);
                    return (
                      <label
                        key={instance.id}
                        className="flex cursor-pointer items-center gap-3 px-4 py-3 transition-colors hover:bg-surface-ghost-hover"
                      >
                        {installed ? (
                          <span className="grid size-5 shrink-0 place-items-center rounded-full bg-success/15 text-success">
                            <Check size={12} aria-hidden="true" />
                          </span>
                        ) : (
                          <input
                            type="checkbox"
                            aria-label={t("select_instance", {
                              instance: instance.name,
                              mod: mod.name,
                            })}
                            className="size-4 shrink-0 accent-[var(--color-accent)]"
                            checked={selected.includes(instance.id)}
                            disabled={compatibility === "incompatible" && !showIncompatible}
                            onChange={(event) =>
                              setSelected((items) =>
                                event.target.checked
                                  ? [...items, instance.id]
                                  : items.filter((id) => id !== instance.id),
                              )
                            }
                          />
                        )}
                        <span className="min-w-0 flex-1">
                          <strong className="block truncate text-sm">{instance.name}</strong>
                          <small className="block truncate text-xs text-text-muted">
                            {t("vintage_story")} {instanceGameVersion(instance, gameVersions)}
                          </small>
                          {installed && (
                            <small className="block truncate text-xs text-text-muted">
                              {t("installed_version_value", { version: installed.version })}
                            </small>
                          )}
                        </span>
                        {installed ? (
                          <span className="shrink-0 text-xs font-medium text-success">
                            {t("installed")}
                          </span>
                        ) : (
                          <span
                            className={`shrink-0 text-xs font-medium ${compatibilityColor[compatibility] ?? "text-text-muted"}`}
                          >
                            {compatibilityLabel(compatibility)}
                          </span>
                        )}
                      </label>
                    );
                  })}
                </div>
              </div>
              <Checkbox
                label={t("show_incompatible_instances")}
                checked={showIncompatible}
                onChange={(event) => setShowIncompatible(event.target.checked)}
              />
            </div>
          )}
          {error && (
            <div className="inlineError" role="alert">
              {error}
            </div>
          )}
          <div className="modalActions mt-auto pt-2">
            <Button variant="ghost" onClick={onClose}>
              {t("cancel")}
            </Button>
            {!downloaded && (
              <Button variant="secondary" busy={busy} onClick={() => void start(true)}>
                {t("download_only")}
              </Button>
            )}
            <Button busy={busy} disabled={selected.length === 0} onClick={() => void start(false)}>
              {downloaded ? t("install") : t("download_and_install")}
            </Button>
          </div>
        </div>
      )}

      {phase === "progress" && (
        <div
          className="flex min-h-80 flex-col items-center justify-center gap-4 p-6 text-center"
          aria-live="polite"
        >
          <div className="grid size-16 place-items-center rounded-full bg-accent-muted text-2xl text-accent-hover">
            <Download size={26} aria-hidden="true" />
          </div>
          <h3 className="font-display text-2xl font-semibold">
            {progress?.message || t("preparing_mod", { name: mod.name })}
          </h3>
          <div className="w-[min(420px,100%)]">
            <Progress
              value={Math.round((progress?.progress ?? 0.05) * 100)}
              aria-label={t("download_progress_header")}
            />
          </div>
          <p className="text-[length:var(--fs-body)] text-text-muted">
            {progress?.totalBytes
              ? t("download_progress", {
                  downloaded: formatBytes(progress.downloadedBytes),
                  total: formatBytes(progress.totalBytes),
                })
              : t("contacting_mod_database")}
          </p>
          <Button variant="ghost" onClick={() => void cancel()}>
            {t("cancel")}
          </Button>
        </div>
      )}

      {phase === "result" && result && (
        <div className="flex min-h-80 flex-col items-center justify-center gap-4 p-6 text-center">
          <div className="grid size-16 place-items-center rounded-full bg-success/15 text-2xl text-success">
            <Check size={26} aria-hidden="true" />
          </div>
          <h3 className="font-display text-2xl font-semibold">
            {result.installations.length === 0
              ? t("mod_downloaded_successfully")
              : t("installed_to_instances", {
                  installed: result.installations.filter((item) => item.installed).length,
                  total: result.installations.length,
                })}
          </h3>
          {result.installations.length > 0 && (
            <div className="w-[min(460px,100%)] divide-y divide-border-subtle">
              {result.installations.map((item) => (
                <div
                  key={item.instanceId}
                  className="flex justify-between gap-3 py-2.5 text-[length:var(--fs-body)]"
                >
                  <strong>{item.instanceName}</strong>
                  <span className={item.installed ? "text-success" : "text-danger"}>
                    {item.message}
                  </span>
                </div>
              ))}
            </div>
          )}
          {downloadedDependencies.length > 0 && (
            <div className="w-[min(460px,100%)] divide-y divide-border-subtle">
              <div className="flex justify-between gap-3 py-2.5 text-[length:var(--fs-body)]">
                <strong>
                  {t("required_dependencies_downloaded", {
                    count: downloadedDependencies.length,
                  })}
                </strong>
                <span className="text-success">
                  <Check size={14} aria-hidden="true" />
                </span>
              </div>
              {downloadedDependencies.map((dependency) => (
                <div
                  key={`${dependency.modId}:${dependency.version}`}
                  className="flex justify-between gap-3 py-2.5 text-[length:var(--fs-body)]"
                >
                  <strong>{dependency.name}</strong>
                  <span className="text-text-muted">{dependency.version}</span>
                </div>
              ))}
            </div>
          )}
          <div className="modalActions">
            <Button onClick={onClose}>{t("done")}</Button>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={incompatConfirm.open}
        title={incompatConfirm.title}
        message={incompatConfirm.message}
        warningMessage={incompatConfirm.warningMessage}
        onConfirm={() => {
          setIncompatConfirm((s) => ({ ...s, open: false }));
          incompatConfirm.onConfirm();
        }}
        onCancel={() => setIncompatConfirm((s) => ({ ...s, open: false }))}
      />
    </Modal>
  );
}
