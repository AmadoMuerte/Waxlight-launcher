import { Clock, Download, Rocket } from "lucide-react";
import type { Root } from "mdast";
import type { ReactNode } from "react";
import { useRef } from "react";
import { Trans, useTranslation } from "react-i18next";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { visit } from "unist-util-visit";

import { useAppShellStore } from "../../app/stores/app-shell";
import { useSettingsQuery } from "../../entities/settings/queries";
import type { LauncherUpdate, LauncherUpdateProgress } from "../../shared/api/types";
import { updatesApi } from "../../shared/api/updates";
import { formatBytes } from "../../shared/lib";
import { Button } from "../../shared/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "../../shared/ui/dialog";
import { Progress } from "../../shared/ui/progress";

const mentionPattern = /@[a-zA-Z0-9][a-zA-Z0-9-]*(?![a-zA-Z0-9-])/g;

function remarkMentions() {
  return (tree: Root) => {
    visit(tree, "text", (node, index, parent) => {
      if (index === undefined || !parent) {
        return;
      }
      const matches = node.value.match(mentionPattern);
      if (!matches) {
        return;
      }

      const parts: Array<
        | { type: "text"; value: string }
        | {
            type: "element";
            data: { hName: "span"; hProperties: { className: string } };
            children: [{ type: "text"; value: string }];
          }
      > = [];
      let offset = 0;
      for (const match of matches) {
        const start = node.value.indexOf(match, offset);
        if (start > offset) {
          parts.push({ type: "text", value: node.value.slice(offset, start) });
        }
        parts.push({
          type: "element",
          data: { hName: "span", hProperties: { className: "markdownMention" } },
          children: [{ type: "text", value: match }],
        });
        offset = start + match.length;
      }
      if (offset < node.value.length) {
        parts.push({ type: "text", value: node.value.slice(offset) });
      }

      (parent.children as Array<unknown>).splice(index, 1, ...parts);
    });
  };
}

function MarkdownLink({ href, children }: { href?: string; children?: ReactNode }) {
  return (
    <a
      href={href}
      onClick={(event) => {
        event.preventDefault();
        if (href) {
          void updatesApi.openUrl(href);
        }
      }}
    >
      {children}
    </a>
  );
}

function Changelog({ notes }: { notes: string }) {
  const { t } = useTranslation();

  if (!notes.trim()) {
    return <p className="updateDialogChangelogFallback">{t("release_notes_unavailable")}</p>;
  }

  return (
    <div className="markdownBody">
      <ReactMarkdown remarkPlugins={[remarkGfm, remarkMentions]} components={{ a: MarkdownLink }}>
        {notes}
      </ReactMarkdown>
    </div>
  );
}

function UpdateProgressBody({
  update,
  progress,
}: {
  update: LauncherUpdate;
  progress: LauncherUpdateProgress;
}) {
  const { t } = useTranslation();
  const percent = Math.round(progress.progress * 100);

  return (
    <div className="updateDialogBody updateDialogBodyCentered">
      <DialogTitle className="updateDialogProgressTitle">
        {t(`update_phase_${progress.phase}`)}
      </DialogTitle>
      <p className="updateDialogProgressProduct">
        Waxlight Launcher <span>{update.version}</span>
      </p>
      <div className="updateDialogProgressBar">
        <Progress max={1} value={progress.progress} />
        <p className="updateDialogProgressBytes">
          {formatBytes(progress.downloadedBytes)} / {formatBytes(progress.totalBytes)}
          <span>· {percent}%</span>
        </p>
      </div>
    </div>
  );
}

