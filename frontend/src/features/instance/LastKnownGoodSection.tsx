import { History } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { useLastKnownGoodQuery } from "../../entities/last-known-good/queries";
import { formatDate } from "../../shared/lib";
import { Button } from "../../shared/ui/button";
import { ChangesDialog } from "../recovery/ChangesDialog";
import { ChangesList } from "../recovery/ChangesList";

// LastKnownGoodSection shows the recorded Last Known Good marker of an
// instance and how the current configuration compares to it. It is purely
// informational; recovery itself is offered after a failed launch.
export function LastKnownGoodSection({ instanceId }: { instanceId: string }) {
  const { t } = useTranslation();
  const { data: status, isPending } = useLastKnownGoodQuery(instanceId);
  const [changesOpen, setChangesOpen] = useState(false);

  if (isPending || !status?.recordedAt) {
    return null;
  }

  const matches = status.matchesCurrent;
  const details = [
    t("snapshot_game_version", { version: status.gameVersion }),
    t("mods_count", { count: status.modCount }),
  ].join(" · ");

  return (
    <section className="lastKnownGoodSection">
      <header className="lastKnownGoodHeader">
        <h3>{t("last_known_good")}</h3>
        <small>{formatDate(status.recordedAt)}</small>
      </header>
      <p className={matches ? "lastKnownGoodMatch" : "lastKnownGoodDiff"}>
        {matches
          ? t("last_known_good_matches")
          : t("last_known_good_changes_count", { count: status.changeCount })}
      </p>
      <p className="lastKnownGoodDetails">{details}</p>
      {!matches && (
        <Button variant="secondary" onClick={() => setChangesOpen(true)}>
          {t("view_changes")}
        </Button>
      )}

      {changesOpen && (
        <ChangesDialog
          open={changesOpen}
          onClose={() => setChangesOpen(false)}
          icon={<History />}
          iconVariant="muted"
          title={t("changes_since_title")}
          description={t("last_successful_launch", { date: formatDate(status.recordedAt) })}
          footer={
            <Button type="button" variant="ghost" onClick={() => setChangesOpen(false)}>
              {t("close")}
            </Button>
          }
        >
          <div className="recoveryChanges">
            <ChangesList changes={status.changes} />
          </div>
        </ChangesDialog>
      )}
    </section>
  );
}
