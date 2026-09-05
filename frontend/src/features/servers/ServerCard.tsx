import {
  Boxes,
  Copy,
  Eye,
  Globe2,
  Heart,
  Link,
  LockKeyhole,
  MoreHorizontal,
  ShieldCheck,
  Users,
  Wrench,
} from "lucide-react";
import { memo, useId } from "react";
import type { MouseEvent } from "react";
import { useTranslation } from "react-i18next";

import type { FavoriteServer, Instance, PublicServer } from "../../shared/api/types";
import { Card, CardContent, CardFooter, CardHeader } from "../../shared/ui/card";
import { CoverArt } from "../../shared/ui/cover-art";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../../shared/ui/dropdown-menu";
import { IconButton } from "../../shared/ui/icon-button";
import { canRequestServerLaunch } from "./lib";
import { ServerPlayButton } from "./ServerPlayButton";

export interface ServerCardProps {
  server: PublicServer;
  favorite?: FavoriteServer;
  preferredInstance?: Instance;
  busy?: boolean;
  favoriteBusy?: boolean;
  onJoin: (server: PublicServer, favorite?: FavoriteServer) => void;
  onToggleFavorite: (server: PublicServer, favorite?: FavoriteServer) => void;
  onDetails: (server: PublicServer) => void;
  onCopyAddress: (address: string) => void;
  onCopyLink: (address: string) => void;
}

function stopPropagationAndRun(event: MouseEvent, callback: () => void) {
  event.stopPropagation();
  callback();
}

export const ServerCard = memo(function ServerCard({
  server,
  favorite,
  preferredInstance,
  busy = false,
  favoriteBusy = false,
  onJoin,
  onToggleFavorite,
  onDetails,
  onCopyAddress,
  onCopyLink,
}: ServerCardProps) {
  const { t } = useTranslation();
  const titleId = useId();

  return (
    <article className="min-w-0" aria-labelledby={titleId}>
      <Card className="group relative flex min-w-0 flex-col overflow-hidden transition-colors hover:border-accent/60 focus-within:border-accent/60">
        <button
          type="button"
          aria-label={t("open_server_details")}
          disabled={busy}
          onClick={() => onDetails(server)}
          className="absolute inset-0 z-[1] rounded-lg border-0 bg-transparent outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-inset"
        />

        <div className="relative">
          <CoverArt className="aspect-[16/5] min-h-20" seed={server.name} src={server.imageUrl} />
          <div className="absolute top-3 right-3 z-[2] flex items-center gap-1.5">
            <IconButton
              size="sm"
              aria-label={favorite ? t("remove_from_favorites") : t("add_favorite")}
              aria-pressed={Boolean(favorite)}
              disabled={busy || favoriteBusy}
              onClick={(event) =>
                stopPropagationAndRun(event, () => onToggleFavorite(server, favorite))
              }
            >
              <Heart
                className={favorite ? "fill-current text-accent" : ""}
                size={15}
                aria-hidden="true"
              />
            </IconButton>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <IconButton
                  size="sm"
                  disabled={busy}
                  aria-label={t("server_actions", { name: server.name })}
                >
                  <MoreHorizontal aria-hidden="true" />
                </IconButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onSelect={() => onDetails(server)}>
                  <Eye aria-hidden="true" />
                  {t("view_details")}
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={() => onCopyAddress(server.address)}>
                  <Copy aria-hidden="true" />
                  {t("copy_server_address")}
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={() => onCopyLink(server.address)}>
                  <Link aria-hidden="true" />
                  {t("copy_waxlight_link")}
                </DropdownMenuItem>
                {favorite && (
                  <>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      variant="destructive"
                      onSelect={() => onToggleFavorite(server, favorite)}
                    >
                      <Heart aria-hidden="true" />
                      {t("remove_from_favorites")}
                    </DropdownMenuItem>
                  </>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>

        <CardHeader className="relative pb-0">
          <div className="min-w-0">
            <h3
              id={titleId}
              className="line-clamp-2 min-h-[3.2rem] font-display text-xl leading-snug font-semibold"
              title={server.name}
            >
              {server.name}
            </h3>
            <p
              className="mt-1 truncate font-mono text-[11px] text-text-muted"
              title={server.address}
            >
              {server.address || t("server_address")}
            </p>
          </div>
        </CardHeader>

        <CardContent className="mt-auto space-y-2 pb-4">
          <p className="line-clamp-2 min-h-10 text-sm leading-5 text-text-muted">
            {server.description || t("no_description_provided")}
          </p>
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
            {server.location && (
              <span className="inline-flex items-center gap-1" title={server.location}>
                <Globe2 size={12} aria-hidden="true" />
                {server.location}
              </span>
            )}
          </div>
        </CardContent>

        <CardFooter className="justify-between py-3">
          {favorite ? (
            preferredInstance ? (
              <span
                className="min-w-0 truncate text-xs text-text-muted"
                title={preferredInstance.name}
              >
                {t("using_preferred_instance", { name: preferredInstance.name })}
              </span>
            ) : (
              <span className="min-w-0 truncate text-xs text-warning">
                {t("linked_instance_missing")}
              </span>
            )
          ) : (
            <span />
          )}
          <div className="relative z-[2] shrink-0">
            <ServerPlayButton
              blockedByWhitelist={server.requiresWhitelist}
              disabled={!canRequestServerLaunch(server)}
              busy={busy}
              onClick={(event) => stopPropagationAndRun(event, () => onJoin(server, favorite))}
            />
          </div>
        </CardFooter>
      </Card>
    </article>
  );
});
