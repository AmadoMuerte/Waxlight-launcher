import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { useToastStore } from "../../app/stores/toast";
import { versionsApi } from "../../entities/game-version/api";
import { useGameVersionsQuery } from "../../entities/game-version/queries";
import { AvailableVersions } from "../../features/install-game-version/AvailableVersions";
import { InstallLocalVersionModal } from "../../features/install-game-version/InstallLocalVersionModal";
import { errorMessage } from "../../shared/api/bridge";
import { GAME_VERSIONS_QUERY_KEY, OPERATIONS_QUERY_KEY } from "../../shared/api/keys";
import { formatBytes, formatDate } from "../../shared/lib";
import { Button } from "../../shared/ui/button";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { PageHeader } from "../../shared/ui/page-header";
import { StatusPill } from "../../shared/ui/status-pill";

export function VersionsPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  const { data: versions = [] } = useGameVersionsQuery();
  const [installDialogOpen, setInstallDialogOpen] = useState(false);
  const [confirmState, setConfirmState] = useState<{
    open: boolean;
    title: string;
    message?: string;
    onConfirm: () => void;
  }>({ open: false, title: "", onConfirm: () => {} });

  function removeVersion(version: (typeof versions)[number]) {
    setConfirmState({
      open: true,
      title: t("remove_version_confirmation", { name: version.name }),
      onConfirm: async () => {
        try {
          await versionsApi.remove(version.id, true);
          await queryClient.invalidateQueries({ queryKey: GAME_VERSIONS_QUERY_KEY });
          await queryClient.invalidateQueries({ queryKey: OPERATIONS_QUERY_KEY });
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

      <AvailableVersions installedVersionIDs={versions.map((version) => version.id)} />

      {installDialogOpen && (
        <InstallLocalVersionModal
          installedVersionIDs={new Set(versions.map((version) => version.id))}
          onClose={() => setInstallDialogOpen(false)}
          onDone={async () => {
            setInstallDialogOpen(false);
            await queryClient.invalidateQueries({ queryKey: GAME_VERSIONS_QUERY_KEY });
            await queryClient.invalidateQueries({ queryKey: OPERATIONS_QUERY_KEY });
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
