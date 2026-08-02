import { useState } from "react";

import { settingsApi, versionsApi } from "../../shared/api";
import { errorMessage } from "../../shared/api/bridge";
import {
  Button,
  Field,
  Modal,
  SubmitForm,
} from "../../shared/ui";

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
      setError("This game version is already installed.");
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
    <Modal title="Install a local version" onClose={onClose}>
      <SubmitForm className="form" onSubmit={install}>
        <div className="notice">
          <b>Local installation</b>
          <span>
            Use this for a distribution you already downloaded. Waxlight copies
            it into the shared version store.
          </span>
        </div>
        <div className="formRow">
          <Field label="Version ID">
            <input
              autoFocus
              required
              value={id}
              onChange={(event) => setID(event.target.value)}
              placeholder="1.22.6"
              aria-invalid={duplicateID}
            />
          </Field>
          <Field label="Display name">
            <input
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="Vintage Story 1.22.6"
            />
          </Field>
        </div>
        {duplicateID && (
          <div className="inlineError">
            Version {id.trim()} is already installed.
          </div>
        )}
        <Field
          label="Archive or directory"
          hint="Supported archives: .zip, .tar.gz, and .tgz."
        >
          <div className="inputAction">
            <input
              required
              value={sourcePath}
              onChange={(event) => setSourcePath(event.target.value)}
              placeholder="/path/to/vs_client_linux-x64_1.22.6.tar.gz"
            />
            <Button type="button" variant="secondary" onClick={() => void selectArchive()}>
              Choose archive
            </Button>
            <Button type="button" variant="ghost" onClick={() => void selectDirectory()}>
              Directory
            </Button>
          </div>
        </Field>
        <Field
          label="Executable path inside the archive"
          hint="Leave empty to locate Vintagestory automatically."
        >
          <input
            value={executablePath}
            onChange={(event) => setExecutablePath(event.target.value)}
            placeholder="Vintagestory"
          />
        </Field>
        <Field
          label="SHA-256 checksum (optional)"
          hint="Installation stops if the checksum does not match."
        >
          <input
            value={checksum}
            onChange={(event) => setChecksum(event.target.value)}
            placeholder="64 hexadecimal characters"
          />
        </Field>
        {error && <div className="inlineError">{error}</div>}
        <div className="modalActions">
          <Button type="button" variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button busy={busy} disabled={duplicateID}>
            Install
          </Button>
        </div>
      </SubmitForm>
    </Modal>
  );
}
