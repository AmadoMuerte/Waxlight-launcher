import { useTranslation } from "react-i18next";

import type { AvailableGameVersion } from "../../entities/game-version/model";
import { formatBytes } from "../../shared/lib";
import { Button } from "../../shared/ui/button";
import { StatusPill } from "../../shared/ui/status-pill";

interface VersionItemProps {
  version: AvailableGameVersion;
  installed: boolean;
  busy?: boolean;
  onInstall: () => void;
}

export function VersionItem({ version, installed, busy = false, onInstall }: VersionItemProps) {
  const { t } = useTranslation();

  return (
    <div className="versionRow">
      <div className="versionRowIdentity">
        <div className="flex min-w-0 items-center gap-2">
          <strong>{version.name}</strong>
          {version.latest && <span className="latestMark">{t("latest")}</span>}
        </div>
        <small>
          {version.platform} · {version.architecture} · {formatBytes(version.downloadSize)}
        </small>
      </div>
      <StatusPill status={version.channel} />
      <Button
        variant={installed ? "ghost" : "primary"}
        busy={busy}
        disabled={installed}
        onClick={onInstall}
      >
        {installed ? t("installed") : t("download")}
      </Button>
    </div>
  );
}
