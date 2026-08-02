import { useEffect, useMemo, useRef, useState } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";

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
import {
  chooseRelease,
  compatibilityFor,
  compatibilityLabel,
  formatBytes,
  instanceGameVersion,
} from "./lib";

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
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const taskId = useRef("");

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
    let unsubscribe = () => {};
    try {
      unsubscribe = EventsOn("mods:task-progress", (event: ModTaskProgress) => {
        if (event.modId !== mod.id) return;
        taskId.current = event.taskId;
        setProgress(event);
      });
    } catch {
      // The runtime is not present in browser-only tests.
    }
    return unsubscribe;
  }, [mod.id]);

  async function start(downloadOnly: boolean) {
    if (!release) {
      setError("No downloadable version is available.");
      return;
    }
    if (!downloadOnly && selected.length === 0) {
      setError("Select at least one instance or choose Download only.");
      return;
    }
    const hasIncompatible = selected.some((id) => {
      const instance = instances.find((item) => item.id === id);
      return instance && compatibilityFor(instance, gameVersions, release) === "incompatible";
    });
    if (
      hasIncompatible &&
      !window.confirm(
        "This release does not list support for one or more selected game versions. Installing it may cause crashes or corrupted saves. Install anyway?",
      )
    ) {
      return;
    }
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
    <Modal title={`${downloaded ? "Install" : "Download"} “${mod.name}”`} onClose={() => void cancel()}>
      {phase === "select" && (
        <div className="instancePicker">
          <p className="muted">Select one or more instances.</p>
          <div className="formRow">
            <Field label="Mod version">
              <select value={releaseId} onChange={(event) => setReleaseId(event.target.value)}>
                {mod.versions.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.version} · {item.releaseType}
                  </option>
                ))}
              </select>
            </Field>
            <div className="releaseSummary">
              <span>{release?.gameVersions.join(", ") || "Compatibility unknown"}</span>
              <small>{formatBytes(release?.fileSize ?? 0)}</small>
            </div>
          </div>

          {instances.length === 0 ? (
            <Empty
              icon="◌"
              title="No instances available"
              description="Create an instance before installing mods. You can still download the file now."
            />
          ) : (
            <>
              <input
                className="instanceSearch"
                aria-label="Search instances"
                placeholder="Search instances…"
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
                Show incompatible instances
              </label>
            </>
          )}
          {error && <div className="inlineError" role="alert">{error}</div>}
          <div className="modalActions">
            <Button variant="ghost" onClick={onClose}>Cancel</Button>
            {!downloaded && (
              <Button variant="secondary" busy={busy} onClick={() => void start(true)}>
                Download only
              </Button>
            )}
            <Button busy={busy} disabled={selected.length === 0} onClick={() => void start(false)}>
              {downloaded ? "Install" : "Download & Install"}
            </Button>
          </div>
        </div>
      )}

      {phase === "progress" && (
        <div className="taskProgress" aria-live="polite">
          <div className="progressOrb">⇣</div>
          <h3>{progress?.message || `Preparing ${mod.name}`}</h3>
          <div className="progressTrack"><i style={{ width: `${Math.round((progress?.progress ?? 0.05) * 100)}%` }} /></div>
          <p>
            {progress?.totalBytes
              ? `${formatBytes(progress.downloadedBytes)} of ${formatBytes(progress.totalBytes)}`
              : "Contacting the mod database…"}
          </p>
          <Button variant="ghost" onClick={() => void cancel()}>Cancel</Button>
        </div>
      )}

      {phase === "result" && result && (
        <div className="installResult">
          <div className="successMark">✓</div>
          <h3>
            {result.installations.length === 0
              ? "Mod downloaded successfully"
              : `Installed to ${result.installations.filter((item) => item.installed).length} of ${result.installations.length} instances`}
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
          <div className="modalActions"><Button onClick={onClose}>Done</Button></div>
        </div>
      )}
    </Modal>
  );
}
