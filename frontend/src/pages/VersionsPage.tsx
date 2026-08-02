import { useState } from "react";

import { AvailableVersions } from "../features/install-game-version/AvailableVersions";
import { InstallLocalVersionModal } from "../features/install-game-version/InstallLocalVersionModal";
import { versionsApi, type GameVersion } from "../shared/api";
import { errorMessage } from "../shared/api/bridge";
import { formatBytes, formatDate } from "../shared/lib";
import {
  Button,
  PageHeader,
  StatusPill,
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
          <Button variant="secondary" onClick={() => setInstallDialogOpen(true)}>
            Install from file
          </Button>
        }
      />

      {versions.length > 0 && (
        <section className="installedVersions">
          <div className="sectionHeading">
            <div>
              <span className="eyebrow">Shared runtime library</span>
              <h2>Installed versions</h2>
            </div>
          </div>
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
        </section>
      )}

      <AvailableVersions
        installedVersionIDs={versions.map((version) => version.id)}
        notify={notify}
        onOperationStarted={refresh}
      />

      {installDialogOpen && (
        <InstallLocalVersionModal
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
