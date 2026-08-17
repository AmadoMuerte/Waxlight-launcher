import { useQueryClient } from "@tanstack/react-query";
import { Archive } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { useToastStore } from "../../app/stores/toast";
import { useLastKnownGoodQuery } from "../../entities/last-known-good/queries";
import { snapshotsApi } from "../../entities/snapshot/api";
import type { InstanceSnapshot } from "../../entities/snapshot/model";
import { useInstanceSnapshotsQuery } from "../../entities/snapshot/queries";
import { errorMessage } from "../../shared/api/bridge";
import {
  INSTANCES_QUERY_KEY,
  LAST_KNOWN_GOOD_QUERY_KEY,
  SNAPSHOTS_QUERY_KEY,
} from "../../shared/api/keys";
import { formatBytes, formatDate } from "../../shared/lib";
import { Button } from "../../shared/ui/button";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { Empty } from "../../shared/ui/empty";

// SNAPSHOT_REASON_KEYS maps the backend snapshot reasons to i18n keys so the
// Backups UI can explain why an automatic snapshot was created.
const SNAPSHOT_REASON_KEYS: Record<string, string> = {
  before_mod_update: "snapshot_reason_before_mod_update",
  before_mod_removal: "snapshot_reason_before_mod_removal",
  before_game_version_change: "snapshot_reason_before_game_version_change",
};

function snapshotReasonTitle(snapshot: InstanceSnapshot, t: (key: string) => string): string {
  const key = SNAPSHOT_REASON_KEYS[snapshot.reason ?? ""];
  return key ? t(key) : t("snapshot_type_automatic");
}

interface BackupsTabProps {
  instanceId: string;
  onCreated: () => void;
  onRestored: () => void;
}

