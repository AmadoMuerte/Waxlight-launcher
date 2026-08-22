import { useQueryClient } from "@tanstack/react-query";
import { FolderOpen } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import type { GameVersion } from "../../entities/game-version/model";
import { instancesApi } from "../../entities/instance/api";
import type { Instance } from "../../entities/instance/model";
import { modsApi } from "../../entities/mod/api";
import { operationsApi } from "../../entities/operation/api";
import type { Operation } from "../../entities/operation/model";
import { useOperationsQuery } from "../../entities/operation/queries";
import { settingsApi } from "../../entities/settings/api";
import { errorMessage } from "../../shared/api/bridge";
import { INSTANCES_QUERY_KEY, OPERATIONS_QUERY_KEY } from "../../shared/api/keys";
import type { InstalledMod, MigrationCandidate } from "../../shared/api/types";
import { formatBytes } from "../../shared/lib";
import { Button } from "../../shared/ui/button";
import { Card, CardContent } from "../../shared/ui/card";
import { DialogFooter } from "../../shared/ui/dialog";
import { Field } from "../../shared/ui/field";
import { Input } from "../../shared/ui/input";
import { Modal } from "../../shared/ui/modal";
import { Progress } from "../../shared/ui/progress";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../shared/ui/select";

interface ExistingDataImportModalProps {
  versions: GameVersion[];
  initialDraft?: ExistingDataImportDraft;
  onClose: () => void;
  onOpenInstance: (instance: Instance) => void;
  onOpenVersions: (draft: ExistingDataImportDraft) => void;
}

export interface ExistingDataImportDraft {
  sourcePath: string;
  name: string;
}

type Step = "detect" | "summary" | "progress" | "success" | "failed" | "cancelled";

function defaultName(path: string) {
  return path.split(/[\\/]/).findLast(Boolean) ?? "";
}

