import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui/select";

import type { Account } from "../../entities/account/model";
import type { GameVersion } from "../../entities/game-version/model";
import { instancesApi } from "../../entities/instance/api";
import type { Instance } from "../../entities/instance/model";
import { errorMessage } from "../../shared/api/bridge";
import { Button } from "../../shared/ui/button";
import { Field } from "../../shared/ui/field";
import { Modal } from "../../shared/ui/modal";
import { SubmitForm } from "../../shared/ui/submit-form";

function latestInstalledVersion(versions: GameVersion[]): GameVersion | undefined {
  return versions.reduce<GameVersion | undefined>((latest, version) => {
    if (!latest) return version;
    if (latest.channel !== version.channel) {
      return version.channel === "stable" ? version : latest;
    }
    const latestParts = latest.name.match(/\d+/g)?.map(Number) ?? [];
    const versionParts = version.name.match(/\d+/g)?.map(Number) ?? [];
    const length = Math.max(latestParts.length, versionParts.length);
    for (let index = 0; index < length; index += 1) {
      const difference = (versionParts[index] ?? 0) - (latestParts[index] ?? 0);
      if (difference !== 0) return difference > 0 ? version : latest;
    }
    return latest;
  }, undefined);
}

interface CreateInstanceModalProps {
  versions: GameVersion[];
  accounts: Account[];
  onClose: () => void;
  onDone: (instance: Instance) => Promise<void>;
}

export function CreateInstanceModal({
  versions,
  accounts,
  onClose,
  onDone,
}: CreateInstanceModalProps) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [versionID, setVersionID] = useState(() => latestInstalledVersion(versions)?.id ?? "");
  const [accountID, setAccountID] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function createInstance() {
    setBusy(true);
    setError("");
    try {
      const instance = await instancesApi.create({
        name: name.trim(),
        description,
        gameVersionId: versionID,
        defaultAccountId: accountID || undefined,
        directory: "",
        launchArguments: [],
      });
      await onDone(instance);
    } catch (createError) {
      setError(errorMessage(createError));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal title={t("new_instance")} className="createInstanceDialog" onClose={onClose}>
      <SubmitForm className="dialogForm" onSubmit={createInstance}>
        <div className="modalBody formFields">
          <Field label={t("name")}>
            <input
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder={t("default_instance_name")}
            />
          </Field>

          <Field label={t("description")}>
            <textarea
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder={t("instance_description_prompt")}
            />
          </Field>

          <div className="formRow">
            <Field label={t("game_version")}>
              <Select value={versionID} onValueChange={setVersionID}>
                <SelectTrigger>
                  <SelectValue placeholder={t("game_version")} />
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

            <Field label={t("launch_account")}>
              <Select
                value={accountID ? `account:${accountID}` : "global"}
                onValueChange={(value) =>
                  setAccountID(value === "global" ? "" : value.slice("account:".length))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="global">{t("use_globally_selected_account")}</SelectItem>
                  {accounts.map((account) => (
                    <SelectItem key={account.id} value={`account:${account.id}`}>
                      {account.displayName}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
          </div>

          {error && (
            <div className="inlineError" role="alert">
              {error}
            </div>
          )}
        </div>

        <div className="dialogFooter">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("cancel")}
          </Button>
          <Button busy={busy} disabled={versions.length === 0}>
            {t("create")}
          </Button>
        </div>
      </SubmitForm>
    </Modal>
  );
}
