import { useTranslation } from "react-i18next";

import { useAppShellStore } from "../../app/stores/app-shell";
import { useSettingsQuery } from "../../entities/settings/queries";
import { Button } from "../../shared/ui/button";
import { Progress } from "../../shared/ui/progress";

export function UpdateNotice() {
  const { t } = useTranslation();
  const settingsQuery = useSettingsQuery();
  const update = useAppShellStore((state) => state.launcherUpdate);
  const platform = useAppShellStore((state) => state.platform);
  const installingUpdate = useAppShellStore((state) => state.installingUpdate);
  const updateProgress = useAppShellStore((state) => state.updateProgress);
  const installUpdate = useAppShellStore((state) => state.installUpdate);
  const skipUpdate = useAppShellStore((state) => state.skipUpdate);
  const openRelease = useAppShellStore((state) => state.openRelease);
  const dismissUpdate = useAppShellStore((state) => state.dismissUpdate);

  if (!update) {
    return null;
  }

  const channel = settingsQuery.data?.updateChannel ?? "stable";
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
          <Button
            type="button"
            disabled={installingUpdate}
            onClick={() => void openRelease(channel)}
          >
            {t("download_update")}
          </Button>
        ) : (
          <Button type="button" busy={installingUpdate} onClick={() => void installUpdate(channel)}>
            {t("download_and_install_update")}
          </Button>
        )}

        <Button
          type="button"
          variant="secondary"
          disabled={installingUpdate}
          onClick={() => void openRelease(channel)}
        >
          {t("view_full_release")}
        </Button>

        <Button type="button" variant="ghost" disabled={installingUpdate} onClick={dismissUpdate}>
          {t("later")}
        </Button>

        <Button
          type="button"
          variant="ghost"
          disabled={installingUpdate}
          onClick={() => {
            if (settingsQuery.data) {
              void skipUpdate(settingsQuery.data, update.version);
            }
          }}
        >
          {t("skip_this_version")}
        </Button>
      </div>
    </section>
  );
}
