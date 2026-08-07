import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui/select";

import { useToastStore } from "../../app/stores/toast";
import { versionsApi } from "../../entities/game-version/api";
import type { AvailableGameVersion } from "../../entities/game-version/model";
import { errorMessage } from "../../shared/api/bridge";
import { GAME_VERSIONS_QUERY_KEY, OPERATIONS_QUERY_KEY } from "../../shared/api/keys";
import { formatBytes } from "../../shared/lib";
import { Button } from "../../shared/ui/button";
import { Empty } from "../../shared/ui/empty";
import { StatusPill } from "../../shared/ui/status-pill";

interface AvailableVersionsProps {
  installedVersionIDs: string[];
}

type ChannelFilter = "all" | "stable" | "unstable";

export function AvailableVersions({ installedVersionIDs }: AvailableVersionsProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
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
    async function loadVersions() {
      setLoading(true);
      try {
        const items = await versionsApi.available();
        if (active) {
          setVersions(items ?? []);
          setError("");
        }
      } catch (loadError) {
        if (active) {
          setError(errorMessage(loadError));
        }
      } finally {
        if (active) {
          setLoading(false);
        }
      }
    }

    void loadVersions();
    return () => {
      active = false;
    };
  }, [installedKey]);

  const installed = useMemo(() => new Set(installedVersionIDs), [installedVersionIDs]);
  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase();
    return versions.filter((version) => {
      const matchesChannel = channel === "all" || version.channel === channel;
      const matchesSearch = query === "" || version.name.toLowerCase().includes(query);
      return matchesChannel && matchesSearch;
    });
  }, [channel, search, versions]);

  async function install(version: AvailableGameVersion) {
    setStartingVersionID(version.id);
    try {
      await versionsApi.installAvailable(version.id);
      await queryClient.invalidateQueries({ queryKey: GAME_VERSIONS_QUERY_KEY });
      await queryClient.invalidateQueries({ queryKey: OPERATIONS_QUERY_KEY });
      notify(t("downloading_version", { name: version.name }));
    } catch (installError) {
      notify(errorMessage(installError), "error");
    } finally {
      setStartingVersionID("");
    }
  }

  if (loading) {
    return <div className="catalogState">{t("loading_official_version_catalog")}</div>;
  }

  if (error) {
    return (
      <div className="catalogState errorText">
        <strong>{t("could_not_load_available_versions")}</strong>
        <span>{error}</span>
      </div>
    );
  }

  return (
    <section className="versionCatalog">
      <div className="sectionHeading">
        <div>
          <span className="eyebrow">{t("official_releases")}</span>
          <h2>{t("available_to_download")}</h2>
        </div>
        <div className="versionFilters">
          <input
            type="search"
            value={search}
            onChange={(event) => {
              setSearch(event.target.value);
              setVisibleCount(20);
            }}
            placeholder={t("search_versions")}
            aria-label={t("search_versions")}
          />
          <Select
            value={channel}
            onValueChange={(value) => {
              setChannel(channelFilter(value));
              setVisibleCount(20);
            }}
          >
            <SelectTrigger aria-label={t("release_channel")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="stable">{t("stable")}</SelectItem>
              <SelectItem value="unstable">{t("preview_and_release_candidates")}</SelectItem>
              <SelectItem value="all">{t("all_channels")}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {filtered.length === 0 ? (
        <Empty
          icon="⌕"
          title={t("no_matching_versions")}
          description={t("try_other_version_filter")}
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
                      {version.latest && <span className="latestMark">{t("latest")}</span>}
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
                    {isInstalled ? t("installed") : t("download")}
                  </Button>
                </article>
              );
            })}
          </div>
          {visibleCount < filtered.length && (
            <div className="loadMore">
              <Button variant="ghost" onClick={() => setVisibleCount((count) => count + 20)}>
                {t("show_more_versions")}
              </Button>
            </div>
          )}
        </>
      )}
    </section>
  );
}

function channelFilter(value: string): ChannelFilter {
  switch (value) {
    case "all":
    case "stable":
    case "unstable":
      return value;
    default:
      return "stable";
  }
}
