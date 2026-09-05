import {
  Boxes,
  Copy,
  Globe2,
  Heart,
  Link,
  LockKeyhole,
  ShieldCheck,
  UserRound,
  Users,
  Wrench,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { ModDescription } from "../../features/mods/ModDescription";
import type { FavoriteServer, Instance, PublicServer } from "../../shared/api/types";
import { Button } from "../../shared/ui/button";
import { CoverArt } from "../../shared/ui/cover-art";
import { DialogFooter } from "../../shared/ui/dialog";
import { IconButton } from "../../shared/ui/icon-button";
import { Modal } from "../../shared/ui/modal";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { canRequestServerLaunch } from "./lib";
import { ServerPlayButton } from "./ServerPlayButton";

function openExternal(url: string) {
  try {
    if (new URL(url).protocol === "https:") BrowserOpenURL(url);
  } catch {
    // Ignore malformed URLs from a remote catalog entry.
  }
}

export interface ServerDetailsContentProps {
  server: PublicServer;
  favorite?: FavoriteServer;
  preferredInstance?: Instance;
  favoriteBusy?: boolean;
  detailsLoading?: boolean;
  detailsError?: string;
  onRetry?: () => void;
  onToggleFavorite: () => void;
}

export function ServerDetailsContent({
  server,
  favorite,
  preferredInstance,
  favoriteBusy = false,
  detailsLoading = false,
  detailsError,
  onRetry,
  onToggleFavorite,
}: ServerDetailsContentProps) {
  const { t } = useTranslation();
  return (
    <div className="flex min-h-0 flex-col gap-5 p-6">
      {(server.bannerUrl || server.imageUrl) && (
        <CoverArt
          className="aspect-[3/1] min-h-28 rounded-md border border-border-subtle"
          seed={server.name}
          src={server.bannerUrl ?? server.imageUrl}
        />
      )}
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
            {server.maxPlayers
              ? `${server.players} / ${server.maxPlayers}`
              : t("server_players", { count: server.players })}
          </span>
        )}
        {server.gameVersion && <span>{server.gameVersion}</span>}
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
        {server.modified && (
          <span className="inline-flex items-center gap-1">
            <Wrench size={12} aria-hidden="true" />
            {t("mods")}
          </span>
        )}
      </div>

      {(server.location || server.languages?.length || server.operator) && (
        <div className="grid gap-3 border-y border-border-subtle py-4 text-sm sm:grid-cols-2">
          {server.location && (
            <div className="flex items-center gap-2 text-text-secondary">
              <Globe2 size={15} aria-hidden="true" />
              {server.location}
            </div>
          )}
          {server.languages?.length ? (
            <div className="flex flex-wrap items-center gap-1.5">
              {server.languages.map((language) => (
                <span
                  key={language}
                  className="rounded-full border border-border-subtle px-2 py-0.5 text-xs text-text-secondary"
                >
                  {language}
                </span>
              ))}
            </div>
          ) : null}
          {server.operator && (
            <button
              type="button"
              className="inline-flex w-fit items-center gap-2 text-left text-text-secondary hover:text-accent"
              onClick={() => server.operatorUrl && openExternal(server.operatorUrl)}
              disabled={!server.operatorUrl}
            >
              <UserRound size={15} aria-hidden="true" />
              {t("by_author", { name: server.operator })}
            </button>
          )}
        </div>
      )}

      <div>
        <h3 className="mb-2 font-display text-lg font-semibold">{t("full_description")}</h3>
        {detailsLoading ? (
          <p className="text-sm text-text-muted" aria-live="polite">
            {t("loading_servers")}
          </p>
        ) : (
          <ModDescription
            description={server.descriptionHtml || server.fullDescription || server.description}
            fallback={t("no_description_provided")}
            onOpenExternal={openExternal}
          />
        )}
        {detailsError && (
          <output className="mt-3 flex items-center gap-3 text-sm text-warning">
            <span>{detailsError}</span>
            {onRetry && (
              <Button variant="ghost" onClick={onRetry}>
                {t("retry")}
              </Button>
            )}
          </output>
        )}
      </div>

      {server.mods?.length ? (
        <div>
          <h3 className="mb-2 font-display text-lg font-semibold">{t("mods")}</h3>
          <ul className="grid gap-1 text-sm text-text-secondary">
            {server.mods.map((mod) => (
              <li key={`${mod.name}:${mod.version}`}>
                <button
                  type="button"
                  className="text-left hover:text-accent"
                  onClick={() => openExternal(mod.url)}
                >
                  {mod.name}
                  {mod.version && <span className="text-text-muted"> @{mod.version}</span>}
                </button>
              </li>
            ))}
          </ul>
        </div>
      ) : null}

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
        {server.url && (
          <Button variant="ghost" onClick={() => openExternal(server.url!)}>
            <Link size={15} aria-hidden="true" />
            {t("view_details")}
          </Button>
        )}
        <ServerPlayButton
          blockedByWhitelist={server.requiresWhitelist}
          disabled={!canRequestServerLaunch(server)}
          onClick={onJoin}
        />
      </DialogFooter>
    </Modal>
  );
}