function UpdateActions({
  update,
  isWindowsPortable,
  installing,
  channel,
}: {
  update: LauncherUpdate;
  isWindowsPortable: boolean;
  installing: boolean;
  channel: "stable" | "prerelease";
}) {
  const { t } = useTranslation();
  const installUpdate = useAppShellStore((state) => state.installUpdate);
  const openRelease = useAppShellStore((state) => state.openRelease);
  const dismissUpdate = useAppShellStore((state) => state.dismissUpdate);

  return (
    <>
      <div className="updateDialogMetadata">
        {update.assetSize > 0 && (
          <p>
            {t("update_version_metadata", {
              version: update.version,
              size: formatBytes(update.assetSize),
            })}
          </p>
        )}
        <button type="button" className="linkButton" onClick={() => void openRelease(channel)}>
          {t("update_release_link")}
        </button>
      </div>

      <div className="updateDialogActions">
        {isWindowsPortable ? (
          <Button type="button" disabled={installing} onClick={() => void openRelease(channel)}>
            <Download className="size-4" />
            {t("download_update")}
          </Button>
        ) : (
          <Button type="button" busy={installing} onClick={() => void installUpdate(channel)}>
            <Download className="size-4" />
            {t("download_and_install_update")}
          </Button>
        )}

        <Button type="button" variant="secondary" disabled={installing} onClick={dismissUpdate}>
          <Clock className="size-4" />
          {t("update_remind_later")}
        </Button>
      </div>
    </>
  );
}

export function UpdateDialog() {
  const { t } = useTranslation();
  const update = useAppShellStore((state) => state.launcherUpdate);
  const platform = useAppShellStore((state) => state.platform);
  const installingUpdate = useAppShellStore((state) => state.installingUpdate);
  const updateProgress = useAppShellStore((state) => state.updateProgress);
  const updateDialogOpen = useAppShellStore((state) => state.updateDialogOpen);
  const dismissUpdate = useAppShellStore((state) => state.dismissUpdate);

  const lastUpdateRef = useRef<LauncherUpdate | undefined>(undefined);
  if (update) {
    lastUpdateRef.current = update;
  }
  const currentUpdate = update ?? lastUpdateRef.current;

  const settingsQuery = useSettingsQuery();
  const channel = settingsQuery.data?.updateChannel ?? "stable";
  const isWindowsPortable =
    currentUpdate?.installationMode === "portable" && platform === "windows";

  return (
    <Dialog
      open={updateDialogOpen && Boolean(update)}
      onOpenChange={(open) => {
        if (!open && !installingUpdate) {
          dismissUpdate();
        }
      }}
    >
      <DialogContent className="updateDialog" closable={!installingUpdate}>
        {currentUpdate && !installingUpdate && (
          <div className="updateDialogBody">
            <DialogTitle className="updateDialogTitle">
              <Rocket className="size-5 shrink-0 text-accent" />
              <span>
                {currentUpdate.prerelease
                  ? t("prerelease_available")
                  : currentUpdate.downgrade
                    ? t("downgrade_available")
                    : t("update_available")}
              </span>
            </DialogTitle>

            <p className="updateDialogVersions">
              <Trans
                i18nKey="update_version_transition"
                values={{ current: currentUpdate.installedVersion, new: currentUpdate.version }}
                components={{ new: <span className="text-accent font-semibold" /> }}
              />
            </p>

            <DialogDescription className="updateDialogDescription">
              {t("update_available_description")}
              {isWindowsPortable && (
                <span className="updateDialogPortableHint">{t("portable_update_hint")}</span>
              )}
            </DialogDescription>

            <div className="updateDialogChangelog">
              <Changelog notes={currentUpdate.releaseNotes} />
            </div>
          </div>
        )}

        {currentUpdate && installingUpdate && updateProgress && (
          <UpdateProgressBody update={currentUpdate} progress={updateProgress} />
        )}

        {currentUpdate && !installingUpdate && (
          <footer className="updateDialogFooter">
            <UpdateActions
              update={currentUpdate}
              isWindowsPortable={isWindowsPortable}
              installing={installingUpdate}
              channel={channel}
            />
          </footer>
        )}
      </DialogContent>
    </Dialog>
  );
}
