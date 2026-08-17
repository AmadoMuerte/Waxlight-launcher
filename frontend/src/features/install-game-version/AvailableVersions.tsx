import { useQueryClient } from "@tanstack/react-query";
import { Search } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui/select";

import { useToastStore } from "../../app/stores/toast";
import { versionsApi } from "../../entities/game-version/api";
import type { AvailableGameVersion } from "../../entities/game-version/model";
import { errorMessage } from "../../shared/api/bridge";
import { GAME_VERSIONS_QUERY_KEY, OPERATIONS_QUERY_KEY } from "../../shared/api/keys";
import { Button } from "../../shared/ui/button";
import { Card } from "../../shared/ui/card";
import { Empty } from "../../shared/ui/empty";
import { ErrorState } from "../../shared/ui/error-state";
import { Input } from "../../shared/ui/input";
import { LoadingState } from "../../shared/ui/loading-state";
import { PageSection } from "../../shared/ui/page";
import { SectionHeader } from "../../shared/ui/section-header";
import { Toolbar, ToolbarGroup } from "../../shared/ui/toolbar";
import { VersionItem } from "./VersionItem";

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
    return <LoadingState>{t("loading_official_version_catalog")}</LoadingState>;
  }

  if (error) {
    return <ErrorState title={t("could_not_load_available_versions")} description={error} />;
  }

  return (
    <PageSection className="versionCatalog">
      <SectionHeader
        eyebrow={t("official_releases")}
        title={t("available_to_download")}
        actions={
          <Toolbar className="versionToolbar">
            <ToolbarGroup>
              <Input
                type="search"
                className="w-[210px]"
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
                <SelectTrigger className="w-[220px]" aria-label={t("release_channel")}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="stable">{t("stable")}</SelectItem>
                  <SelectItem value="unstable">{t("preview_and_release_candidates")}</SelectItem>
                  <SelectItem value="all">{t("all_channels")}</SelectItem>
                </SelectContent>
              </Select>
            </ToolbarGroup>
          </Toolbar>
        }
      />

      {filtered.length === 0 ? (
        <Empty
          icon={<Search size={24} aria-hidden="true" />}
          title={t("no_matching_versions")}
          description={t("try_other_version_filter")}
        />
      ) : (
        <>
          <Card variant="subtle" className="divide-y divide-border-subtle">
            {filtered.slice(0, visibleCount).map((version) => {
              const isInstalled = installed.has(version.id) || version.installed;
              return (
                <VersionItem
                  key={version.id}
                  version={version}
                  installed={isInstalled}
                  busy={startingVersionID === version.id}
                  onInstall={() => void install(version)}
                />
              );
            })}
          </Card>
          {visibleCount < filtered.length && (
            <div className="flex justify-center pt-7">
              <Button variant="ghost" onClick={() => setVisibleCount((count) => count + 20)}>
                {t("show_more_versions")}
              </Button>
            </div>
          )}
        </>
      )}
    </PageSection>
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
