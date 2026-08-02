import { useState } from "react";

import {
  settingsApi,
  versionsApi,
  type GameVersion,
} from "../shared/api";
import { errorMessage } from "../shared/api/bridge";
import { formatBytes, formatDate } from "../shared/lib";
import {
  Button,
  Empty,
  Field,
  Modal,
  PageHeader,
  StatusPill,
  SubmitForm,
} from "../shared/ui";

interface VersionsPageProps {
  versions: GameVersion[];
  refresh: () => Promise<void>;
  notify: (message: string, type?: "ok" | "error") => void;
}

export function VersionsPage({
  versions,
  refresh,
  notify,
}: VersionsPageProps) {
  const [installDialogOpen, setInstallDialogOpen] = useState(false);

  async function removeVersion(version: GameVersion) {
    const confirmed = window.confirm(
      `Remove Vintage Story ${version.name} and its installed files?`,
    );
    if (!confirmed) {
      return;
    }

    try {
      await versionsApi.remove(version.id, true);
      await refresh();
      notify("Game version removed");
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  return (
    <>
      <PageHeader
        eyebrow="Game runtime"
        title="Game versions"
        description="Keep multiple Vintage Story versions in one tidy library."
        action={
          <Button onClick={() => setInstallDialogOpen(true)}>
            ＋ Add version
          </Button>
        }
      />

      {versions.length === 0 ? (
        <Empty
          icon="⬡"
          title="No installed versions"
          description="Choose a local ZIP, TAR.GZ, TGZ, or an extracted Vintage Story directory."
          action={
            <Button onClick={() => setInstallDialogOpen(true)}>
              Install from file
            </Button>
          }
        />
      ) : (
        <div className="table versionTable">
          <div className="tableHead">
            <span>Version</span>
            <span>Platform</span>
            <span>Size</span>
            <span>Installed</span>
            <span />
          </div>

          {versions.map((version) => (
            <div className="tableRow" key={version.id}>
              <span className="versionIdentity">
                <strong>{version.name}</strong>
                <small>{version.id}</small>
                <small title={version.executablePath}>
                  {version.executablePath}
                </small>
              </span>
              <span>
                {version.platform} · {version.architecture}
              </span>
              <span>{formatBytes(version.sizeBytes)}</span>
              <span>{formatDate(version.installedAt)}</span>
              <span className="row tableActions">
                <StatusPill status={version.status} />
                <Button
                  variant="ghost"
                  onClick={() => void removeVersion(version)}
                >
                  Remove
                </Button>
              </span>
            </div>
          ))}
        </div>
      )}

      {installDialogOpen && (
        <InstallVersionModal
          installedVersionIDs={new Set(versions.map((version) => version.id))}
          onClose={() => setInstallDialogOpen(false)}
          onDone={async () => {
            setInstallDialogOpen(false);
            await refresh();
            notify("Game version installed");
          }}
        />
      )}
    </>
  );
}

interface InstallVersionModalProps {
  installedVersionIDs: Set<string>;
  onClose: () => void;
  onDone: () => Promise<void>;
}

function InstallVersionModal({
  installedVersionIDs,
  onClose,
  onDone,
}: InstallVersionModalProps) {
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
    <Modal title="Add game version" onClose={onClose}>
      <SubmitForm className="form" onSubmit={install}>
        <div className="notice">
          <b>Local installation</b>
          <span>
            Waxlight copies the selected distribution into its version store and
            automatically looks for the Vintagestory executable.
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
            Version {id.trim()} is already installed. Remove it before adding it
            again.
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
            <Button
              type="button"
              variant="secondary"
              onClick={() => void selectArchive()}
            >
              Choose archive
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => void selectDirectory()}
            >
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
