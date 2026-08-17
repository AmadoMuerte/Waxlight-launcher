import { Boxes, Copy, Heart, Link, LockKeyhole, ShieldCheck, Users } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { FavoriteServer, Instance, PublicServer } from "../../shared/api/types";
import { Button } from "../../shared/ui/button";
import { DialogFooter } from "../../shared/ui/dialog";
import { IconButton } from "../../shared/ui/icon-button";
import { Modal } from "../../shared/ui/modal";
import { canRequestServerLaunch } from "./lib";
import { ServerPlayButton } from "./ServerPlayButton";

export interface ServerDetailsContentProps {
  server: PublicServer;
  favorite?: FavoriteServer;
  preferredInstance?: Instance;
  favoriteBusy?: boolean;
  onToggleFavorite: () => void;
}

export function ServerDetailsContent({
  server,
  favorite,
  preferredInstance,
  favoriteBusy = false,
  onToggleFavorite,
}: ServerDetailsContentProps) {
  const { t } = useTranslation();
  return (
    <div className="flex min-h-0 flex-col gap-5 p-6">
      <div className="flex items-start justify-between gap-3">
        <p className="min-w-0 truncate font-mono text-xs text-text-muted" title={server.address}>
          {server.address || t("server_address")}
        </p>
        <IconButton
          aria-label={favorite ? t("remove_from_favorites") : t("add_favorite")}
          aria-pressed={Boolean(favorite)}
          disabled={favoriteBusy}
          onClick={onToggleFavorite}
        >
          <Heart
            className={favorite ? "fill-current text-accent" : ""}
            size={16}
            aria-hidden="true"
          />
        </IconButton>
      </div>

      <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-text-muted">
        {server.players > 0 && (
          <span className="inline-flex items-center gap-1">
            <Users size={12} aria-hidden="true" />
            {t("server_players", { count: server.players })}
          </span>
        )}
        {server.modCount > 0 && (
          <span className="inline-flex items-center gap-1">
            <Boxes size={12} aria-hidden="true" />
            {t("server_mods", { count: server.modCount })}
          </span>
        )}
        {server.requiresWhitelist && (
          <span className="inline-flex items-center gap-1">
            <ShieldCheck size={12} aria-hidden="true" />
            {t("server_whitelist")}
          </span>
        )}
        {server.accessRestricted && (
          <span className="inline-flex items-center gap-1">
            <LockKeyhole size={12} aria-hidden="true" />
            {t("server_password")}
          </span>
        )}
      </div>

      <div>
        <h3 className="mb-2 font-display text-lg font-semibold">{t("full_description")}</h3>
        <p className="max-w-prose text-sm leading-6 text-text-secondary">
          {server.description || t("no_description_provided")}
        </p>
      </div>

      <div>
        <h3 className="mb-2 font-display text-lg font-semibold">{t("linked_instance")}</h3>
        {preferredInstance ? (
          <p className="text-sm text-text-secondary">
            {t("using_preferred_instance", { name: preferredInstance.name })}
          </p>
        ) : favorite ? (
          <p className="text-sm text-warning">{t("linked_instance_missing")}</p>
        ) : (
          <p className="text-sm text-text-muted">{t("no_linked_instance")}</p>
        )}
      </div>
    </div>
  );
}

export interface ServerDetailsDialogProps extends ServerDetailsContentProps {
  onClose: () => void;
  onCopyAddress: () => void;
  onCopyLink: () => void;
  onJoin: () => void;
}

export function ServerDetailsDialog({
  server,
  onClose,
  onCopyAddress,
  onCopyLink,
  onJoin,
  ...content
}: ServerDetailsDialogProps) {
  const { t } = useTranslation();
  return (
    <Modal title={server.name} onClose={onClose}>
      <ServerDetailsContent server={server} {...content} />
      <DialogFooter>
        <Button variant="ghost" onClick={onClose}>
          {t("close")}
        </Button>
        <Button variant="ghost" onClick={onCopyAddress}>
          <Copy size={15} aria-hidden="true" />
          {t("copy_server_address")}
        </Button>
        <Button variant="ghost" onClick={onCopyLink}>
          <Link size={15} aria-hidden="true" />
          {t("copy_waxlight_link")}
        </Button>
        <ServerPlayButton
          blockedByWhitelist={server.requiresWhitelist}
          disabled={!canRequestServerLaunch(server)}
          onClick={onJoin}
        />
      </DialogFooter>
    </Modal>
  );
}
