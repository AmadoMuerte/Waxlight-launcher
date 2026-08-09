import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { useToastStore } from "../../app/stores/toast";
import { modCatalogApi, modsApi } from "../../entities/mod/api";
import type { InstanceModUpdateReport, ModUpdate, ModVersion } from "../../entities/mod/model";
import { errorMessage } from "../../shared/api/bridge";
import { Button } from "../../shared/ui/button";
import { Checkbox } from "../../shared/ui/checkbox-control";
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
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    const updateable = report.mods.filter((mod) => mod.status === "update_available");
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
      if (active) setVersionsByModId(Object.fromEntries(entries));
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
    setBusy(true);
    setError("");
    try {
      // One coordinated backend operation: the backend creates a single
      // safety snapshot first and then applies every update.
      await modsApi.updateInstance({
        instanceId,
        mods: pending.map((mod) => ({
          modId: mod.modId,
          versionId: versionIds[mod.modId] ?? mod.targetVersionId,
        })),
        allowIncompatible,
      });
      await onApplied();
      notify(t("mod_updates_applied"));
      onClose();
    } catch (applyError) {
      setError(errorMessage(applyError));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal title={t("update_mods")} className="modUpdatesDialog" onClose={onClose}>
      <div className="modalBody formFields">
        <p className="muted">
          {t("mod_updates_for", { name: instanceName })}
          {report.gameVersion ? ` · ${t("vintage_story")} ${report.gameVersion}` : ""}
        </p>

        {updates.length === 0 ? (
          <div className="inlineNotice">{t("no_mod_updates")}</div>
        ) : (
          <ul className="modUpdateList">
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

        <Checkbox
          label={t("allow_incompatible_mod_updates")}
          checked={allowIncompatible}
          onChange={(event) => setAllowIncompatible(event.target.checked)}
        />

        {error && (
          <div className="inlineError" role="alert">
            {error}
          </div>
        )}
      </div>

      <div className="dialogFooter">
        <Button type="button" variant="ghost" onClick={onClose} disabled={busy}>
          {t("cancel")}
        </Button>
        <Button
          type="button"
          busy={busy}
          disabled={updates.length === 0 || pending.length === 0}
          onClick={() => void applyUpdates()}
        >
          {t("update_mods_count", { count: pending.length })}
        </Button>
      </div>
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
    <li className="modUpdateRow">
      <div className="modUpdateHeader">
        <Checkbox
          label={mod.name}
          checked={selected}
          onChange={(event) => onSelectedChange(event.target.checked)}
        />
        {versions.length > 0 ? (
          <Select value={selectedVersionId} onValueChange={onVersionChange}>
            <SelectTrigger
              className="modUpdateVersions"
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
          <span className="modUpdateVersions">
            {t("mod_update_versions", {
              installed: mod.installedVersion,
              latest: mod.targetVersion,
            })}
          </span>
        )}
      </div>

      {(!compatible || mod.prerelease) && (
        <div className="modUpdateTags">
          {!compatible && (
            <span className="tag tagDanger">
              {t("update_incompatible_with", { version: gameVersion })}
            </span>
          )}
          {mod.prerelease && <span className="tag">{t("mod_update_prerelease")}</span>}
        </div>
      )}

      {mod.changelog && (
        <details className="modUpdateChangelog">
          <summary>{t("mod_update_changelog")}</summary>
          <p>{plainText(mod.changelog)}</p>
        </details>
      )}

      {mod.addedDeps.length > 0 && (
        <p className="modUpdateDeps">
          {t("dependencies_added", {
            names: mod.addedDeps.map((dep) => dep.name || dep.modId).join(", "),
          })}
        </p>
      )}
      {mod.removedDeps.length > 0 && (
        <p className="modUpdateDeps">
          {t("dependencies_removed", {
            names: mod.removedDeps.map((dep) => dep.name || dep.modId).join(", "),
          })}
        </p>
      )}
    </li>
  );
}
