import { useQueryClient } from "@tanstack/react-query";
import { Clock, TriangleAlert } from "lucide-react";
import { useTranslation } from "react-i18next";

import { useRecoveryStore } from "../../app/stores/recovery";
import { useToastStore } from "../../app/stores/toast";
import { snapshotsApi } from "../../entities/snapshot/api";
import { errorMessage } from "../../shared/api/bridge";
import {
  INSTANCES_QUERY_KEY,
  LAST_KNOWN_GOOD_QUERY_KEY,
  SNAPSHOTS_QUERY_KEY,
} from "../../shared/api/keys";
import { formatDate } from "../../shared/lib";
import { log } from "../../shared/lib/logger";
import { Button } from "../../shared/ui/button";
import { ChangesDialog } from "./ChangesDialog";
import { ChangesList } from "./ChangesList";

// RecoveryDialog renders the backend's recovery suggestion after a failed
// startup. Waxlight never rolls back automatically: every action here is an
// explicit user choice. Restoring reuses the existing snapshot restore path.
export function RecoveryDialog() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  const suggestion = useRecoveryStore((state) => state.suggestion);
  const restoring = useRecoveryStore((state) => state.restoring);
  const acknowledge = useRecoveryStore((state) => state.acknowledge);
  const hide = useRecoveryStore((state) => state.hide);
  const setRestoring = useRecoveryStore((state) => state.setRestoring);

  async function restoreLastKnownGood() {
    if (!suggestion?.snapshotId) {
      return;
    }
    setRestoring(true);
    log.info("last known good restore requested", {
      instanceId: suggestion.instanceId,
      snapshotId: suggestion.snapshotId,
    });
    try {
      await snapshotsApi.restore(suggestion.instanceId, suggestion.snapshotId);
      notify(t("snapshot_restored"));
      await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
      await queryClient.invalidateQueries({ queryKey: SNAPSHOTS_QUERY_KEY(suggestion.instanceId) });
      await queryClient.invalidateQueries({
        queryKey: LAST_KNOWN_GOOD_QUERY_KEY(suggestion.instanceId),
      });
      hide();
    } catch (error) {
      notify(errorMessage(error), "error");
      setRestoring(false);
    }
  }

  return (
    <ChangesDialog
      open={Boolean(suggestion)}
      onClose={() => {
        if (!restoring) {
          acknowledge();
        }
      }}
      icon={<TriangleAlert />}
      iconVariant="warning"
      title={t("recovery_failed_start_title")}
      description={t("recovery_changes_since_title")}
      footer={
        <>
          <Button type="button" variant="ghost" disabled={restoring} onClick={acknowledge}>
            {t("keep_current_state")}
          </Button>
          {suggestion?.snapshotExists && (
            <Button type="button" busy={restoring} onClick={() => void restoreLastKnownGood()}>
              {t("restore_last_known_good")}
            </Button>
          )}
        </>
      }
    >
      {suggestion && (
        <>
          <div className="recoveryChanges">
            <ChangesList changes={suggestion.changes} />
          </div>
          <p className="recoveryLastLaunch">
            <Clock aria-hidden="true" />
            <span>{t("last_successful_launch", { date: formatDate(suggestion.recordedAt) })}</span>
          </p>
          {!suggestion.snapshotExists && (
            <div className="recoveryNoSnapshot" role="note">
              <TriangleAlert className="recoveryNoSnapshotIcon" aria-hidden="true" />
              <div>
                <strong>{t("no_recovery_snapshot")}</strong>
                <p>{t("no_recovery_snapshot_hint")}</p>
              </div>
            </div>
          )}
        </>
      )}
    </ChangesDialog>
  );
}