export function BackupsTab({ instanceId, onCreated, onRestored }: BackupsTabProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  const { data: snapshots = [], isPending } = useInstanceSnapshotsQuery(instanceId);
  const { data: lastKnownGood } = useLastKnownGoodQuery(instanceId);
  const [creating, setCreating] = useState(false);
  const [restoringId, setRestoringId] = useState<string>();
  const [deletingId, setDeletingId] = useState<string>();
  const [restoreConfirm, setRestoreConfirm] = useState<InstanceSnapshot>();
  const [deleteConfirm, setDeleteConfirm] = useState<InstanceSnapshot>();

  const busy = creating || restoringId !== undefined || deletingId !== undefined;

  async function refresh() {
    await queryClient.invalidateQueries({ queryKey: SNAPSHOTS_QUERY_KEY(instanceId) });
    await queryClient.invalidateQueries({ queryKey: LAST_KNOWN_GOOD_QUERY_KEY(instanceId) });
    await queryClient.invalidateQueries({ queryKey: INSTANCES_QUERY_KEY });
  }

  async function createSnapshot() {
    setCreating(true);
    // Leave the instance details so the user can watch the snapshot operation
    // on the Operations page; the operation is already running on the backend.
    onCreated();
    try {
      await snapshotsApi.create(instanceId);
      notify(t("snapshot_created"));
      await refresh();
    } catch (error) {
      notify(errorMessage(error), "error");
    } finally {
      setCreating(false);
    }
  }

  async function restoreSnapshot(snapshot: InstanceSnapshot) {
    setRestoreConfirm(undefined);
    setRestoringId(snapshot.id);
    try {
      await snapshotsApi.restore(instanceId, snapshot.id);
      notify(t("snapshot_restored"));
      onRestored();
      await refresh();
    } catch (error) {
      notify(errorMessage(error), "error");
    } finally {
      setRestoringId(undefined);
    }
  }

  async function deleteSnapshot(snapshot: InstanceSnapshot) {
    setDeleteConfirm(undefined);
    setDeletingId(snapshot.id);
    try {
      await snapshotsApi.remove(instanceId, snapshot.id);
      notify(t("snapshot_deleted"));
      await refresh();
    } catch (error) {
      notify(errorMessage(error), "error");
    } finally {
      setDeletingId(undefined);
    }
  }

  return (
    <div className="instanceTabBody backupsTab" role="tabpanel">
      <header className="instanceToolbar">
        <div>
          <h3>{t("backups")}</h3>
          <p>{t("backups_description")}</p>
        </div>
        <div className="row">
          <Button
            busy={creating}
            disabled={busy && !creating}
            onClick={() => void createSnapshot()}
          >
            {creating ? t("creating_snapshot") : `＋ ${t("create_snapshot")}`}
          </Button>
        </div>
      </header>

      {isPending ? (
        <p className="muted">{t("loading_snapshots")}</p>
      ) : snapshots.length === 0 ? (
        <Empty
          icon={<Archive size={24} aria-hidden="true" />}
          title={t("no_snapshots")}
          description={t("no_snapshots_description")}
          action={
            <Button disabled={busy} onClick={() => void createSnapshot()}>
              {t("create_snapshot")}
            </Button>
          }
        />
      ) : (
        <div className="installedModList">
          {snapshots.map((snapshot) => {
            const automatic = snapshot.type === "automatic";
            const recoveryPoint =
              lastKnownGood?.snapshotExists && lastKnownGood.snapshotId === snapshot.id;
            return (
              <article className="installedModRow snapshotRow" key={snapshot.id}>
                <div className="modRowIcon" aria-hidden="true">
                  ◈
                </div>
                <div className="modRowCopy">
                  <strong>
                    {automatic ? snapshotReasonTitle(snapshot, t) : formatDate(snapshot.createdAt)}
                  </strong>
                  <small>
                    {automatic ? (
                      <>
                        <span className={`snapshotTypeBadge ${snapshot.type}`}>
                          {t("snapshot_type_automatic")}
                        </span>{" "}
                        {formatDate(snapshot.createdAt)}
                      </>
                    ) : (
                      t("manual_snapshot")
                    )}
                    {recoveryPoint && (
                      <>
                        {" "}
                        <span className="snapshotTypeBadge lastKnownGood">
                          {t("last_known_good")}
                        </span>
                      </>
                    )}
                  </small>
                  {automatic &&
                    snapshot.context?.fromGameVersion &&
                    snapshot.context?.toGameVersion && (
                      <small>
                        {snapshot.context.fromGameVersion} → {snapshot.context.toGameVersion}
                      </small>
                    )}
                  <small>
                    {t("snapshot_game_version", { version: snapshot.gameVersion })}
                    {snapshot.modCount > 0
                      ? ` · ${t("mods_count", { count: snapshot.modCount })}`
                      : ""}
                    {` · ${formatBytes(snapshot.sizeBytes)}`}
                  </small>
                </div>
                <div className="modRowActions">
                  <Button
                    variant="secondary"
                    disabled={busy}
                    onClick={() => setRestoreConfirm(snapshot)}
                  >
                    {t("restore")}
                  </Button>
                  <Button
                    variant="danger"
                    disabled={busy}
                    onClick={() => setDeleteConfirm(snapshot)}
                  >
                    {t("delete")}
                  </Button>
                </div>
              </article>
            );
          })}
        </div>
      )}

      <ConfirmDialog
        open={Boolean(restoreConfirm)}
        title={t("restore_snapshot_confirmation")}
        message={t("restore_snapshot_confirmation_description", {
          date: restoreConfirm ? formatDate(restoreConfirm.createdAt) : "",
        })}
        destructive
        confirmLabel={t("restore")}
        loading={restoringId !== undefined}
        onConfirm={() => {
          if (restoreConfirm) {
            void restoreSnapshot(restoreConfirm);
          }
        }}
        onCancel={() => setRestoreConfirm(undefined)}
      />

      <ConfirmDialog
        open={Boolean(deleteConfirm)}
        title={t("delete_snapshot_confirmation")}
        message={t("delete_snapshot_confirmation_description")}
        destructive
        loading={deletingId !== undefined}
        onConfirm={() => {
          if (deleteConfirm) {
            void deleteSnapshot(deleteConfirm);
          }
        }}
        onCancel={() => setDeleteConfirm(undefined)}
      />
    </div>
  );
}
