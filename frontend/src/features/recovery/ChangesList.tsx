import { Gamepad2, Package } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { ConfigurationChanges } from "../../entities/last-known-good/model";

// ChangesList renders the factual differences between the Last Known Good
// state and the current configuration. It never claims a cause: it only lists
// what changed since the last successful launch.
export function ChangesList({ changes }: { changes: ConfigurationChanges }) {
  const { t } = useTranslation();

  // The backend always serializes the three lists as arrays; the guards keep
  // stale event payloads from older launcher versions from crashing the UI.
  const updated = changes.updated ?? [];
  const added = changes.added ?? [];
  const removed = changes.removed ?? [];

  const versionChanged =
    changes.gameVersionFrom !== undefined && changes.gameVersionTo !== undefined;

  return (
    <div className="changesList">
      {versionChanged && (
        <div className="changeGroup">
          <span className="changeGroupLabel">{t("changes_game_version")}</span>
          <div className="changeRow">
            <span className="changeRowIcon" aria-hidden="true">
              <Gamepad2 />
            </span>
            <span className="changeRowName">{t("vintage_story")}</span>
            <span className="changeRowVersions">
              {changes.gameVersionFrom} → {changes.gameVersionTo}
            </span>
          </div>
        </div>
      )}

      {updated.length > 0 && (
        <div className="changeGroup">
          <span className="changeGroupLabel">{t("changes_updated")}</span>
          {updated.map((change) => (
            <div className="changeRow" key={`${change.name}-${change.from}-${change.to}`}>
              <span className="changeRowIcon" aria-hidden="true">
                <Package />
              </span>
              <span className="changeRowName">{change.name}</span>
              <span className="changeRowVersions">
                {change.from && change.to
                  ? `${change.from} → ${change.to}`
                  : (change.from ?? change.to)}
              </span>
            </div>
          ))}
        </div>
      )}

      {added.length > 0 && (
        <div className="changeGroup">
          <span className="changeGroupLabel">{t("changes_added")}</span>
          {added.map((change) => (
            <div className="changeRow" key={`${change.name}-${change.to}`}>
              <span className="changeRowIcon" aria-hidden="true">
                <Package />
              </span>
              <span className="changeRowName">{change.name}</span>
              {change.to && <span className="changeRowVersions">{change.to}</span>}
            </div>
          ))}
        </div>
      )}

      {removed.length > 0 && (
        <div className="changeGroup">
          <span className="changeGroupLabel">{t("changes_removed")}</span>
          {removed.map((change) => (
            <div className="changeRow" key={`${change.name}-${change.from}`}>
              <span className="changeRowIcon removed" aria-hidden="true">
                <Package />
              </span>
              <span className="changeRowName">{change.name}</span>
              {change.from && <span className="changeRowVersions">{change.from}</span>}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
