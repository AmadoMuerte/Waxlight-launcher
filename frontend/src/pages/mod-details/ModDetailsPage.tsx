import { useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, ChevronLeft, ChevronRight, Download, ExternalLink, Link } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams, useSearchParams } from "react-router";

import { useToastStore } from "../../app/stores/toast";
import { useGameVersionsQuery } from "../../entities/game-version/queries";
import { useInstancesQuery } from "../../entities/instance/queries";
import { useDownloadedModsQuery, useModDetailsQuery } from "../../entities/mod/queries";
import { InstancePickerDialog } from "../../features/mods/InstancePickerDialog";
import {
  formatBytes,
  formatDownloads,
  formatGameVersions,
  plainText,
  relativeDate,
  releaseTypeLabel,
  sideLabel,
} from "../../features/mods/lib";
import { ModArtwork } from "../../features/mods/ModArtwork";
import { ModDescription } from "../../features/mods/ModDescription";
import { ModScreenshotCarousel } from "../../features/mods/ModScreenshotCarousel";
import { errorMessage } from "../../shared/api/bridge";
import { DOWNLOADED_MODS_QUERY_KEY } from "../../shared/api/keys";
import { modShareURL } from "../../shared/lib/waxlight-links";
import { Button } from "../../shared/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../../shared/ui/card";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { Dialog, DialogContent } from "../../shared/ui/dialog";
import { ErrorState } from "../../shared/ui/error-state";
import { IconButton } from "../../shared/ui/icon-button";
import { LoadingState } from "../../shared/ui/loading-state";
import { Page, PageContent } from "../../shared/ui/page";
import { BrowserOpenURL, ClipboardSetText } from "../../wailsjs/runtime/runtime";

