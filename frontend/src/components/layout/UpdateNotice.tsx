import { useTranslation } from "react-i18next";

import { Progress } from "@/components/ui/progress";

import type { LauncherUpdate, LauncherUpdateProgress } from "../../shared/api";
import { Button } from "../../shared/ui";

interface UpdateNoticeProps {
  update: LauncherUpdate;
  platform: string;
  installingUpdate: boolean;
  updateProgress?: LauncherUpdateProgress;
  onInstall: () => void;
  onOpenRelease: () => void;
  onSkip: () => void;
  onDismiss: () => void;
}

export function UpdateNotice({
  update,
  platform,
  installingUpdate,
  updateProgress,
  onInstall,
  onOpenRelease,
  onSkip,
  onDismiss,
}: UpdateNoticeProps) {
  const { t } = useTranslation();

  const isWindowsPortable = update.installationMode === "portable" && platform === "windows";

  return (
    <section className="launcherUpdateNotice" aria-label={t("update_available")}>
      <div>
        <span className="eyebrow">
          {update.downgrade ? t("downgrade_available") : t("update_available")}
        </span>

        <strong>
          {t("launcher_update_versions", {
            installed: update.installedVersion,
            latest: update.version,
          })}
        </strong>

        <p>{update.releaseNotes || t("release_notes_unavailable")}</p>

        {isWindowsPortable && <p className="updateHint">{t("portable_update_hint")}</p>}

        {installingUpdate && updateProgress && (
          <div className="launcherUpdateProgress">
            <Progress max={1} value={updateProgress.progress} />
            <small>{t(`update_phase_${updateProgress.phase}`)}</small>
          </div>
        )}
      </div>

      <div className="launcherUpdateActions">
        {isWindowsPortable ? (
          <Button type="button" disabled={installingUpdate} onClick={onOpenRelease}>
            {t("download_update")}
          </Button>
        ) : (
          <Button type="button" busy={installingUpdate} onClick={onInstall}>
            {t("download_and_install_update")}
          </Button>
        )}

        <Button
          type="button"
          variant="secondary"
          disabled={installingUpdate}
          onClick={onOpenRelease}
        >
          {t("view_full_release")}
        </Button>

        <Button type="button" variant="ghost" disabled={installingUpdate} onClick={onDismiss}>
          {t("later")}
        </Button>

        <Button type="button" variant="ghost" disabled={installingUpdate} onClick={onSkip}>
          {t("skip_this_version")}
        </Button>
      </div>
    </section>
  );
}
