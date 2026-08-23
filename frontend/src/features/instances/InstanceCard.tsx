import {
  Copy,
  Eye,
  FolderOpen,
  MoreHorizontal,
  PackageOpen,
  Pencil,
  Play,
  Square,
  Star,
  Trash2,
} from "lucide-react";
import { memo, useId } from "react";
import { useTranslation } from "react-i18next";

import type { GameVersion } from "../../entities/game-version/model";
import type { Instance } from "../../entities/instance/model";
import { formatDate, formatDuration } from "../../shared/lib";
import { Button } from "../../shared/ui/button";
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
import { StatusPill } from "../../shared/ui/status-pill";

interface InstanceCardProps {
  instance: Instance;
  version?: GameVersion;
  updateCount?: number;
  busy?: boolean;
  pinBusy?: boolean;
  onOpen: (instance: Instance) => void;
  onEdit: (instance: Instance) => void;
  onOpenDirectory: (instance: Instance) => void;
  onClone: (instance: Instance) => void;
  onExport: (instance: Instance) => void;
  onDelete: (instance: Instance) => void;
  onLaunch: (instance: Instance) => void;
  onStop: (instance: Instance) => Promise<void>;
  onTogglePin: (instance: Instance) => void;
}

export const InstanceCard = memo(function InstanceCard({
  instance,
  version,
  updateCount = 0,
  busy = false,
  pinBusy = false,
  onOpen,
  onEdit,
  onOpenDirectory,
  onClone,
  onExport,
  onDelete,
  onLaunch,
  onStop,
  onTogglePin,
}: InstanceCardProps) {
  const { t } = useTranslation();
  const titleId = useId();
  const running = instance.status === "running";

  return (
    <article className="min-w-0" aria-labelledby={titleId}>
      <Card className="group relative flex min-w-0 flex-col overflow-hidden transition-colors hover:border-accent/60 focus-within:border-accent/60">
        <button
          type="button"
          className="absolute inset-0 z-[1] rounded-lg border-0 bg-transparent outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-inset"
          aria-label={t("open_instance_details")}
          disabled={busy}
          onClick={() => onOpen(instance)}
        />

        <div className="relative">
          <CoverArt
            className="aspect-[16/7] min-h-32"
            src={instance.coverUrl}
            seed={instance.name}
          />
          <div className="absolute top-3 right-3 max-w-[calc(100%-24px)]">
            <StatusPill status={instance.status} />
          </div>
          {updateCount > 0 && (
            <span
              className="absolute right-3 bottom-3 max-w-[calc(100%-24px)] truncate rounded-full border border-border-default bg-surface-2/95 px-2 py-1 text-[11px] font-bold text-warning"
              title={t("mod_updates_available", { count: updateCount })}
            >
              {t("mod_updates_available", { count: updateCount })}
            </span>
          )}
        </div>

        <CardHeader className="relative pb-0">
          <div className="flex min-w-0 items-start justify-between gap-3">
            <h3
              id={titleId}
              className="min-w-0 truncate font-display text-xl font-semibold"
              title={instance.name}
            >
              {instance.name}
            </h3>
            <div className="relative z-[2] flex shrink-0 items-center gap-1">
              <IconButton
                className={instance.isPinned ? "text-warning" : undefined}
                variant="ghost"
                size="sm"
                aria-label={`${t(instance.isPinned ? "unpin_instance" : "pin_instance")}: ${instance.name}`}
                aria-pressed={instance.isPinned}
                title={t(instance.isPinned ? "unpin_instance" : "pin_instance")}
                disabled={busy || pinBusy}
                onClick={() => onTogglePin(instance)}
              >
                <Star fill={instance.isPinned ? "currentColor" : "none"} aria-hidden="true" />
              </IconButton>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <IconButton
                    className="shrink-0"
                    variant="ghost"
                    size="sm"
                    aria-label={t("instance_actions", { name: instance.name })}
                    disabled={busy}
                  >
                    <MoreHorizontal aria-hidden="true" />
                  </IconButton>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onSelect={() => onOpen(instance)}>
                    <Eye aria-hidden="true" />
                    {t("overview")}
                  </DropdownMenuItem>
                  <DropdownMenuItem onSelect={() => onEdit(instance)}>
                    <Pencil aria-hidden="true" />
                    {t("settings")}
                  </DropdownMenuItem>
                  <DropdownMenuItem onSelect={() => onOpenDirectory(instance)}>
                    <FolderOpen aria-hidden="true" />
                    {t("open_directory")}
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onSelect={() => onClone(instance)}>
                    <Copy aria-hidden="true" />
                    {t("clone_instance")}
                  </DropdownMenuItem>
                  <DropdownMenuItem onSelect={() => onExport(instance)}>
                    <PackageOpen aria-hidden="true" />
                    {t("export_instance")}
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem variant="destructive" onSelect={() => onDelete(instance)}>
                    <Trash2 aria-hidden="true" />
                    {t("delete_instance")}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
          <p className="line-clamp-2 min-h-10 text-sm leading-5 text-text-muted">
            {instance.description || t("instance_default_description")}
          </p>
        </CardHeader>

        <CardContent className="mt-auto space-y-2 pb-4">
          <strong
            className="block truncate text-sm font-semibold text-text-secondary"
            title={version?.name ?? instance.gameVersionId}
          >
            {version?.name ?? instance.gameVersionId}
          </strong>
          <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-text-muted">
            <span>{t("mods_count", { count: instance.enabledModCount })}</span>
            <span>{formatDuration(instance.playtimeSeconds)}</span>
          </div>
        </CardContent>

        <CardFooter className="justify-between py-3">
          <span
            className="min-w-0 truncate text-xs text-text-muted"
            title={formatDate(instance.lastPlayedAt)}
          >
            {formatDate(instance.lastPlayedAt)}
          </span>
          {running ? (
            <Button
              className="relative z-[2]"
              variant="danger"
              busy={busy}
              onClick={() => void onStop(instance)}
            >
              <Square size={14} aria-hidden="true" />
              {t("stop")}
            </Button>
          ) : (
            <Button className="relative z-[2]" busy={busy} onClick={() => onLaunch(instance)}>
              <Play size={15} fill="currentColor" aria-hidden="true" />
              {busy ? t("starting_instance") : t("play")}
            </Button>
          )}
        </CardFooter>
      </Card>
    </article>
  );
});
