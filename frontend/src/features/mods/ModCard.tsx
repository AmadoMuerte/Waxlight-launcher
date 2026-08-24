import { Download, MoreHorizontal, Trash2 } from "lucide-react";
import { memo, useId } from "react";
import type { KeyboardEvent, MouseEvent } from "react";
import { useTranslation } from "react-i18next";

import { cn } from "@/shared/lib/utils";

import type { DownloadedMod, ModSummary } from "../../shared/api";
import { Button } from "../../shared/ui/button";
import { Card, CardContent, CardFooter, CardHeader } from "../../shared/ui/card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "../../shared/ui/dropdown-menu";
import { IconButton } from "../../shared/ui/icon-button";
import { SelectionCheckbox } from "../../shared/ui/selection-checkbox";
import { formatBytes, formatDownloads, relativeDate, sideLabel } from "./lib";
import { ModArtwork } from "./ModArtwork";

interface ModCardProps {
  mod: ModSummary;
  downloaded?: DownloadedMod;
  layout: "grid" | "list";
  onOpen: (modId: string) => void;
  onInstall: (modId: string, downloaded?: DownloadedMod) => void;
  selected?: boolean;
  onSelectedChange?: (modId: string, selected: boolean) => void;
  onDelete?: (downloaded: DownloadedMod) => void;
  installBusy?: boolean;
}

function stopPropagationAndRun(event: MouseEvent, callback: () => void) {
  event.stopPropagation();
  callback();
}

export const ModCard = memo(function ModCard({
  mod,
  downloaded,
  layout,
  onOpen,
  onInstall,
  selected = false,
  onSelectedChange,
  onDelete,
  installBusy = false,
}: ModCardProps) {
  const { t } = useTranslation();
  const titleId = useId();
  function openFromKeyboard(event: KeyboardEvent<HTMLElement>) {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onOpen(mod.id);
    }
  }

  const actionLabel = downloaded
    ? downloaded.updateAvailable
      ? t("update")
      : downloaded.installedInstances.length > 0
        ? t("install_to_another")
        : t("install_to_instance")
    : mod.updateAvailable
      ? t("update")
      : mod.isDownloaded
        ? t("install_to_instance")
        : t("download");

  let statusText: string | null = null;
  let statusClassName = "text-text-muted";
  if (downloaded?.updateAvailable) {
    statusText = `${downloaded.downloadedVersion} → ${downloaded.latestVersion}`;
    statusClassName = "text-warning";
  } else if (downloaded) {
    statusText =
      downloaded.installedInstances.length > 0
        ? t("installed_in_count", { count: downloaded.installedInstances.length })
        : t("downloaded_not_installed");
    if (downloaded.installedInstances.length > 0) statusClassName = "text-success";
  } else if (mod.isInstalled) {
    statusText = t("installed_status");
    statusClassName = "text-success";
  } else if (mod.isDownloaded) {
    statusText = t("downloaded_status");
  }

  const imageUrl = downloaded?.imageUrl ?? mod.imageUrl;
  const staticImageUrl = imageUrl && !/\.gif(?:$|[?#])/i.test(imageUrl) ? imageUrl : undefined;

  return (
    <article className="modCard min-w-0" aria-labelledby={titleId}>
      <Card
        variant="subtle"
        className="group relative flex min-w-0 flex-col overflow-hidden transition-colors hover:border-accent/60 focus-within:border-accent/60"
      >
        <button
          type="button"
          aria-label={t("open_mod", { name: mod.name })}
          onClick={() => onOpen(mod.id)}
          onKeyDown={openFromKeyboard}
          className="absolute inset-0 z-[1] rounded-lg border-0 bg-transparent outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-inset"
        />

        <div
          className={cn(
            "min-w-0",
            layout === "list" && "sm:grid sm:grid-cols-[minmax(180px,26%)_minmax(0,1fr)]",
          )}
        >
          <div className="relative">
            <ModArtwork
              src={staticImageUrl}
              alt={t("cover_alt", { name: mod.name })}
              seed={mod.name}
              className={layout === "list" ? "aspect-auto h-full min-h-60" : ""}
            />
            {onSelectedChange && (
              <SelectionCheckbox
                className="absolute top-3 right-3 z-[2]"
                label={t("select_mod", { name: mod.name })}
                checked={selected}
                onCheckedChange={(next) => onSelectedChange(mod.id, next)}
              />
            )}
          </div>

          <div className="flex min-w-0 flex-col">
            <CardHeader className="relative pb-0">
              <div className="flex min-w-0 items-start justify-between gap-3">
                <h3
                  id={titleId}
                  className="min-w-0 truncate font-display text-xl font-semibold"
                  title={mod.name}
                >
                  {mod.name}
                </h3>
                {downloaded && onDelete && (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <IconButton
                        className="relative z-[2] shrink-0"
                        variant="ghost"
                        size="sm"
                        aria-label={t("mod_actions", { name: mod.name })}
                      >
                        <MoreHorizontal aria-hidden="true" />
                      </IconButton>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem variant="destructive" onSelect={() => onDelete(downloaded)}>
                        <Trash2 aria-hidden="true" />
                        {t("delete")}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
              </div>
              <p className="text-xs text-text-muted">{t("by_author", { name: mod.authorName })}</p>
            </CardHeader>

            <CardContent className="mt-auto space-y-2 pb-4">
              <p className="line-clamp-2 min-h-10 text-sm leading-5 text-text-muted">
                {downloaded
                  ? t("version_with_size", {
                      version: downloaded.downloadedVersion,
                      size: formatBytes(downloaded.fileSize),
                    })
                  : mod.summary || t("no_description_provided")}
              </p>
              <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-text-muted">
                <span>{sideLabel(mod.side)}</span>
                <span>{mod.gameVersions.slice(-2).join(", ") || t("versions_in_details")}</span>
                <span>{t("updated_relative", { date: relativeDate(mod.updatedAt) })}</span>
                <span className="inline-flex items-center gap-1">
                  <Download size={12} aria-hidden="true" />
                  {formatDownloads(mod.downloads)}
                </span>
              </div>
            </CardContent>

            <CardFooter className="justify-between py-3">
              {statusText && (
                <span
                  className={cn("min-w-0 truncate text-xs font-medium", statusClassName)}
                  title={statusText}
                >
                  {statusText}
                </span>
              )}
              <Button
                className="relative z-[2]"
                busy={installBusy}
                onClick={(event) =>
                  stopPropagationAndRun(event, () => onInstall(mod.id, downloaded))
                }
              >
                {actionLabel}
              </Button>
            </CardFooter>
          </div>
        </div>
      </Card>
    </article>
  );
});
