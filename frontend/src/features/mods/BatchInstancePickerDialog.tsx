import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { Account } from "../../entities/account/model";
import type { GameVersion } from "../../entities/game-version/model";
import type { Instance } from "../../entities/instance/model";
import { errorMessage } from "../../shared/api/bridge";
import { modCatalogApi } from "../../shared/api/mod-catalog";
import type { ModBatchInstallResult, ModDetails, ModVersion } from "../../shared/api/types";
import { Button } from "../../shared/ui/button";
import { Empty } from "../../shared/ui/empty";
import { Modal } from "../../shared/ui/modal";
import { CreateInstanceModal } from "../instances/CreateInstanceModal";
import { instanceGameVersion } from "./lib";

interface SelectedMod {
  details: ModDetails;
  release: ModVersion;
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
  const instance = instances.find((item) => item.id === instanceID);
  const modNameByID = new Map(mods.map(({ details }) => [details.id, details.name]));

  async function install() {
    if (!instance) return;
    setBusy(true);
    setError("");
    try {
      const response = await modCatalogApi.downloadBatch({
        instanceId: instance.id,
        targets: mods.map(({ details, release }) => ({ modId: details.id, versionId: release.id })),
      });
      setResults(response);
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
              {instances.map((item) => (
                <label
                  key={item.id}
                  className={`instanceChoice ${item.id === instanceID ? "selected" : ""}`}
                  aria-label={item.name}
                >
                  <input
                    type="radio"
                    name="batch-instance"
                    checked={item.id === instanceID}
                    onChange={() => setInstanceID(item.id)}
                  />
                  <span>
                    <strong>{item.name}</strong>
                    <small>Vintage Story {instanceGameVersion(item, gameVersions)}</small>
                  </span>
                </label>
              ))}
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
              {results.map((result) => (
                <p
                  key={`${result.modId}:${result.versionId}`}
                  className={result.error ? "resultError" : "resultOk"}
                >
                  <strong>{modNameByID.get(result.modId) ?? result.modId}</strong>:{" "}
                  {result.error || t("mod_installed")}
                </p>
              ))}
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
          {mods.map(({ details, release }) => {
            const incompatible =
              instance &&
              !release.gameVersions.includes(instanceGameVersion(instance, gameVersions));
            const supportedVersion = release.gameVersions.at(-1) || t("compatibility_unknown");
            return (
              <div
                key={details.id}
                className={`batchModItem ${incompatible ? "incompatible" : ""}`}
              >
                <strong>{details.name}</strong>
                <small>{t("last_supported_game_version", { version: supportedVersion })}</small>
                {incompatible && <span>{t("mod_version_mismatch")}</span>}
              </div>
            );
          })}
        </aside>
      </div>
    </Modal>
  );
}
