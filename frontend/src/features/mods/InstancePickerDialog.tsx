import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

import { ConfirmDialog } from "../../components/ui/confirm-dialog";
import {
  modCatalogApi,
  type DownloadedMod,
  type GameVersion,
  type Instance,
  type ModDetails,
  type ModInstallResult,
  type ModTaskProgress,
} from "../../shared/api";
import { errorMessage } from "../../shared/api/bridge";
import { Button, Empty, Field, Modal } from "../../shared/ui";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  chooseRelease,
  compatibilityFor,
  compatibilityLabel,
  formatBytes,
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
  const visibleInstances = useMemo(() => {
    if (!release) return [];
    return instances.filter((instance) => {
      const matches = instance.name.toLowerCase().includes(instanceQuery.toLowerCase());
      const compatibility = compatibilityFor(instance, gameVersions, release);
      return matches && (showIncompatible || compatibility !== "incompatible");
    });
  }, [gameVersions, instanceQuery, instances, release, showIncompatible]);

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
    if (taskId.current) {
      try {
        await modCatalogApi.cancelTask(taskId.current);
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
        <div className="instancePicker">
          <p className="muted">{t("select_one_or_more_instances")}</p>
          <div className="formRow">
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
            <div className="releaseSummary">
              <span>{release?.gameVersions.join(", ") || t("compatibility_unknown")}</span>
              <small>{formatBytes(release?.fileSize ?? 0)}</small>
            </div>
          </div>

          {instances.length === 0 ? (
            <Empty
              icon="◌"
              title={t("no_instances_available")}
              description={t("create_instance_before_mods")}
            />
          ) : (
            <>
              <input
                className="instanceSearch"
                aria-label={t("search_instances")}
                placeholder={t("search_instances_placeholder")}
                value={instanceQuery}
                onChange={(event) => setInstanceQuery(event.target.value)}
              />
              <div className="instanceChoices">
                {visibleInstances.map((instance) => {
                  const compatibility = release
                    ? compatibilityFor(instance, gameVersions, release)
                    : "unknown";
                  return (
                    <label key={instance.id} className="instanceChoice">
                      <input
                        type="checkbox"
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
                      <span>
                        <strong>{instance.name}</strong>
                        <small>Vintage Story {instanceGameVersion(instance, gameVersions)}</small>
                      </span>
                      <span className={`compatibility compatibility-${compatibility}`}>
                        {compatibilityLabel(compatibility)}
                      </span>
                    </label>
                  );
                })}
              </div>
              <label className="checkRow">
                <input
                  type="checkbox"
                  checked={showIncompatible}
                  onChange={(event) => setShowIncompatible(event.target.checked)}
                />
                {t("show_incompatible_instances")}
              </label>
            </>
          )}
          {error && (
            <div className="inlineError" role="alert">
              {error}
            </div>
          )}
          <div className="modalActions">
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
        <div className="taskProgress" aria-live="polite">
          <div className="progressOrb">⇣</div>
          <h3>{progress?.message || t("preparing_mod", { name: mod.name })}</h3>
          <div className="progressTrack">
            <i style={{ width: `${Math.round((progress?.progress ?? 0.05) * 100)}%` }} />
          </div>
          <p>
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
        <div className="installResult">
          <div className="successMark">✓</div>
          <h3>
            {result.installations.length === 0
              ? t("mod_downloaded_successfully")
              : t("installed_to_instances", {
                  installed: result.installations.filter((item) => item.installed).length,
                  total: result.installations.length,
                })}
          </h3>
          {result.installations.length > 0 && (
            <div className="resultList">
              {result.installations.map((item) => (
                <div key={item.instanceId}>
                  <strong>{item.instanceName}</strong>
                  <span className={item.installed ? "resultOk" : "resultError"}>
                    {item.message}
                  </span>
                </div>
              ))}
            </div>
          )}
          {downloadedDependencies.length > 0 && (
            <div className="resultList">
              <div>
                <strong>
                  {t("required_dependencies_downloaded", {
                    count: downloadedDependencies.length,
                  })}
                </strong>
                <span className="resultOk">✓</span>
              </div>
              {downloadedDependencies.map((dependency) => (
                <div key={`${dependency.modId}:${dependency.version}`}>
                  <strong>{dependency.name}</strong>
                  <span>{dependency.version}</span>
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
