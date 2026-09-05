import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { useToastStore } from "../../app/stores/toast";
import { modCatalogApi, modsApi } from "../../entities/mod/api";
import type { InstanceModUpdateReport, ModUpdate, ModVersion } from "../../entities/mod/model";
import { errorMessage } from "../../shared/api/bridge";
import { Button } from "../../shared/ui/button";
import { Checkbox } from "../../shared/ui/checkbox-control";
import { DialogFooter } from "../../shared/ui/dialog";
import { LoadingState } from "../../shared/ui/loading-state";
import { Modal } from "../../shared/ui/modal";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../shared/ui/select";
import { plainText, releaseTypeLabel } from "./lib";

interface ModUpdatesModalProps {
  instanceId: string;
  instanceName: string;
  report: InstanceModUpdateReport;
  onClose: () => void;
  onApplied: () => Promise<void>;
}

export function ModUpdatesModal({
  instanceId,
  instanceName,
  report,
  onClose,
  onApplied,
}: ModUpdatesModalProps) {
  const { t } = useTranslation();
  const notify = useToastStore((state) => state.notify);
  const [allowIncompatible, setAllowIncompatible] = useState(false);
  const updates = report.mods.filter((mod) => mod.status === "update_available");
  const [selectedModIds, setSelectedModIds] = useState(
    () => new Set(updates.filter((mod) => mod.compatible).map((mod) => mod.modId)),
  );
  const [versionIds, setVersionIds] = useState<Record<string, string>>(() =>
    Object.fromEntries(updates.map((mod) => [mod.modId, mod.targetVersionId])),
  );
  const [versionsByModId, setVersionsByModId] = useState<Record<string, ModVersion[]>>({});
  const [versionsLoading, setVersionsLoading] = useState(updates.length > 0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const applyingRef = useRef(false);

  useEffect(() => {
    let active = true;
    const updateable = report.mods.filter((mod) => mod.status === "update_available");
    setVersionsLoading(updateable.length > 0);
    void (async () => {
      const entries = await Promise.all(
        updateable.map(async (mod) => {
          try {
            const details = await modCatalogApi.get(mod.modId);
            return [mod.modId, details.versions] as const;
          } catch {
            return [mod.modId, []] as const;
          }
        }),
      );
      if (active) {
        setVersionsByModId(Object.fromEntries(entries));
        setVersionsLoading(false);
      }
    })();
    return () => {
      active = false;
    };
  }, [report]);

  function selectedVersionIsCompatible(mod: ModUpdate) {
    const version = versionsByModId[mod.modId]?.find(
      (item) => item.id === (versionIds[mod.modId] ?? mod.targetVersionId),
    );
    return version ? version.gameVersions.includes(report.gameVersion) : mod.compatible;
  }

  const pending = updates.filter(
    (mod) =>
      selectedModIds.has(mod.modId) && (selectedVersionIsCompatible(mod) || allowIncompatible),
  );
  const skipped = updates.filter(
    (mod) =>
      selectedModIds.has(mod.modId) && !selectedVersionIsCompatible(mod) && !allowIncompatible,
  );
  const notUpdatable = report.summary.notUpdatableLocal + report.summary.notUpdatableAbsent;

  async function applyUpdates() {
    if (versionsLoading || applyingRef.current || pending.length === 0) return;
    applyingRef.current = true;
    setBusy(true);
    setError("");
    try {
      // One coordinated backend operation: the backend creates a single
      // safety snapshot first and then applies every update.
      const result = await modsApi.updateInstance({
        instanceId,
        mods: pending.map((mod) => ({
          modId: mod.modId,
          versionId: versionIds[mod.modId] ?? mod.targetVersionId,
        })),
        allowIncompatible,
      });
      await onApplied();
      notify(
        (result.skippedByPolicy ?? 0) > 0
          ? `${t("mods_updated_count", { count: result.updated })} · ${t("mods_skipped_by_update_policy", { count: result.skippedByPolicy })}`
          : t("mod_updates_applied"),
      );
      onClose();
    } catch (applyError) {
      setError(errorMessage(applyError));
    } finally {
      applyingRef.current = false;
      setBusy(false);
    }
  }

  return (
    <Modal title={t("update_mods")} className="w-[min(720px,calc(100vw-48px))]" onClose={onClose}>
      <div className="max-h-[60vh] overflow-y-auto p-6">
        {versionsLoading ? (
          <LoadingState>{t("loading_mods")}</LoadingState>
        ) : (
          <div className="space-y-4">
            <p className="muted">
              {t("mod_updates_for", { name: instanceName })}
              {report.gameVersion ? ` · ${t("vintage_story")} ${report.gameVersion}` : ""}
            </p>

            {updates.length === 0 ? (
              <div className="inlineNotice">{t("no_mod_updates")}</div>
            ) : (
              <ul className="divide-y divide-border-subtle">
                {updates.map((mod) => {
                  const selectedVersionId = versionIds[mod.modId] ?? mod.targetVersionId;
                  const versions = (versionsByModId[mod.modId] ?? []).filter(
                    (version) => version.version !== mod.installedVersion,
                  );
                  return (
                    <ModUpdateRow
                      key={mod.modId}
                      mod={mod}
                      gameVersion={report.gameVersion}
                      versions={versions}
                      selected={selectedModIds.has(mod.modId)}
                      selectedVersionId={selectedVersionId}
                      compatible={selectedVersionIsCompatible(mod)}
                      onSelectedChange={(selected) => {
                        setSelectedModIds((current) => {
                          const next = new Set(current);
                          if (selected) next.add(mod.modId);
                          else next.delete(mod.modId);
                          return next;
                        });
                      }}
                      onVersionChange={(versionId) =>
                        setVersionIds((current) => ({ ...current, [mod.modId]: versionId }))
                      }
                    />
                  );
                })}
              </ul>
            )}

            {skipped.length > 0 && (
              <div className="inlineNotice warning">
                {t("mod_updates_skipped", { count: skipped.length })}
              </div>
            )}

            {notUpdatable > 0 && (
              <div className="inlineNotice">
                {t("mods_not_updatable_hint", { count: notUpdatable })}
              </div>
            )}

            {error && (
              <div className="inlineError" role="alert">
                {error}
              </div>
            )}
          </div>
        )}
      </div>

      <DialogFooter>
        <Checkbox
          label={t("allow_incompatible_mod_updates")}
          title={t("allow_incompatible_mod_updates")}
          checked={allowIncompatible}
          onChange={(event) => setAllowIncompatible(event.target.checked)}
        />
        <div className="row">
          <Button type="button" variant="ghost" onClick={onClose} disabled={busy}>
            {t("cancel")}
          </Button>
          <Button
            type="button"
            busy={busy}
            disabled={versionsLoading || updates.length === 0 || pending.length === 0}
            onClick={() => void applyUpdates()}
          >
            {versionsLoading
              ? t("loading_mods")
              : t("update_mods_count", { count: pending.length })}
          </Button>
        </div>
      </DialogFooter>
    </Modal>
  );
}

function ModUpdateRow({
  mod,
  gameVersion,
  versions,
  selected,
  selectedVersionId,
  compatible,
  onSelectedChange,
  onVersionChange,
}: {
  mod: ModUpdate;
  gameVersion: string;
  versions: ModVersion[];
  selected: boolean;
  selectedVersionId: string;
  compatible: boolean;
  onSelectedChange: (selected: boolean) => void;
  onVersionChange: (versionId: string) => void;
}) {
  const { t } = useTranslation();
  return (
    <li className="py-4">
      <div className="flex flex-wrap items-center gap-3">
        <Checkbox
          label={mod.name}
          checked={selected}
          onChange={(event) => onSelectedChange(event.target.checked)}
        />
        {versions.length > 0 ? (
          <Select value={selectedVersionId} onValueChange={onVersionChange}>
            <SelectTrigger
              className="h-8 w-auto min-w-44 text-xs"
              aria-label={t("update_to_version", { version: mod.name })}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {versions.map((version) => (
                <SelectItem key={version.id} value={version.id}>
                  {t("mod_update_versions", {
                    installed: mod.installedVersion,
                    latest: version.version,
                  })}{" "}
                  · {releaseTypeLabel(version.releaseType)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : (
          <span className="text-sm text-text-secondary">
            {t("mod_update_versions", {
              installed: mod.installedVersion,
              latest: mod.targetVersion,
            })}
          </span>
        )}
      </div>

      {(!compatible || mod.prerelease) && (
        <div className="mt-2 flex flex-wrap gap-2">
          {!compatible && (
            <span className="rounded-full border border-danger-border bg-danger-surface px-2 py-0.5 text-[length:var(--fs-label)] font-semibold text-danger-foreground">
              {t("update_incompatible_with", { version: gameVersion })}
            </span>
          )}
          {mod.prerelease && (
            <span className="rounded-full border border-border-default bg-surface-3 px-2 py-0.5 text-[length:var(--fs-label)] font-semibold text-text-secondary">
              {t("mod_update_prerelease")}
            </span>
          )}
        </div>
      )}

      {mod.changelog && (
        <details className="mt-2">
          <summary className="cursor-pointer text-xs font-semibold text-text-muted transition-colors hover:text-text-primary">
            {t("mod_update_changelog")}
          </summary>
          <p className="mt-2 text-xs leading-5 text-text-muted">{plainText(mod.changelog)}</p>
        </details>
      )}

      {mod.addedDeps.length > 0 && (
        <p className="mt-2 text-xs leading-5 text-text-muted">
          {t("dependencies_added", {
            names: mod.addedDeps.map((dep) => dep.name || dep.modId).join(", "),
          })}
        </p>
      )}
      {mod.removedDeps.length > 0 && (
        <p className="mt-2 text-xs leading-5 text-text-muted">
          {t("dependencies_removed", {
            names: mod.removedDeps.map((dep) => dep.name || dep.modId).join(", "),
          })}
        </p>
      )}
    </li>
  );
}