export function ModDetailsPage() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  const { modId = "" } = useParams();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { data: instances = [] } = useInstancesQuery();
  const { data: versions = [] } = useGameVersionsQuery();
  const modQuery = useModDetailsQuery(modId);
  const downloadedQuery = useDownloadedModsQuery();
  const [installVersion, setInstallVersion] = useState<string>();
  const [lightbox, setLightbox] = useState<number>();
  const [pendingExternalUrl, setPendingExternalUrl] = useState<string>();
  const from = searchParams.get("from") ?? "";
  const instanceId = new URLSearchParams(from).get("instanceId") ?? undefined;

  const mod = modQuery.data;
  const loading = modQuery.isPending;
  const error = modQuery.error ? errorMessage(modQuery.error) : "";

  useEffect(() => {
    if (lightbox === undefined) return () => {};
    function keydown(event: KeyboardEvent) {
      if (!mod) return;
      if (event.key === "Escape") setLightbox(undefined);
      if (event.key === "ArrowRight")
        setLightbox((index) => ((index ?? 0) + 1) % mod.screenshots.length);
      if (event.key === "ArrowLeft")
        setLightbox(
          (index) => ((index ?? 0) - 1 + mod.screenshots.length) % mod.screenshots.length,
        );
    }
    window.addEventListener("keydown", keydown);
    return () => window.removeEventListener("keydown", keydown);
  }, [lightbox, mod]);

  const local = useMemo(() => {
    const sorted = [...(downloadedQuery.data ?? [])].filter((item) => item.modId === modId);
    Array.prototype.sort.call(
      sorted,
      (left, right) =>
        new Date(right.downloadedAt).getTime() - new Date(left.downloadedAt).getTime(),
    );
    return sorted[0];
  }, [downloadedQuery.data, modId]);

  if (loading) {
    return (
      <Page>
        <LoadingState>{t("loading_mods")}</LoadingState>
      </Page>
    );
  }
  if (error || !mod) {
    return (
      <Page>
        <ErrorState
          title={t("could_not_load_mod")}
          description={error || t("mod_not_found")}
          action={<Button onClick={() => void modQuery.refetch()}>{t("retry")}</Button>}
        />
      </Page>
    );
  }

  const selectedVersion = installVersion
    ? mod.versions.find((version) => version.id === installVersion)
    : undefined;

  function openExternal(url: string) {
    if (!url.startsWith("https://")) return;
    setPendingExternalUrl(url);
  }

  function confirmExternalUrl() {
    if (pendingExternalUrl) {
      try {
        BrowserOpenURL(pendingExternalUrl);
      } catch {
        window.open(pendingExternalUrl, "_blank", "noopener,noreferrer");
      }
    }
    setPendingExternalUrl(undefined);
  }

  async function copyWaxlightLink() {
    const url = modShareURL(modId);
    if (!url) {
      notify(t("invalid_waxlight_link"), "error");
      return;
    }
    try {
      if (!(await ClipboardSetText(url))) throw new Error("clipboard unavailable");
      notify(t("waxlight_link_copied"));
    } catch {
      notify(t("waxlight_link_copy_failed"), "error");
    }
  }

  const primaryLabel = local?.updateAvailable
    ? t("update_to_version", { version: local.latestVersion })
    : local
      ? t("install_to_instance")
      : t("download");
  const primaryVersionId = local?.updateAvailable
    ? mod.versions[0]?.id
    : (local?.versionId ?? mod.versions[0]?.id);
  const statusText = local?.updateAvailable
    ? `${local.downloadedVersion} → ${local.latestVersion}`
    : local
      ? local.installedInstances.length > 0
        ? t("installed_in_count", { count: local.installedInstances.length })
        : t("downloaded_not_installed")
      : null;

  return (
    <Page>
      <div className="mb-4">
        <Button variant="ghost" onClick={() => navigate(`/mods${from}`)}>
          <ArrowLeft size={16} aria-hidden="true" />
          {t("back_to_mods")}
        </Button>
      </div>

      <Card className="overflow-hidden">
        <div className="grid gap-6 p-6 lg:grid-cols-[200px_minmax(0,1fr)_auto] lg:items-center">
          <ModArtwork
            src={mod.imageUrl}
            alt={t("cover_alt", { name: mod.name })}
            seed={mod.name}
            className="aspect-[16/9] rounded-lg"
          />
          <div className="min-w-0 space-y-2">
            <p className="eyebrow">{t("vintage_story_moddb")}</p>
            <h1 className="font-display text-3xl font-semibold leading-tight">{mod.name}</h1>
            <p className="text-sm text-text-secondary">
              {t("by_author", { name: mod.authorName })}
            </p>
            <div className="flex flex-wrap items-center gap-2">
              <span className="rounded-full border border-border-subtle bg-surface-3 px-2 py-0.5 text-[11px] font-semibold text-text-secondary">
                {sideLabel(mod.side)}
              </span>
              {mod.gameVersions.slice(-3).map((version) => (
                <span
                  key={version}
                  className="rounded-full border border-border-subtle bg-surface-3 px-2 py-0.5 text-[11px] font-semibold text-text-secondary"
                >
                  {version}
                </span>
              ))}
              {local && (
                <span className="rounded-full border border-border-subtle bg-surface-3 px-2 py-0.5 text-[11px] font-semibold text-text-secondary">
                  {t("downloaded_version", { version: local.downloadedVersion })}
                </span>
              )}
            </div>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-text-muted">
              <span>
                {t("downloads_count", {
                  count: mod.downloads,
                  formatted: formatDownloads(mod.downloads),
                })}
              </span>
              <span>{t("updated_relative", { date: relativeDate(mod.updatedAt) })}</span>
            </div>
            {statusText && (
              <p
                className={
                  local?.updateAvailable
                    ? "text-xs font-medium text-warning"
                    : "text-xs font-medium text-text-secondary"
                }
              >
                {statusText}
              </p>
            )}
          </div>
          <div className="flex gap-2 lg:justify-end">
            <Button onClick={() => setInstallVersion(primaryVersionId)}>
              <Download size={16} aria-hidden="true" />
              {primaryLabel}
            </Button>
            <IconButton
              variant="ghost"
              aria-label={t("copy_waxlight_link")}
              onClick={() => void copyWaxlightLink()}
            >
              <Link size={16} aria-hidden="true" />
            </IconButton>
          </div>
        </div>
      </Card>

      <PageContent className="mt-6">
        {mod.screenshots.length > 0 && (
          <ModScreenshotCarousel
            key={mod.id}
            screenshots={mod.screenshots}
            modName={mod.name}
            onOpen={(index) => setLightbox(index)}
          />
        )}

        <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_290px]">
          <div className="flex min-w-0 flex-col gap-6">
            <Card>
              <CardHeader>
                <CardTitle>{t("description")}</CardTitle>
              </CardHeader>
              <CardContent>
                <ModDescription
                  description={mod.description}
                  fallback={mod.summary}
                  onOpenExternal={openExternal}
                />
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>{t("versions")}</CardTitle>
              </CardHeader>
              <CardContent>
                {mod.versions.length === 0 ? (
                  <p className="text-sm text-text-muted">{t("no_downloadable_releases")}</p>
                ) : (
                  <ul className="max-h-[60vh] divide-y divide-border-subtle overflow-y-auto pr-2">
                    {mod.versions.map((release) => (
                      <li
                        key={release.id}
                        className="flex flex-wrap items-center justify-between gap-3 py-4"
                      >
                        <div className="min-w-0 space-y-1">
                          <div className="flex flex-wrap items-center gap-2">
                            <strong className="text-sm">{release.version}</strong>
                            <span className="rounded-full border border-border-subtle bg-surface-3 px-2 py-0.5 text-[11px] font-semibold text-text-secondary">
                              {releaseTypeLabel(release.releaseType)}
                            </span>
                          </div>
                          <p className="text-xs text-text-muted">
                            {release.gameVersions.length > 0
                              ? formatGameVersions(release.gameVersions)
                              : t("compatibility_unknown")}
                            {release.publishedAt && (
                              <>
                                {" · "}
                                {t("updated_relative", { date: relativeDate(release.publishedAt) })}
                              </>
                            )}
                          </p>
                          {release.changelog && (
                            <p className="line-clamp-3 text-xs leading-5 text-text-muted">
                              {plainText(release.changelog)}
                            </p>
                          )}
                        </div>
                        <div className="flex items-center gap-3">
                          <span className="text-xs text-text-muted">
                            {formatBytes(release.fileSize)}
                          </span>
                          <Button variant="secondary" onClick={() => setInstallVersion(release.id)}>
                            {t("download")}
                          </Button>
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>

            {local?.installedInstances && local.installedInstances.length > 0 && (
              <Card>
                <CardHeader>
                  <CardTitle>{t("installed_in")}</CardTitle>
                </CardHeader>
                <CardContent>
                  <ul className="divide-y divide-border-subtle">
                    {local.installedInstances.map((installed) => (
                      <li
                        key={installed.instanceId}
                        className="flex flex-wrap items-center justify-between gap-3 py-3 text-sm"
                      >
                        <strong>{installed.instanceName}</strong>
                        <span className="flex flex-wrap items-center gap-3 text-xs text-text-muted">
                          <span>{t("version_value", { version: installed.version })}</span>
                          <span>{installed.enabled ? t("enabled") : t("disabled")}</span>
                        </span>
                      </li>
                    ))}
                  </ul>
                </CardContent>
              </Card>
            )}
          </div>

          <aside className="flex min-w-0 flex-col gap-6 lg:sticky lg:top-5">
            <Card>
              <CardHeader>
                <CardTitle>{t("information")}</CardTitle>
              </CardHeader>
              <CardContent>
                <dl className="divide-y divide-border-subtle">
                  <InfoRow label={t("author")} value={mod.authorName} />
                  {mod.latestVersion && (
                    <InfoRow label={t("latest_version")} value={mod.latestVersion} />
                  )}
                  <InfoRow label={t("side")} value={sideLabel(mod.side)} />
                  <InfoRow label={t("downloads")} value={formatDownloads(mod.downloads)} />
                  {mod.createdAt && (
                    <InfoRow
                      label={t("created")}
                      value={new Date(mod.createdAt).toLocaleDateString(i18n.resolvedLanguage)}
                    />
                  )}
                  {mod.updatedAt && (
                    <InfoRow
                      label={t("last_updated")}
                      value={new Date(mod.updatedAt).toLocaleDateString(i18n.resolvedLanguage)}
                    />
                  )}
                  {mod.license && <InfoRow label={t("license")} value={mod.license} />}
                </dl>
                {mod.tags.length > 0 && (
                  <div className="mt-4 flex flex-wrap gap-2">
                    {mod.tags.map((tag) => (
                      <span
                        key={tag}
                        className="rounded-md bg-surface-3 px-2 py-1 text-[11px] font-medium text-text-secondary"
                      >
                        {tag}
                      </span>
                    ))}
                  </div>
                )}
                <div className="mt-4 space-y-1">
                  {mod.websiteUrl && (
                    <Button
                      variant="ghost"
                      className="w-full justify-start"
                      onClick={() => openExternal(mod.websiteUrl!)}
                    >
                      <ExternalLink size={15} aria-hidden="true" />
                      {t("website_external")}
                    </Button>
                  )}
                  {mod.sourceUrl && (
                    <Button
                      variant="ghost"
                      className="w-full justify-start"
                      onClick={() => openExternal(mod.sourceUrl!)}
                    >
                      <ExternalLink size={15} aria-hidden="true" />
                      {t("source_code_external")}
                    </Button>
                  )}
                </div>
              </CardContent>
            </Card>
          </aside>
        </div>
      </PageContent>

      {selectedVersion && (
        <InstancePickerDialog
          mod={mod}
          downloaded={local?.versionId === selectedVersion.id ? local : undefined}
          instances={instances}
          gameVersions={versions}
          preferredInstanceId={instanceId}
          preferredVersionId={selectedVersion.id}
          onClose={() => setInstallVersion(undefined)}
          onDone={async () => {
            await queryClient.invalidateQueries({ queryKey: DOWNLOADED_MODS_QUERY_KEY });
            notify(t("mod_task_completed"));
          }}
        />
      )}

      <ConfirmDialog
        open={pendingExternalUrl !== undefined}
        title={t("open_external_link_confirmation")}
        message={pendingExternalUrl ?? ""}
        confirmLabel={t("open")}
        onConfirm={confirmExternalUrl}
        onCancel={() => setPendingExternalUrl(undefined)}
      />

      <Dialog
        open={lightbox !== undefined && mod.screenshots.length > 0}
        onOpenChange={(open) => {
          if (!open) setLightbox(undefined);
        }}
      >
        <DialogContent
          className="w-[min(960px,calc(100vw-48px))] bg-bg-app"
          aria-label={t("screenshot_viewer")}
        >
          {lightbox !== undefined && mod.screenshots[lightbox] && (
            <div className="flex items-center gap-4 px-4 py-6">
              <IconButton
                aria-label={t("previous_screenshot")}
                onClick={() =>
                  setLightbox((lightbox - 1 + mod.screenshots.length) % mod.screenshots.length)
                }
              >
                <ChevronLeft size={20} aria-hidden="true" />
              </IconButton>
              <figure className="min-w-0 flex-1 text-center">
                <img
                  src={mod.screenshots[lightbox].url}
                  alt={
                    mod.screenshots[lightbox].caption ||
                    t("screenshot_alt", { name: mod.name, number: lightbox + 1 })
                  }
                  className="mx-auto max-h-[70vh] max-w-full object-contain"
                />
                {mod.screenshots[lightbox].caption && (
                  <figcaption className="mt-3 text-sm text-text-muted">
                    {mod.screenshots[lightbox].caption}
                  </figcaption>
                )}
              </figure>
              <IconButton
                aria-label={t("next_screenshot")}
                onClick={() => setLightbox((lightbox + 1) % mod.screenshots.length)}
              >
                <ChevronRight size={20} aria-hidden="true" />
              </IconButton>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </Page>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-4 py-2.5 text-xs">
      <dt className="text-text-muted">{label}</dt>
      <dd className="text-right font-medium text-text-secondary">{value}</dd>
    </div>
  );
}
