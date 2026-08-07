import { useQueryClient } from "@tanstack/react-query";
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
import { errorMessage } from "../../shared/api/bridge";
import { DOWNLOADED_MODS_QUERY_KEY } from "../../shared/api/keys";
import { Button } from "../../shared/ui/button";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { Empty } from "../../shared/ui/empty";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";

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
      <div className="modDetailsSkeleton">
        <i />
        <i />
        <i />
      </div>
    );
  }
  if (error || !mod) {
    return (
      <Empty
        icon="!"
        title={t("could_not_load_mod")}
        description={error || t("mod_not_found")}
        action={<Button onClick={() => void modQuery.refetch()}>{t("retry")}</Button>}
      />
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

  return (
    <>
      <button className="backToMods" onClick={() => navigate(`/mods${from}`)}>
        ← {t("back_to_mods")}
      </button>

      <header className="modDetailsHeader">
        <ModArtwork
          src={mod.imageUrl}
          alt={t("cover_alt", { name: mod.name })}
          className="modDetailsArtwork"
        />
        <div className="modDetailsIdentity">
          <span className="eyebrow">{t("vintage_story_moddb")}</span>
          <h1>{mod.name}</h1>
          <p>{t("by_author", { name: mod.authorName })}</p>
          <div className="detailBadges">
            <span className={`sideBadge side-${mod.side}`}>{sideLabel(mod.side)}</span>
            {mod.gameVersions.slice(-3).map((version) => (
              <span key={version}>{version}</span>
            ))}
          </div>
          <div className="detailStats">
            <span>
              {t("downloads_count", {
                count: mod.downloads,
                formatted: formatDownloads(mod.downloads),
              })}
            </span>
            <span>{t("updated_relative", { date: relativeDate(mod.updatedAt) })}</span>
          </div>
        </div>
        <div className="modPrimaryAction">
          <Button
            onClick={() =>
              setInstallVersion(
                local?.updateAvailable
                  ? mod.versions[0]?.id
                  : (local?.versionId ?? mod.versions[0]?.id),
              )
            }
          >
            {local?.updateAvailable
              ? t("update_to_version", { version: local.latestVersion })
              : local
                ? t("install_to_instance")
                : t("download")}
          </Button>
          {local && <small>{t("downloaded_version", { version: local.downloadedVersion })}</small>}
        </div>
      </header>

      <div className="modDetailsLayout">
        <div className="modDetailsMain">
          <section className="detailSection">
            <h2>{t("description")}</h2>
            <div className="safeDescription">{plainText(mod.description) || mod.summary}</div>
          </section>

          {mod.screenshots.length > 0 && (
            <section className="detailSection">
              <h2>{t("screenshots")}</h2>
              <div className="screenshotGrid">
                {mod.screenshots.map((screenshot, index) => (
                  <button key={screenshot.url} onClick={() => setLightbox(index)}>
                    <img
                      src={screenshot.url}
                      alt={
                        screenshot.caption ||
                        t("screenshot_alt", { name: mod.name, number: index + 1 })
                      }
                      loading="lazy"
                    />
                  </button>
                ))}
              </div>
            </section>
          )}

          <section className="detailSection">
            <h2>{t("versions")}</h2>
            {mod.versions.length === 0 ? (
              <p className="muted">{t("no_downloadable_releases")}</p>
            ) : (
              <div className="releaseList">
                {mod.versions.map((release) => (
                  <article key={release.id}>
                    <div>
                      <strong>{release.version}</strong>
                      <span className={`releaseType release-${release.releaseType}`}>
                        {releaseTypeLabel(release.releaseType)}
                      </span>
                      <small>
                        {release.gameVersions.length > 0
                          ? formatGameVersions(release.gameVersions)
                          : t("compatibility_unknown")}
                      </small>
                    </div>
                    <div>
                      <span>{formatBytes(release.fileSize)}</span>
                      <Button variant="secondary" onClick={() => setInstallVersion(release.id)}>
                        {t("download")}
                      </Button>
                    </div>
                    {release.changelog && <p>{plainText(release.changelog)}</p>}
                  </article>
                ))}
              </div>
            )}
          </section>

          {local?.installedInstances && local.installedInstances.length > 0 && (
            <section className="detailSection">
              <h2>{t("installed_in")}</h2>
              <div className="installedInList">
                {local.installedInstances.map((installed) => (
                  <div key={installed.instanceId}>
                    <strong>{installed.instanceName}</strong>
                    <span>{t("version_value", { version: installed.version })}</span>
                    <span>{installed.enabled ? t("enabled") : t("disabled")}</span>
                  </div>
                ))}
              </div>
            </section>
          )}
        </div>

        <aside className="modInformation">
          <h2>{t("information")}</h2>
          <dl>
            <div>
              <dt>{t("author")}</dt>
              <dd>{mod.authorName}</dd>
            </div>
            {mod.latestVersion && (
              <div>
                <dt>{t("latest_version")}</dt>
                <dd>{mod.latestVersion}</dd>
              </div>
            )}
            <div>
              <dt>{t("side")}</dt>
              <dd>{sideLabel(mod.side)}</dd>
            </div>
            <div>
              <dt>{t("downloads")}</dt>
              <dd>{formatDownloads(mod.downloads)}</dd>
            </div>
            {mod.createdAt && (
              <div>
                <dt>{t("created")}</dt>
                <dd>{new Date(mod.createdAt).toLocaleDateString(i18n.resolvedLanguage)}</dd>
              </div>
            )}
            {mod.updatedAt && (
              <div>
                <dt>{t("last_updated")}</dt>
                <dd>{new Date(mod.updatedAt).toLocaleDateString(i18n.resolvedLanguage)}</dd>
              </div>
            )}
            {mod.license && (
              <div>
                <dt>{t("license")}</dt>
                <dd>{mod.license}</dd>
              </div>
            )}
          </dl>
          {mod.tags.length > 0 && (
            <div className="detailTags">
              {mod.tags.map((tag) => (
                <span key={tag}>{tag}</span>
              ))}
            </div>
          )}
          {mod.websiteUrl && (
            <Button variant="ghost" onClick={() => openExternal(mod.websiteUrl!)}>
              {t("website_external")}
            </Button>
          )}
          {mod.sourceUrl && (
            <Button variant="ghost" onClick={() => openExternal(mod.sourceUrl!)}>
              {t("source_code_external")}
            </Button>
          )}
        </aside>
      </div>

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

      {lightbox !== undefined && mod.screenshots[lightbox] && (
        <dialog className="lightbox" open aria-modal="true" aria-label={t("screenshot_viewer")}>
          <button
            className="lightboxClose"
            aria-label={t("close")}
            onClick={() => setLightbox(undefined)}
          >
            ×
          </button>
          <button
            aria-label={t("previous_screenshot")}
            onClick={() =>
              setLightbox((lightbox - 1 + mod.screenshots.length) % mod.screenshots.length)
            }
          >
            ‹
          </button>
          <figure>
            <img
              src={mod.screenshots[lightbox].url}
              alt={
                mod.screenshots[lightbox].caption ||
                t("screenshot_alt", { name: mod.name, number: lightbox + 1 })
              }
            />
            {mod.screenshots[lightbox].caption && (
              <figcaption>{mod.screenshots[lightbox].caption}</figcaption>
            )}
          </figure>
          <button
            aria-label={t("next_screenshot")}
            onClick={() => setLightbox((lightbox + 1) % mod.screenshots.length)}
          >
            ›
          </button>
        </dialog>
      )}
    </>
  );
}
