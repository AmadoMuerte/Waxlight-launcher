import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import type { Account } from "../../entities/account/model";
import type { GameVersion } from "../../entities/game-version/model";
import type { Instance } from "../../entities/instance/model";
import { errorMessage } from "../../shared/api/bridge";
import { modCatalogApi } from "../../shared/api/mod-catalog";
import type {
  DownloadedMod,
  InstalledModInstance,
  ModBatchInstallResult,
  ModDetails,
  ModVersion,
} from "../../shared/api/types";
import { Button } from "../../shared/ui/button";
import { Empty } from "../../shared/ui/empty";
import { Modal } from "../../shared/ui/modal";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../shared/ui/select";
import { CreateInstanceModal } from "../instances/CreateInstanceModal";
import { instanceGameVersion, releaseTypeLabel } from "./lib";

interface SelectedMod {
  details: ModDetails;
  release: ModVersion;
  downloaded?: DownloadedMod;
}

interface BatchInstancePickerDialogProps {
  mods: SelectedMod[];
  instances: Instance[];
  gameVersions: GameVersion[];
  accounts: Account[];
  onClose: () => void;
  onCreated: (instance: Instance) => Promise<void>;
  onDone: () => Promise<void>;
}

export function BatchInstancePickerDialog({
  mods,
  instances,
  gameVersions,
  accounts,
  onClose,
  onCreated,
  onDone,
}: BatchInstancePickerDialogProps) {
  const { t } = useTranslation();
  const [instanceID, setInstanceID] = useState("");
  const [creating, setCreating] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [results, setResults] = useState<ModBatchInstallResult[]>();
  const [modsList, setModsList] = useState(mods);
  const instance = instances.find((item) => item.id === instanceID);
  const modNameByID = new Map(mods.map(({ details }) => [details.id, details.name]));
  const installedCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const item of instances) {
      let count = 0;
      for (const mod of modsList) {
        const local = mod.downloaded;
        if (
          local &&
          local.versionId === mod.release.id &&
          local.installedInstances.some((entry) => entry.instanceId === item.id)
        ) {
          count += 1;
        }
      }
      if (count > 0) counts.set(item.id, count);
    }
    return counts;
  }, [instances, modsList]);

  async function install() {
    if (!instance) return;
    setBusy(true);
    setError("");
    try {
      const response = await modCatalogApi.downloadBatch({
        instanceId: instance.id,
        targets: modsList.map(({ details, release }) => ({
          modId: details.id,
          versionId: release.id,
        })),
      });
      setResults(response);
      setModsList((current) =>
        current.map((mod) => {
          const item = response.find(
            (entry) => entry.modId === mod.details.id && entry.versionId === mod.release.id,
          );
          const fresh = item?.error ? undefined : item?.result.downloaded;
          if (!fresh) return mod;
          const instancesById = new Map<string, InstalledModInstance>();
          for (const entry of mod.downloaded?.installedInstances ?? []) {
            instancesById.set(entry.instanceId, entry);
          }
          for (const entry of fresh.installedInstances ?? []) {
            instancesById.set(entry.instanceId, entry);
          }
          for (const installation of item?.result.installations ?? []) {
            if (!installation.installed) continue;
            instancesById.set(installation.instanceId, {
              instanceId: installation.instanceId,
              instanceName: installation.instanceName,
              version: fresh.downloadedVersion,
              enabled: true,
            });
          }
          return {
            ...mod,
            downloaded: { ...fresh, installedInstances: [...instancesById.values()] },
          };
        }),
      );
      await onDone();
    } catch (installError) {
      setError(errorMessage(installError));
    } finally {
      setBusy(false);
    }
  }

  if (creating) {
    return (
      <CreateInstanceModal
        versions={gameVersions}
        accounts={accounts}
        onClose={() => setCreating(false)}
        onDone={async (created) => {
          await onCreated(created);
          setInstanceID(created.id);
          setCreating(false);
        }}
      />
    );
  }

  return (
    <Modal title={t("add_mods_to_instance")} className="batchModDialog" onClose={onClose}>
      <div className="batchModLayout">
        <section className="batchModMain">
          <p className="muted">{t("choose_instance_for_mods", { count: mods.length })}</p>
          {instances.length === 0 ? (
            <Empty
              icon="◌"
              title={t("no_instances_available")}
              description={t("create_instance_before_mods")}
            />
          ) : (
            <div className="instanceChoices">
              {instances.map((item) => {
                const installedCount = installedCounts.get(item.id) ?? 0;
                const fullyInstalled = installedCount === modsList.length;
                const partiallyInstalled = installedCount > 0 && !fullyInstalled;
                const installedVersion = modsList[0]?.downloaded?.installedInstances.find(
                  (entry) => entry.instanceId === item.id,
                )?.version;
                return (
                  <label
                    key={item.id}
                    className={`instanceChoice ${item.id === instanceID ? "selected" : ""} ${
                      fullyInstalled ? "installed" : ""
                    }`}
                    aria-label={item.name}
                  >
                    {fullyInstalled ? (
                      <span className="installedCheck" aria-hidden="true">
                        ✓
                      </span>
                    ) : (
                      <input
                        type="radio"
                        name="batch-instance"
                        checked={item.id === instanceID}
                        onChange={() => setInstanceID(item.id)}
                      />
                    )}
                    <span>
                      <strong>{item.name}</strong>
                      <small>Vintage Story {instanceGameVersion(item, gameVersions)}</small>
                      {fullyInstalled && modsList.length === 1 && installedVersion && (
                        <small className="installedHint">
                          {t("installed_version_value", { version: installedVersion })}
                        </small>
                      )}
                      {partiallyInstalled && (
                        <small className="installedHint">
                          {t("batch_mods_partially_installed", {
                            count: installedCount,
                            total: modsList.length,
                          })}
                        </small>
                      )}
                    </span>
                    {fullyInstalled ? (
                      <span className="installedPill">{t("installed")}</span>
                    ) : null}
                  </label>
                );
              })}
            </div>
          )}
          {error && (
            <div className="inlineError" role="alert">
              {error}
            </div>
          )}
          {busy && <p className="batchDownloading">{t("downloading_mods")}</p>}
          {results && (
            <div className="batchInstallResults">
              {results.map((result) => {
                const failedInstallations = (result.result?.installations ?? []).filter(
                  (item) => !item.installed,
                );
                const failed = Boolean(result.error) || failedInstallations.length > 0;
                const message =
                  result.error ||
                  failedInstallations
                    .map((item) => item.message)
                    .filter(Boolean)
                    .join("; ") ||
                  (failed ? t("mod_install_failed") : t("mod_installed"));
                return (
                  <p
                    key={`${result.modId}:${result.versionId}`}
                    className={failed ? "resultError" : "resultOk"}
                  >
                    <strong>{modNameByID.get(result.modId) ?? result.modId}</strong>: {message}
                  </p>
                );
              })}
            </div>
          )}
          <div className="modalActions">
            <Button variant="secondary" onClick={() => setCreating(true)}>
              {t("create_new_instance")}
            </Button>
            <Button variant="ghost" onClick={onClose}>
              {t("cancel")}
            </Button>
            <Button busy={busy} disabled={!instance} onClick={() => void install()}>
              {t("add_to_instance")}
            </Button>
          </div>
        </section>
        <aside className="batchModList" aria-label={t("selected_mods")}>
          <h3>{t("selected_mods")}</h3>
          {modsList.map(({ details, release }) => {
            const incompatible =
              instance &&
              !release.gameVersions.includes(instanceGameVersion(instance, gameVersions));
            return (
              <div
                key={details.id}
                className={`batchModItem ${incompatible ? "incompatible" : ""}`}
              >
                <strong>{details.name}</strong>
                <Select
                  value={release.id}
                  onValueChange={(releaseId) => {
                    const nextRelease = details.versions.find(
                      (version) => version.id === releaseId,
                    );
                    if (!nextRelease) return;
                    setModsList((current) =>
                      current.map((mod) =>
                        mod.details.id === details.id ? { ...mod, release: nextRelease } : mod,
                      ),
                    );
                  }}
                >
                  <SelectTrigger aria-label={t("update_to_version", { version: details.name })}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {details.versions.map((version) => (
                      <SelectItem key={version.id} value={version.id}>
                        {version.version} · {releaseTypeLabel(version.releaseType)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {incompatible && <span>{t("mod_version_mismatch")}</span>}
              </div>
            );
          })}
        </aside>
      </div>
    </Modal>
  );
}
