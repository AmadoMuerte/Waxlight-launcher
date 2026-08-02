import { useEffect, useMemo, useState } from "react";

import {
  versionsApi,
  type AvailableGameVersion,
} from "../../shared/api";
import { errorMessage } from "../../shared/api/bridge";
import { formatBytes } from "../../shared/lib";
import { Button, Empty, Select, StatusPill } from "../../shared/ui";

type Notify = (message: string, type?: "ok" | "error") => void;

interface AvailableVersionsProps {
  installedVersionIDs: string[];
  notify: Notify;
  onOperationStarted: () => Promise<void>;
}

type ChannelFilter = "all" | "stable" | "unstable";

export function AvailableVersions({
  installedVersionIDs,
  notify,
  onOperationStarted,
}: AvailableVersionsProps) {
  const [versions, setVersions] = useState<AvailableGameVersion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const [channel, setChannel] = useState<ChannelFilter>("stable");
  const [visibleCount, setVisibleCount] = useState(20);
  const [startingVersionID, setStartingVersionID] = useState("");
  const installedKey = installedVersionIDs.join("|");

  useEffect(() => {
    let active = true;
    setLoading(true);
    versionsApi
      .available()
      .then((items) => {
        if (active) {
          setVersions(items ?? []);
          setError("");
        }
      })
      .catch((loadError: unknown) => {
        if (active) {
          setError(errorMessage(loadError));
        }
      })
      .finally(() => {
        if (active) {
          setLoading(false);
        }
      });
    return () => {
      active = false;
    };
  }, [installedKey]);

  const installed = useMemo(
    () => new Set(installedVersionIDs),
    [installedKey],
  );
  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase();
    return versions.filter((version) => {
      const matchesChannel =
        channel === "all" || version.channel === channel;
      const matchesSearch =
        query === "" || version.name.toLowerCase().includes(query);
      return matchesChannel && matchesSearch;
    });
  }, [channel, search, versions]);

  async function install(version: AvailableGameVersion) {
    setStartingVersionID(version.id);
    try {
      await versionsApi.installAvailable(version.id);
      await onOperationStarted();
      notify(`Downloading Vintage Story ${version.name}`);
    } catch (installError) {
      notify(errorMessage(installError), "error");
    } finally {
      setStartingVersionID("");
    }
  }

  if (loading) {
    return <div className="catalogState">Loading the official version catalog…</div>;
  }

  if (error) {
    return (
      <div className="catalogState errorText">
        <strong>Could not load available versions</strong>
        <span>{error}</span>
      </div>
    );
  }

  return (
    <section className="versionCatalog">
      <div className="sectionHeading">
        <div>
          <span className="eyebrow">Official releases</span>
          <h2>Available to download</h2>
        </div>
        <div className="versionFilters">
          <input
            type="search"
            value={search}
            onChange={(event) => {
              setSearch(event.target.value);
              setVisibleCount(20);
            }}
            placeholder="Search versions"
            aria-label="Search versions"
          />
          <Select
            value={channel}
            onChange={(event) => {
              setChannel(event.target.value as ChannelFilter);
              setVisibleCount(20);
            }}
            aria-label="Release channel"
          >
            <option value="stable">Stable</option>
            <option value="unstable">Preview and release candidates</option>
            <option value="all">All channels</option>
          </Select>
        </div>
      </div>

      {filtered.length === 0 ? (
        <Empty
          icon="⌕"
          title="No matching versions"
          description="Try another version number or release channel."
        />
      ) : (
        <>
          <div className="availableVersionList">
            {filtered.slice(0, visibleCount).map((version) => {
              const isInstalled = installed.has(version.id) || version.installed;
              return (
                <article className="availableVersion" key={version.id}>
                  <div className="versionIdentity">
                    <div className="row">
                      <strong>{version.name}</strong>
                      {version.latest && <span className="latestMark">Latest</span>}
                    </div>
                    <small>
                      {version.platform} · {version.architecture} ·{" "}
                      {formatBytes(version.downloadSize)}
                    </small>
                  </div>
                  <StatusPill status={version.channel} />
                  <Button
                    variant={isInstalled ? "ghost" : "secondary"}
                    busy={startingVersionID === version.id}
                    disabled={isInstalled}
                    onClick={() => void install(version)}
                  >
                    {isInstalled ? "Installed" : "Download"}
                  </Button>
                </article>
              );
            })}
          </div>
          {visibleCount < filtered.length && (
            <div className="loadMore">
              <Button
                variant="ghost"
                onClick={() => setVisibleCount((count) => count + 20)}
              >
                Show more versions
              </Button>
            </div>
          )}
        </>
      )}
    </section>
  );
}
