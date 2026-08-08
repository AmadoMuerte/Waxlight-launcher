import { useState } from "react";
import { useTranslation } from "react-i18next";

import { useToastStore } from "../../app/stores/toast";
import { modsApi } from "../../entities/mod/api";
import type { InstanceModUpdateReport, ModUpdate } from "../../entities/mod/model";
import { errorMessage } from "../../shared/api/bridge";
import { Button } from "../../shared/ui/button";
import { Checkbox } from "../../shared/ui/checkbox-control";
import { Modal } from "../../shared/ui/modal";
import { plainText } from "./lib";

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
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const updates = report.mods.filter((mod) => mod.status === "update_available");
  const pending = updates.filter((mod) => mod.compatible || allowIncompatible);
  const skipped = updates.filter((mod) => !mod.compatible && !allowIncompatible);
  const notUpdatable = report.summary.notUpdatableLocal + report.summary.notUpdatableAbsent;

  async function applyUpdates() {
    setBusy(true);
    setError("");
    try {
      // One coordinated backend operation: the backend creates a single
      // safety snapshot first and then applies every update.
      await modsApi.updateInstance({
        instanceId,
        mods: pending.map((mod) => ({ modId: mod.modId, versionId: mod.targetVersionId })),
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
          {report.gameVersion
            ? ` · ${t("update_incompatible_with", { version: report.gameVersion })}`
            : ""}
        </p>

        {updates.length === 0 ? (
          <div className="inlineNotice">{t("no_mod_updates")}</div>
        ) : (
          <ul className="modUpdateList">
            {updates.map((mod) => (
              <ModUpdateRow key={mod.modId} mod={mod} gameVersion={report.gameVersion} />
            ))}
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

        {updates.some((mod) => !mod.compatible) && (
          <Checkbox
            label={t("allow_incompatible_mod_updates")}
            checked={allowIncompatible}
            onChange={(event) => setAllowIncompatible(event.target.checked)}
          />
        )}

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

function ModUpdateRow({ mod, gameVersion }: { mod: ModUpdate; gameVersion: string }) {
  const { t } = useTranslation();
  return (
    <li className="modUpdateRow">
      <div className="modUpdateHeader">
        <strong>{mod.name}</strong>
        <span className="modUpdateVersions">
          {t("mod_update_versions", {
            installed: mod.installedVersion,
            latest: mod.targetVersion,
          })}
        </span>
      </div>

      {(!mod.compatible || mod.prerelease) && (
        <div className="modUpdateTags">
          {!mod.compatible && (
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
