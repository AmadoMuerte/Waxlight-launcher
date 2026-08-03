import type { KeyboardEvent, MouseEvent } from "react";
import { useTranslation } from "react-i18next";

import type { DownloadedMod, ModSummary } from "../../shared/api";
import { Button } from "../../shared/ui";
import { formatBytes, formatDownloads, relativeDate, sideLabel } from "./lib";
import { ModArtwork } from "./ModArtwork";

interface ModCardProps {
  mod: ModSummary;
  downloaded?: DownloadedMod;
  layout: "grid" | "list";
  onOpen: () => void;
  onInstall: () => void;
  onDelete?: () => void;
}

export function ModCard({
  mod,
  downloaded,
  layout,
  onOpen,
  onInstall,
  onDelete,
}: ModCardProps) {
  const { t } = useTranslation();
  function openFromKeyboard(event: KeyboardEvent<HTMLElement>) {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onOpen();
    }
  }

  function action(event: MouseEvent, callback: () => void) {
    event.stopPropagation();
    callback();
  }

  const actionLabel = downloaded
    ? downloaded.updateAvailable
      ? t("update")
      : downloaded.installedInstances.length > 0
        ? t("install_to_another")
        : t("install_to_instance")
    : mod.updateAvailable
      ? t("update")
      : mod.isDownloaded
        ? t("install_to_instance")
        : t("download");

  return (
    <article
      className={`modCard modCard-${layout}`}
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={openFromKeyboard}
      aria-label={t("open_mod", { name: mod.name })}
    >
      <ModArtwork src={downloaded?.imageUrl ?? mod.imageUrl} alt={t("cover_alt", { name: mod.name })} />
      <div className="modCardBody">
        <div className="modCardTitle">
          <div>
            <h3>{mod.name}</h3>
            <span>{t("by_author", { name: mod.authorName })}</span>
          </div>
          <span className={`sideBadge side-${mod.side}`}>{sideLabel(mod.side)}</span>
        </div>

        {downloaded ? (
          <p className="modSummary">
            {t("version_with_size", { version: downloaded.downloadedVersion, size: formatBytes(downloaded.fileSize) })}
          </p>
        ) : (
          <p className="modSummary">{mod.summary || t("no_description_provided")}</p>
        )}

        <div className="modCardMeta">
          <span>{mod.gameVersions.slice(-2).join(", ") || t("versions_in_details")}</span>
          <span>{t("updated_relative", { date: relativeDate(mod.updatedAt) })}</span>
          <span>↓ {formatDownloads(mod.downloads)}</span>
        </div>

        <div className="modCardFooter">
          <div className="modStateText">
            {downloaded?.updateAvailable ? (
              <span className="updateBadge">
                {downloaded.downloadedVersion} → {downloaded.latestVersion}
              </span>
            ) : downloaded ? (
              <span>
                {downloaded.installedInstances.length > 0
                  ? t("installed_in_count", { count: downloaded.installedInstances.length })
                  : t("downloaded_not_installed")}
              </span>
            ) : mod.isInstalled ? (
              <span>{t("installed_status")}</span>
            ) : mod.isDownloaded ? (
              <span>{t("downloaded_status")}</span>
            ) : null}
          </div>
          <div className="row">
            {onDelete && (
              <Button variant="ghost" onClick={(event) => action(event, onDelete)}>
                {t("delete")}
              </Button>
            )}
            <Button onClick={(event) => action(event, onInstall)}>{actionLabel}</Button>
          </div>
        </div>
      </div>
    </article>
  );
}
