import { useState } from "react";
import { useTranslation } from "react-i18next";

import { errorMessage } from "../../shared/api/bridge";
import { versionsApi } from "../../shared/api/game-versions";
import { settingsApi } from "../../shared/api/settings";
import { Button } from "../../shared/ui/button";
import { Field } from "../../shared/ui/field";
import { Modal } from "../../shared/ui/modal";
import { SubmitForm } from "../../shared/ui/submit-form";

interface InstallLocalVersionModalProps {
  installedVersionIDs: Set<string>;
  onClose: () => void;
  onDone: () => Promise<void>;
}

export function InstallLocalVersionModal({
  installedVersionIDs,
  onClose,
  onDone,
}: InstallLocalVersionModalProps) {
  const { t } = useTranslation();
  const [id, setID] = useState("");
  const [name, setName] = useState("");
  const [sourcePath, setSourcePath] = useState("");
  const [executablePath, setExecutablePath] = useState("");
  const [checksum, setChecksum] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const duplicateID = installedVersionIDs.has(id.trim());

  async function install() {
    if (duplicateID) {
      setError(t("version_already_installed"));
      return;
    }
    setBusy(true);
    setError("");
    try {
      await versionsApi.install({
        id,
        name,
        sourcePath,
        executableRelativePath: executablePath,
        expectedSha256: checksum,
      });
      await onDone();
    } catch (installError) {
      setError(errorMessage(installError));
    } finally {
      setBusy(false);
    }
  }

  async function selectArchive() {
    const selectedPath = await settingsApi.selectGameArchive();
    if (selectedPath) {
      setSourcePath(selectedPath);
    }
  }

  async function selectDirectory() {
    const selectedPath = await settingsApi.selectGameDirectory();
    if (selectedPath) {
      setSourcePath(selectedPath);
    }
  }

  return (
    <Modal title={t("install_local_version")} className="installVersionDialog" onClose={onClose}>
      <SubmitForm className="dialogForm" onSubmit={install}>
        <div className="modalBody formFields">
          <div className="notice">
            <b>{t("local_installation")}</b>
            <span>{t("local_installation_description")}</span>
          </div>
          <div className="formRow">
            <Field label={t("version_id")}>
              <input
                required
                value={id}
                onChange={(event) => setID(event.target.value)}
                placeholder="1.22.6"
                aria-invalid={duplicateID}
              />
            </Field>
            <Field label={t("display_name")}>
              <input
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="Vintage Story 1.22.6"
              />
            </Field>
          </div>
          {duplicateID && (
            <div className="inlineError">
              {t("version_already_installed_detail", { id: id.trim() })}
            </div>
          )}
          <Field label={t("archive_or_directory")} hint={t("supported_archives_hint")}>
            <div className="inputAction">
              <input
                required
                value={sourcePath}
                onChange={(event) => setSourcePath(event.target.value)}
                placeholder="/path/to/vs_client_linux-x64_1.22.6.tar.gz"
              />
              <Button type="button" variant="secondary" onClick={() => void selectArchive()}>
                {t("choose_archive")}
              </Button>
              <Button type="button" variant="ghost" onClick={() => void selectDirectory()}>
                {t("directory")}
              </Button>
            </div>
          </Field>
          <Field label={t("executable_path_inside_archive")} hint={t("locate_executable_hint")}>
            <input
              value={executablePath}
              onChange={(event) => setExecutablePath(event.target.value)}
              placeholder="Vintagestory"
            />
          </Field>
          <Field label={t("sha256_checksum_optional")} hint={t("checksum_mismatch_hint")}>
            <input
              value={checksum}
              onChange={(event) => setChecksum(event.target.value)}
              placeholder={t("hexadecimal_characters_64")}
            />
          </Field>
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
          <Button busy={busy} disabled={duplicateID}>
            {t("install")}
          </Button>
        </div>
      </SubmitForm>
    </Modal>
  );
}
