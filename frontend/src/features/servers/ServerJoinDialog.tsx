import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { FavoriteServer, GameVersion, Instance, PublicServer } from "../../shared/api/types";
import { Button } from "../../shared/ui/button";
import { DialogFooter } from "../../shared/ui/dialog";
import { Empty } from "../../shared/ui/empty";
import { Field } from "../../shared/ui/field";
import { Modal } from "../../shared/ui/modal";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../shared/ui/select";

export interface ServerJoinDialogProps {
  server: PublicServer;
  favorite?: FavoriteServer;
  instances: Instance[];
  versions: GameVersion[];
  busy: boolean;
  onClose: () => void;
  onConfirm: (instanceId: string) => void;
}

export function ServerJoinDialog({
  server,
  favorite,
  instances,
  versions,
  busy,
  onClose,
  onConfirm,
}: ServerJoinDialogProps) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState(() =>
    favorite?.instanceId && instances.some((instance) => instance.id === favorite.instanceId)
      ? favorite.instanceId
      : "",
  );
  const versionNames = new Map(versions.map((version) => [version.id, version.name]));

  return (
    <Modal title={server.name} onClose={onClose} closable={!busy}>
      <div className="flex min-h-0 flex-col gap-4 p-6">
        {server.address && (
          <p className="truncate font-mono text-xs text-text-muted" title={server.address}>
            {server.address}
          </p>
        )}
        {server.accessRestricted && (
          <p className="rounded-lg border border-warning/40 bg-warning/10 px-3 py-2 text-[length:var(--fs-body)] leading-5 text-warning">
            {t("server_password_enter_in_game")}
          </p>
        )}
        {instances.length === 0 ? (
          <Empty
            title={t("no_instances_available")}
            description={t("create_instance_before_joining")}
          />
        ) : (
          <Field label={t("linked_instance")}>
            <Select value={selected} onValueChange={setSelected}>
              <SelectTrigger>
                <SelectValue placeholder={t("select_instance_to_join")} />
              </SelectTrigger>
              <SelectContent>
                {instances.map((instance) => (
                  <SelectItem key={instance.id} value={instance.id}>
                    <span className="block">
                      <span className="block text-sm">{instance.name}</span>
                      <span className="block text-xs text-text-muted">
                        {t("vintage_story")}{" "}
                        {versionNames.get(instance.gameVersionId) ?? instance.gameVersionId}
                      </span>
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
        )}
      </div>
      <DialogFooter>
        <Button variant="ghost" disabled={busy} onClick={onClose}>
          {t("cancel")}
        </Button>
        <Button busy={busy} disabled={!selected} onClick={() => selected && onConfirm(selected)}>
          {busy ? t("connecting") : t("play")}
        </Button>
      </DialogFooter>
    </Modal>
  );
}
