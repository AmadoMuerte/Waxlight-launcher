import type { KeyboardEvent, MouseEvent } from "react";

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
      ? "Update"
      : downloaded.installedInstances.length > 0
        ? "Install to another"
        : "Install to instance"
    : mod.updateAvailable
      ? "Update"
      : mod.isDownloaded
        ? "Install to instance"
        : "Download";

  return (
    <article
      className={`modCard modCard-${layout}`}
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={openFromKeyboard}
      aria-label={`Open ${mod.name}`}
    >
      <ModArtwork src={downloaded?.imageUrl ?? mod.imageUrl} alt={`${mod.name} cover`} />
      <div className="modCardBody">
        <div className="modCardTitle">
          <div>
            <h3>{mod.name}</h3>
            <span>by {mod.authorName}</span>
          </div>
          <span className={`sideBadge side-${mod.side}`}>{sideLabel(mod.side)}</span>
        </div>

        {downloaded ? (
          <p className="modSummary">
            Version {downloaded.downloadedVersion} · {formatBytes(downloaded.fileSize)}
          </p>
        ) : (
          <p className="modSummary">{mod.summary || "No description provided."}</p>
        )}

        <div className="modCardMeta">
          <span>{mod.gameVersions.slice(-2).join(", ") || "Versions in details"}</span>
          <span>Updated {relativeDate(mod.updatedAt)}</span>
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
                  ? `Installed in ${downloaded.installedInstances.length}`
                  : "Downloaded · Not installed"}
              </span>
            ) : mod.isInstalled ? (
              <span>✓ Installed</span>
            ) : mod.isDownloaded ? (
              <span>✓ Downloaded</span>
            ) : null}
          </div>
          <div className="row">
            {onDelete && (
              <Button variant="ghost" onClick={(event) => action(event, onDelete)}>
                Delete
              </Button>
            )}
            <Button onClick={(event) => action(event, onInstall)}>{actionLabel}</Button>
          </div>
        </div>
      </div>
    </article>
  );
}
