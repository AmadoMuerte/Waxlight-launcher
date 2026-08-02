import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { BrowserOpenURL } from "../wailsjs/runtime/runtime";

import {
  modCatalogApi,
  type DownloadedMod,
  type GameVersion,
  type Instance,
  type ModDetails,
} from "../shared/api";
import { errorMessage } from "../shared/api/bridge";
import { Button, Empty } from "../shared/ui";
import { InstancePickerDialog } from "../features/mods/InstancePickerDialog";
import { ModArtwork } from "../features/mods/ModArtwork";
import {
  formatBytes,
  formatDownloads,
  plainText,
  relativeDate,
  sideLabel,
} from "../features/mods/lib";

type Notify = (message: string, type?: "ok" | "error") => void;

export function ModDetailsPage({
  instances,
  versions,
  notify,
}: {
  instances: Instance[];
  versions: GameVersion[];
  notify: Notify;
}) {
  const { modId = "" } = useParams();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [mod, setMod] = useState<ModDetails>();
  const [downloaded, setDownloaded] = useState<DownloadedMod[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [installVersion, setInstallVersion] = useState<string>();
  const [lightbox, setLightbox] = useState<number>();
  const from = searchParams.get("from") ?? "";
  const instanceId = new URLSearchParams(from).get("instanceId") ?? undefined;

  async function load() {
    setLoading(true);
    try {
      const [details, local] = await Promise.all([
        modCatalogApi.get(modId),
        modCatalogApi.downloaded(),
      ]);
      setMod(details);
      setDownloaded((local ?? []).filter((item) => item.modId === details.id));
      setError("");
    } catch (loadError) {
      setError(errorMessage(loadError));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [modId]);

  useEffect(() => {
    if (lightbox === undefined) return;
    function keydown(event: KeyboardEvent) {
      if (!mod) return;
      if (event.key === "Escape") setLightbox(undefined);
      if (event.key === "ArrowRight")
        setLightbox((index) => ((index ?? 0) + 1) % mod.screenshots.length);
      if (event.key === "ArrowLeft")
        setLightbox((index) => ((index ?? 0) - 1 + mod.screenshots.length) % mod.screenshots.length);
    }
    window.addEventListener("keydown", keydown);
    return () => window.removeEventListener("keydown", keydown);
  }, [lightbox, mod]);

  const local = useMemo(
    () => [...downloaded].sort((left, right) =>
      new Date(right.downloadedAt).getTime() - new Date(left.downloadedAt).getTime(),
    )[0],
    [downloaded],
  );

  if (loading) {
    return <div className="modDetailsSkeleton"><i /><i /><i /></div>;
  }
  if (error || !mod) {
    return (
      <Empty
        icon="!"
        title="Could not load this mod"
        description={error || "Mod not found"}
        action={<Button onClick={() => void load()}>Retry</Button>}
      />
    );
  }

  const selectedVersion = installVersion
    ? mod.versions.find((version) => version.id === installVersion)
    : undefined;

  function openExternal(url: string) {
    if (!url.startsWith("https://")) return;
    if (window.confirm(`Open this link in your browser?\n\n${url}`)) {
      try {
        BrowserOpenURL(url);
      } catch {
        window.open(url, "_blank", "noopener,noreferrer");
      }
    }
  }

  return (
    <>
      <button className="backToMods" onClick={() => navigate(`/mods${from}`)}>
        ← Back to Mods
      </button>

      <header className="modDetailsHeader">
        <ModArtwork src={mod.imageUrl} alt={`${mod.name} cover`} className="modDetailsArtwork" />
        <div className="modDetailsIdentity">
          <span className="eyebrow">Vintage Story ModDB</span>
          <h1>{mod.name}</h1>
          <p>by {mod.authorName}</p>
          <div className="detailBadges">
            <span className={`sideBadge side-${mod.side}`}>{sideLabel(mod.side)}</span>
            {mod.gameVersions.slice(-3).map((version) => <span key={version}>{version}</span>)}
          </div>
          <div className="detailStats">
            <span>↓ {formatDownloads(mod.downloads)} downloads</span>
            <span>Updated {relativeDate(mod.updatedAt)}</span>
          </div>
        </div>
        <div className="modPrimaryAction">
          <Button onClick={() => setInstallVersion(local?.updateAvailable ? mod.versions[0]?.id : local?.versionId ?? mod.versions[0]?.id)}>
            {local?.updateAvailable
              ? `Update to ${local.latestVersion}`
              : local
                ? "Install to instance"
                : "Download"}
          </Button>
          {local && <small>Downloaded {local.downloadedVersion}</small>}
        </div>
      </header>

      <div className="modDetailsLayout">
        <div className="modDetailsMain">
          <section className="detailSection">
            <h2>Description</h2>
            <div className="safeDescription">{plainText(mod.description) || mod.summary}</div>
          </section>

          {mod.screenshots.length > 0 && (
            <section className="detailSection">
              <h2>Screenshots</h2>
              <div className="screenshotGrid">
                {mod.screenshots.map((screenshot, index) => (
                  <button key={screenshot.url} onClick={() => setLightbox(index)}>
                    <img src={screenshot.url} alt={screenshot.caption || `${mod.name} screenshot ${index + 1}`} loading="lazy" />
                  </button>
                ))}
              </div>
            </section>
          )}

          <section className="detailSection">
            <h2>Versions</h2>
            {mod.versions.length === 0 ? (
              <p className="muted">No downloadable releases are available.</p>
            ) : (
              <div className="releaseList">
                {mod.versions.map((release) => (
                  <article key={release.id}>
                    <div>
                      <strong>{release.version}</strong>
                      <span className={`releaseType release-${release.releaseType}`}>{release.releaseType}</span>
                      <small>{release.gameVersions.join(", ") || "Compatibility unknown"}</small>
                    </div>
                    <div>
                      <span>{formatBytes(release.fileSize)}</span>
                      <Button variant="secondary" onClick={() => setInstallVersion(release.id)}>
                        Download
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
              <h2>Installed in</h2>
              <div className="installedInList">
                {local.installedInstances.map((installed) => (
                  <div key={installed.instanceId}>
                    <strong>{installed.instanceName}</strong>
                    <span>Version {installed.version}</span>
                    <span>{installed.enabled ? "Enabled" : "Disabled"}</span>
                  </div>
                ))}
              </div>
            </section>
          )}
        </div>

        <aside className="modInformation">
          <h2>Information</h2>
          <dl>
            <div><dt>Author</dt><dd>{mod.authorName}</dd></div>
            {mod.latestVersion && <div><dt>Latest version</dt><dd>{mod.latestVersion}</dd></div>}
            <div><dt>Side</dt><dd>{sideLabel(mod.side)}</dd></div>
            <div><dt>Downloads</dt><dd>{formatDownloads(mod.downloads)}</dd></div>
            {mod.createdAt && <div><dt>Created</dt><dd>{new Date(mod.createdAt).toLocaleDateString()}</dd></div>}
            {mod.updatedAt && <div><dt>Last updated</dt><dd>{new Date(mod.updatedAt).toLocaleDateString()}</dd></div>}
            {mod.license && <div><dt>License</dt><dd>{mod.license}</dd></div>}
          </dl>
          {mod.tags.length > 0 && <div className="detailTags">{mod.tags.map((tag) => <span key={tag}>{tag}</span>)}</div>}
          {mod.websiteUrl && <Button variant="ghost" onClick={() => openExternal(mod.websiteUrl!)}>Website ↗</Button>}
          {mod.sourceUrl && <Button variant="ghost" onClick={() => openExternal(mod.sourceUrl!)}>Source code ↗</Button>}
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
            const items = await modCatalogApi.downloaded();
            setDownloaded((items ?? []).filter((item) => item.modId === mod.id));
            notify("Mod task completed");
          }}
        />
      )}

      {lightbox !== undefined && mod.screenshots[lightbox] && (
        <div className="lightbox" role="dialog" aria-modal="true" aria-label="Screenshot viewer">
          <button className="lightboxClose" aria-label="Close" onClick={() => setLightbox(undefined)}>×</button>
          <button aria-label="Previous screenshot" onClick={() => setLightbox((lightbox - 1 + mod.screenshots.length) % mod.screenshots.length)}>‹</button>
          <figure>
            <img src={mod.screenshots[lightbox].url} alt={mod.screenshots[lightbox].caption || `${mod.name} screenshot`} />
            {mod.screenshots[lightbox].caption && <figcaption>{mod.screenshots[lightbox].caption}</figcaption>}
          </figure>
          <button aria-label="Next screenshot" onClick={() => setLightbox((lightbox + 1) % mod.screenshots.length)}>›</button>
        </div>
      )}
    </>
  );
}
