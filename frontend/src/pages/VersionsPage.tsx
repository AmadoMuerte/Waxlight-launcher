import { useState } from "react";
import { useTranslation } from "react-i18next";

import { ConfirmDialog } from "../components/ui/confirm-dialog";
import { AvailableVersions } from "../features/install-game-version/AvailableVersions";
import { InstallLocalVersionModal } from "../features/install-game-version/InstallLocalVersionModal";
import { versionsApi, type GameVersion } from "../shared/api";
import { errorMessage } from "../shared/api/bridge";
import { formatBytes, formatDate } from "../shared/lib";
import { Button, PageHeader, StatusPill } from "../shared/ui";

interface VersionsPageProps {
  versions: GameVersion[];
  refresh: () => Promise<void>;
  notify: (message: string, type?: "ok" | "error") => void;
}

export function VersionsPage({ versions, refresh, notify }: VersionsPageProps) {
  const { t } = useTranslation();
  const [installDialogOpen, setInstallDialogOpen] = useState(false);
  const [confirmState, setConfirmState] = useState<{
    open: boolean;
    title: string;
    message?: string;
    onConfirm: () => void;
  }>({ open: false, title: "", onConfirm: () => {} });

  function removeVersion(version: GameVersion) {
    setConfirmState({
      open: true,
      title: t("remove_version_confirmation", { name: version.name }),
      onConfirm: async () => {
        try {
          await versionsApi.remove(version.id, true);
          await refresh();
          notify(t("game_version_removed"));
        } catch (error) {
          notify(errorMessage(error), "error");
        }
      },
    });
  }

  return (
    <>
      <PageHeader
        eyebrow={t("game_runtime")}
        title={t("game_versions")}
        description={t("versions_description")}
        action={
          <Button variant="secondary" onClick={() => setInstallDialogOpen(true)}>
            {t("install_from_file")}
          </Button>
        }
      />

      {versions.length > 0 && (
        <section className="installedVersions">
          <div className="sectionHeading">
            <div>
              <span className="eyebrow">{t("shared_runtime_library")}</span>
              <h2>{t("installed_versions")}</h2>
            </div>
          </div>
          <div className="table versionTable">
            <div className="tableHead">
              <span>{t("version")}</span>
              <span>{t("platform")}</span>
              <span>{t("size")}</span>
              <span>{t("installed")}</span>
              <span />
            </div>

            {versions.map((version) => (
              <div className="tableRow" key={version.id}>
                <span className="versionIdentity">
                  <strong title={version.executablePath}>{version.name}</strong>
                  <small>{version.id}</small>
                </span>
                <span>
                  {version.platform} · {version.architecture}
                </span>
                <span>{formatBytes(version.sizeBytes)}</span>
                <span>{formatDate(version.installedAt)}</span>
                <span className="row tableActions">
                  <StatusPill status={version.status} />
                  <Button variant="ghost" onClick={() => removeVersion(version)}>
                    {t("remove")}
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
            notify(t("game_version_installed"));
          }}
        />
      )}

      <ConfirmDialog
        open={confirmState.open}
        title={confirmState.title}
        message={confirmState.message}
        destructive
        onConfirm={() => {
          setConfirmState((s) => ({ ...s, open: false }));
          confirmState.onConfirm();
        }}
        onCancel={() => setConfirmState((s) => ({ ...s, open: false }))}
      />
    </>
  );
}