export function ExistingDataImportModal({
  versions,
  initialDraft,
  onClose,
  onOpenInstance,
  onOpenVersions,
}: ExistingDataImportModalProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data: operations = [] } = useOperationsQuery({ refetchInterval: 1_000 });
  const [step, setStep] = useState<Step>("detect");
  const [candidates, setCandidates] = useState<MigrationCandidate[]>();
  const [candidate, setCandidate] = useState<MigrationCandidate>();
  const [name, setName] = useState("");
  const [versionID, setVersionID] = useState("");
  const [operation, setOperation] = useState<Operation>();
  const [result, setResult] = useState<{ instance?: Instance; mods: InstalledMod[] }>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const loadedOperation = useRef("");

  useEffect(() => {
    let active = true;
    void (async () => {
      try {
        if (initialDraft) {
          const inspected = await instancesApi.inspectExistingData(initialDraft.sourcePath);
          const detectedVersion = versions.find(
            (version) =>
              version.id === inspected.detectedGameVersion ||
              version.name === inspected.detectedGameVersion,
          );
          if (active) {
            setCandidate(inspected);
            setName(initialDraft.name);
            setVersionID(detectedVersion?.id ?? "");
            setStep("summary");
          }
          return;
        }
        const found = await instancesApi.detectExistingData();
        if (active) setCandidates(found);
      } catch (detectError) {
        if (active) setError(errorMessage(detectError));
      }
    })();
    return () => {
      active = false;
    };
  }, [initialDraft, versions]);

  const trackedOperation = operations.find((item) => item.id === operation?.id) ?? operation;

  useEffect(() => {
    if (
      !trackedOperation ||
      trackedOperation.status !== "completed" ||
      !trackedOperation.resourceId
    )
      return;
    if (loadedOperation.current === trackedOperation.id) return;
    loadedOperation.current = trackedOperation.id;
    const instanceID = trackedOperation.resourceId;
    void (async () => {
      const [instanceResult, modsResult] = await Promise.allSettled([
        instancesApi.get(instanceID),
        modsApi.list(instanceID),
      ]);
      setResult({
        instance: instanceResult.status === "fulfilled" ? instanceResult.value : undefined,
        mods: modsResult.status === "fulfilled" ? modsResult.value : [],
      });
      await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
      setStep("success");
    })();
  }, [queryClient, trackedOperation]);

  useEffect(() => {
    if (trackedOperation?.status === "failed") {
      setError(trackedOperation.errorMessage || t("existing_data_import_failed"));
      setStep("failed");
    } else if (trackedOperation?.status === "cancelled") {
      setStep("cancelled");
    }
  }, [t, trackedOperation]);

  async function inspect(path: string) {
    setBusy(true);
    setError("");
    try {
      const inspected = await instancesApi.inspectExistingData(path);
      const detectedVersion = versions.find(
        (version) =>
          version.id === inspected.detectedGameVersion ||
          version.name === inspected.detectedGameVersion,
      );
      setCandidate(inspected);
      setName(defaultName(inspected.path));
      setVersionID(detectedVersion?.id ?? "");
      setStep("summary");
    } catch (inspectError) {
      setError(errorMessage(inspectError));
    } finally {
      setBusy(false);
    }
  }

  async function chooseFolder() {
    try {
      const path = await settingsApi.selectGameDirectory();
      if (path) await inspect(path);
    } catch (selectError) {
      setError(errorMessage(selectError));
    }
  }

  async function startImport() {
    if (!candidate || !versionID) return;
    setBusy(true);
    setError("");
    try {
      const started = await instancesApi.importExistingData({
        sourcePath: candidate.path,
        name: name.trim(),
        description: "",
        gameVersionId: versionID,
      });
      setOperation(started);
      setStep("progress");
      await queryClient.invalidateQueries({ queryKey: OPERATIONS_QUERY_KEY });
    } catch (startError) {
      setError(errorMessage(startError));
    } finally {
      setBusy(false);
    }
  }

  async function cancelImport() {
    if (!operation) return;
    setBusy(true);
    try {
      await operationsApi.cancel(operation.id);
      await queryClient.invalidateQueries({ queryKey: OPERATIONS_QUERY_KEY });
      setStep("cancelled");
    } catch (cancelError) {
      setError(errorMessage(cancelError));
    } finally {
      setBusy(false);
    }
  }

  const linkedMods = result?.mods.filter((mod) => mod.managed).length ?? 0;
  const localMods = (result?.mods.length ?? 0) - linkedMods;
  const progressPercent = Math.round((trackedOperation?.progress ?? 0) * 100);

  return (
    <Modal
      title={t("import_existing_data")}
      className="max-w-2xl"
      closable={step !== "progress"}
      onClose={onClose}
    >
      {step === "detect" && (
        <>
          <div className="modalBody space-y-3">
            <p className="text-sm text-text-muted">{t("existing_data_detect_description")}</p>
            {candidates === undefined && !error ? (
              <Progress indeterminate aria-label={t("detecting_existing_data")} />
            ) : candidates?.length ? (
              <div className="space-y-2">
                {candidates.map((item) => (
                  <Card key={item.path} variant="subtle">
                    <button
                      type="button"
                      aria-label={item.path}
                      className="w-full text-left"
                      disabled={busy}
                      onClick={() => void inspect(item.path)}
                    >
                      <CardContent className="space-y-1">
                        <strong className="block break-all">{item.path}</strong>
                        <span className="text-text-muted">
                          {t("existing_data_candidate_summary", {
                            worlds: item.worldCount,
                            mods: item.modCount,
                            size: formatBytes(item.totalBytes),
                          })}
                        </span>
                      </CardContent>
                    </button>
                  </Card>
                ))}
              </div>
            ) : !error ? (
              <p className="text-sm text-text-muted">{t("no_existing_data_found")}</p>
            ) : null}
            {error && (
              <div className="inlineError" role="alert">
                {error}
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={onClose}>
              {t("cancel")}
            </Button>
            <Button variant="secondary" busy={busy} onClick={() => void chooseFolder()}>
              <FolderOpen size={16} aria-hidden="true" />
              {t("choose_data_folder")}
            </Button>
          </DialogFooter>
        </>
      )}

      {step === "summary" && candidate && (
        <>
          <div className="modalBody space-y-4">
            <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm">
              <dt className="text-text-muted">{t("source_folder")}</dt>
              <dd className="break-all">{candidate.path}</dd>
              <dt className="text-text-muted">{t("detected_version")}</dt>
              <dd>{candidate.detectedGameVersion || t("not_detected")}</dd>
              <dt className="text-text-muted">{t("worlds")}</dt>
              <dd>{candidate.worldCount}</dd>
              <dt className="text-text-muted">{t("mods")}</dt>
              <dd>{candidate.modCount}</dd>
              <dt className="text-text-muted">{t("data_size")}</dt>
              <dd>
                {formatBytes(candidate.totalBytes)} ·{" "}
                {t("files_count", { count: candidate.totalFiles })}
              </dd>
              <dt className="text-text-muted">{t("client_settings")}</dt>
              <dd>{t(candidate.hasClientSettings ? "found" : "not_found")}</dd>
              <dt className="text-text-muted">{t("mod_config")}</dt>
              <dd>{t(candidate.hasModConfig ? "found" : "not_found")}</dd>
            </dl>
            {candidate.warnings.length > 0 && (
              <Card variant="subtle">
                <CardContent>
                  <strong>{t("warnings")}</strong>
                  <ul className="mt-2 list-disc space-y-1 pl-5">
                    {candidate.warnings.map((warning) => (
                      <li key={warning}>{warning}</li>
                    ))}
                  </ul>
                </CardContent>
              </Card>
            )}
            <Field label={t("name")}>
              <Input value={name} onChange={(event) => setName(event.target.value)} />
            </Field>
            <Field label={t("installed_game_version")}>
              <Select value={versionID} onValueChange={setVersionID}>
                <SelectTrigger>
                  <SelectValue placeholder={t("select_installed_version_placeholder")} />
                </SelectTrigger>
                <SelectContent>
                  {versions.map((version) => (
                    <SelectItem key={version.id} value={version.id}>
                      {version.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            {!versionID && (
              <Card variant="subtle">
                <CardContent className="space-y-3">
                  <p>
                    {t("detected_version_not_installed", {
                      version: candidate.detectedGameVersion || t("unknown"),
                    })}
                  </p>
                  <Button
                    variant="secondary"
                    onClick={() => onOpenVersions({ sourcePath: candidate.path, name })}
                  >
                    {t("open_versions_page")}
                  </Button>
                </CardContent>
              </Card>
            )}
            {error && (
              <div className="inlineError" role="alert">
                {error}
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setStep("detect")}>
              {t("back")}
            </Button>
            <Button busy={busy} disabled={!versionID} onClick={() => void startImport()}>
              {t("start_import")}
            </Button>
          </DialogFooter>
        </>
      )}

      {step === "progress" && trackedOperation && (
        <>
          <output className="modalBody space-y-3" aria-live="polite">
            <p>{t("importing_existing_data")}</p>
            <Progress
              value={progressPercent}
              indeterminate={trackedOperation.status === "queued"}
              aria-label={t("import_progress")}
            />
            <p className="text-sm text-text-muted">
              {t("import_progress_percent", { progress: progressPercent })}
            </p>
            {error && (
              <div className="inlineError" role="alert">
                {error}
              </div>
            )}
          </output>
          <DialogFooter>
            <Button variant="secondary" busy={busy} onClick={() => void cancelImport()}>
              {t("cancel_import")}
            </Button>
          </DialogFooter>
        </>
      )}

      {step === "success" && result && (
        <>
          <output className="modalBody space-y-3" aria-live="polite">
            <p>{t("existing_data_import_complete", { name: result.instance?.name ?? name })}</p>
            <p>
              {t("imported_mods_result", {
                count: result.mods.length,
                linked: linkedMods,
                local: localMods,
              })}
            </p>
            <p className="text-sm text-text-muted">{t("original_data_untouched")}</p>
          </output>
          <DialogFooter>
            <Button variant="ghost" onClick={onClose}>
              {t("close")}
            </Button>
            <Button
              disabled={!result.instance}
              onClick={() => result.instance && onOpenInstance(result.instance)}
            >
              {t("open_instance")}
            </Button>
          </DialogFooter>
        </>
      )}

      {(step === "failed" || step === "cancelled") && (
        <>
          <div className="modalBody space-y-3">
            <p>
              {t(
                step === "failed"
                  ? "existing_data_import_failed"
                  : "existing_data_import_cancelled",
              )}
            </p>
            {error && (
              <div className="inlineError" role="alert">
                {error}
              </div>
            )}
            <p className="text-sm text-text-muted">{t("original_data_untouched")}</p>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={onClose}>
              {t("close")}
            </Button>
            <Button onClick={() => setStep("summary")}>{t("retry")}</Button>
          </DialogFooter>
        </>
      )}
    </Modal>
  );
}
