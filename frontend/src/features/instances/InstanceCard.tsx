import { memo } from "react";
import { useTranslation } from "react-i18next";

import type { GameVersion } from "../../entities/game-version/model";
import type { Instance } from "../../entities/instance/model";
import { formatDate, formatDuration } from "../../shared/lib";
import { Button } from "../../shared/ui/button";
import { StatusPill } from "../../shared/ui/status-pill";

interface InstanceCardProps {
  instance: Instance;
  version?: GameVersion;
  updateCount?: number;
  onOpen: (instance: Instance) => void;
  onLaunch: (instance: Instance) => void;
  onStop: (instance: Instance) => Promise<void>;
}

export const InstanceCard = memo(function InstanceCard({
  instance,
  version,
  updateCount = 0,
  onOpen,
  onLaunch,
  onStop,
}: InstanceCardProps) {
  const { t } = useTranslation();
  return (
    <article className="instanceCard" style={{ position: "relative" }}>
      <button
        type="button"
        aria-label={t("open_instance_details")}
        onClick={() => onOpen(instance)}
        style={{ position: "absolute", zIndex: 1, inset: 0, border: 0, background: "transparent" }}
      />
      <div className="cover">
        <span className="coverLetter">W</span>
        <div className="coverGlow" />
        {instance.coverUrl && <img className="coverImage" src={instance.coverUrl} alt="" />}
        <StatusPill status={instance.status} />
        {updateCount > 0 && (
          <span
            className="modUpdatesBadge"
            title={t("mod_updates_available", { count: updateCount })}
          >
            ▲ {updateCount}
          </span>
        )}
      </div>

      <div className="cardBody">
        <div className="cardTitle">
          <h3>{instance.name}</h3>
          <button
            type="button"
            className="more"
            aria-label={t("open_instance_details")}
            onClick={() => onOpen(instance)}
            style={{ position: "relative", zIndex: 2 }}
          >
            •••
          </button>
        </div>

        <p>{instance.description || t("instance_default_description")}</p>

        <div className="meta">
          <span>◈ {version?.name ?? instance.gameVersionId}</span>
          <span>◇ {t("mods_count", { count: instance.enabledModCount })}</span>
          <span>◷ {formatDuration(instance.playtimeSeconds)}</span>
        </div>

        <div className="cardFooter">
          <span>{formatDate(instance.lastPlayedAt)}</span>
          {instance.status === "running" ? (
            <Button
              variant="danger"
              style={{ position: "relative", zIndex: 2 }}
              onClick={(event) => {
                event.stopPropagation();
                void onStop(instance);
              }}
            >
              {t("stop")}
            </Button>
          ) : (
            <Button
              style={{ position: "relative", zIndex: 2 }}
              onClick={(event) => {
                event.stopPropagation();
                onLaunch(instance);
              }}
            >
              ▶ {t("play")}
            </Button>
          )}
        </div>
      </div>
    </article>
  );
});
