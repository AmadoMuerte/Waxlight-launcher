import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { useToastStore } from "../../app/stores/toast";
import { snapshotsApi } from "../../entities/snapshot/api";
import type { InstanceSnapshot } from "../../entities/snapshot/model";
import { useInstanceSnapshotsQuery } from "../../entities/snapshot/queries";
import { errorMessage } from "../../shared/api/bridge";
import { INSTANCES_QUERY_KEY, SNAPSHOTS_QUERY_KEY } from "../../shared/api/keys";
import { formatBytes, formatDate } from "../../shared/lib";
import { Button } from "../../shared/ui/button";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { Empty } from "../../shared/ui/empty";

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
  const [creating, setCreating] = useState(false);
  const [restoringId, setRestoringId] = useState<string>();
  const [deletingId, setDeletingId] = useState<string>();
  const [restoreConfirm, setRestoreConfirm] = useState<InstanceSnapshot>();
  const [deleteConfirm, setDeleteConfirm] = useState<InstanceSnapshot>();

  const busy = creating || restoringId !== undefined || deletingId !== undefined;

  async function refresh() {
    await queryClient.invalidateQueries({ queryKey: SNAPSHOTS_QUERY_KEY(instanceId) });
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
          icon="◈"
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
          {snapshots.map((snapshot) => (
            <article className="installedModRow snapshotRow" key={snapshot.id}>
              <div className="modRowIcon" aria-hidden="true">
                ◈
              </div>
              <div className="modRowCopy">
                <strong>{formatDate(snapshot.createdAt)}</strong>
                <small>{t("manual_snapshot")}</small>
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
                  variant="ghost"
                  className="dangerGhost"
                  disabled={busy}
                  onClick={() => setDeleteConfirm(snapshot)}
                >
                  {t("delete")}
                </Button>
              </div>
            </article>
          ))}
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
