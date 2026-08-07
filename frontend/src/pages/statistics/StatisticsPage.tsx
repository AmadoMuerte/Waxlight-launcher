import { useTranslation } from "react-i18next";

import { useInstancesQuery } from "../../entities/instance/queries";
import { useStatisticsQuery } from "../../entities/statistics/queries";
import { formatDate, formatDuration } from "../../shared/lib";
import { Empty, PageHeader } from "../../shared/ui";

export function StatisticsPage() {
  const { t } = useTranslation();
  const { data: statistics } = useStatisticsQuery();
  const { data: instances = [] } = useInstancesQuery();
  if (!statistics) {
    return <Empty title={t("loading_statistics")} description={t("collecting_playtime")} />;
  }

  const sessions = statistics.recentSessions ?? [];

  return (
    <>
      <PageHeader
        eyebrow={t("journey_history")}
        title={t("statistics")}
        description={t("statistics_description")}
      />

      <div className="statGrid">
        <article>
          <span>{t("total_playtime")}</span>
          <strong>{formatDuration(statistics.totalPlaytimeSeconds)}</strong>
          <small>{t("across_all_instances")}</small>
        </article>

        <article>
          <span>{t("launches")}</span>
          <strong>{statistics.launchCount}</strong>
          <small>{t("game_sessions")}</small>
        </article>

        <article>
          <span>{t("average_session")}</span>
          <strong>{formatDuration(statistics.averageSessionSeconds)}</strong>
          <small>{t("per_launch")}</small>
        </article>
      </div>

      <section className="panel">
        <h2>{t("recent_sessions")}</h2>

        {sessions.length === 0 ? (
          <Empty
            icon="◷"
            title={t("no_session_history")}
            description={t("first_session_description")}
          />
        ) : (
          <div className="sessionList">
            {sessions.map((session) => {
              const instance = instances.find((item) => item.id === session.instanceId);

              return (
                <div key={session.id}>
                  <span className={`sessionDot ${session.crashed ? "crashed" : ""}`} />
                  <div>
                    <strong>{instance?.name ?? t("removed_instance")}</strong>
                    <small>{formatDate(session.startedAt)}</small>
                  </div>
                  <b>{formatDuration(session.durationSeconds)}</b>
                  {session.crashed && <em>{t("crashed")}</em>}
                </div>
              );
            })}
          </div>
        )}
      </section>
    </>
  );
}
