import { Clock, History, Rocket, Star, Timer } from "lucide-react";
import { useTranslation } from "react-i18next";

import { useInstancesQuery } from "../../entities/instance/queries";
import { useStatisticsQuery } from "../../entities/statistics/queries";
import { StatCard } from "../../features/statistics/StatCard";
import { errorMessage } from "../../shared/api/bridge";
import { formatDate, formatDuration, formatNumber } from "../../shared/lib";
import { cn } from "../../shared/lib/utils";
import { Button } from "../../shared/ui/button";
import { Card } from "../../shared/ui/card";
import { Empty } from "../../shared/ui/empty";
import { ErrorState } from "../../shared/ui/error-state";
import { LoadingState } from "../../shared/ui/loading-state";
import { Page, PageContent, PageSection } from "../../shared/ui/page";
import { PageHeader } from "../../shared/ui/page-header";
import { SectionHeader } from "../../shared/ui/section-header";

export function StatisticsPage() {
  const { t } = useTranslation();
  const { data: statistics, isPending, isError, error, refetch } = useStatisticsQuery();
  const { data: instances = [] } = useInstancesQuery();
  const sessions = statistics?.recentSessions ?? [];

  const mostPlayedInstance = statistics?.mostPlayedInstanceId
    ? instances.find((item) => item.id === statistics.mostPlayedInstanceId)
    : undefined;

  return (
    <Page>
      <PageHeader
        eyebrow={t("journey_history")}
        title={t("statistics")}
        description={t("statistics_description")}
      />

      {isError ? (
        <PageContent>
          <ErrorState
            title={t("could_not_load_statistics")}
            description={errorMessage(error)}
            action={<Button onClick={() => void refetch()}>{t("retry")}</Button>}
          />
        </PageContent>
      ) : isPending || !statistics ? (
        <PageContent>
          <LoadingState>{t("loading_statistics")}</LoadingState>
        </PageContent>
      ) : (
        <PageContent>
          <PageSection>
            <SectionHeader variant="compact" title={t("summary")} />
            <div className="grid grid-cols-[repeat(auto-fill,minmax(min(220px,100%),1fr))] gap-4">
              <StatCard
                icon={Clock}
                label={t("total_playtime")}
                value={formatDuration(statistics.totalPlaytimeSeconds)}
                hint={t("across_all_instances")}
              />
              <StatCard
                icon={Rocket}
                label={t("launches")}
                value={formatNumber(statistics.launchCount)}
                hint={t("game_sessions")}
              />
              <StatCard
                icon={Timer}
                label={t("average_session")}
                value={formatDuration(statistics.averageSessionSeconds)}
                hint={t("per_launch")}
              />
              {statistics.mostPlayedInstanceId && (
                <StatCard
                  icon={Star}
                  variant="secondary"
                  label={t("most_played_instance")}
                  value={mostPlayedInstance?.name ?? t("removed_instance")}
                />
              )}
            </div>
          </PageSection>

          <PageSection>
            <SectionHeader variant="compact" title={t("recent_sessions")} />
            {sessions.length === 0 ? (
              <Empty
                icon={<History size={24} aria-hidden="true" />}
                title={t("no_session_history")}
                description={t("first_session_description")}
              />
            ) : (
              <Card variant="subtle" className="divide-y divide-border-subtle">
                {sessions.map((session) => {
                  const instance = instances.find((item) => item.id === session.instanceId);

                  return (
                    <div className="sessionRow" key={session.id}>
                      <span
                        className={cn("sessionDot", session.crashed && "crashed")}
                        aria-hidden="true"
                      />
                      <div className="min-w-0">
                        <strong className="block truncate">
                          {instance?.name ?? t("removed_instance")}
                        </strong>
                        <small className="text-xs text-text-muted">
                          {formatDate(session.startedAt)}
                        </small>
                      </div>
                      <span className="shrink-0 text-sm font-semibold">
                        {formatDuration(session.durationSeconds)}
                      </span>
                      {session.crashed && (
                        <em className="shrink-0 text-xs font-semibold not-italic text-danger">
                          {t("crashed")}
                        </em>
                      )}
                    </div>
                  );
                })}
              </Card>
            )}
          </PageSection>
        </PageContent>
      )}
    </Page>
  );
}
