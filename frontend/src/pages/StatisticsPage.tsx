import { Instance, Statistics } from "../shared/api";
import { formatDate, formatDuration } from "../shared/lib";
import { Empty, PageHeader } from "../shared/ui";

interface StatisticsPageProps {
  statistics?: Statistics;
  instances: Instance[];
}

export function StatisticsPage({
  statistics,
  instances,
}: StatisticsPageProps) {
  if (!statistics) {
    return (
      <Empty
        title="Loading statistics"
        description="Collecting your playtime…"
      />
    );
  }

  const sessions = statistics.recentSessions ?? [];

  return (
    <>
      <PageHeader
        eyebrow="Journey history"
        title="Statistics"
        description="Playtime is measured by the backend while the game process is alive."
      />

      <div className="statGrid">
        <article>
          <span>Total playtime</span>
          <strong>{formatDuration(statistics.totalPlaytimeSeconds)}</strong>
          <small>across all instances</small>
        </article>

        <article>
          <span>Launches</span>
          <strong>{statistics.launchCount}</strong>
          <small>game sessions</small>
        </article>

        <article>
          <span>Average session</span>
          <strong>{formatDuration(statistics.averageSessionSeconds)}</strong>
          <small>per launch</small>
        </article>
      </div>

      <section className="panel">
        <h2>Recent sessions</h2>

        {sessions.length === 0 ? (
          <Empty
            icon="◷"
            title="No session history yet"
            description="Your first successful game launch will appear here."
          />
        ) : (
          <div className="sessionList">
            {sessions.map((session) => {
              const instance = instances.find(
                (item) => item.id === session.instanceId,
              );

              return (
                <div key={session.id}>
                  <span
                    className={`sessionDot ${
                      session.crashed ? "crashed" : ""
                    }`}
                  />
                  <div>
                    <strong>{instance?.name ?? "Removed instance"}</strong>
                    <small>{formatDate(session.startedAt)}</small>
                  </div>
                  <b>{formatDuration(session.durationSeconds)}</b>
                  {session.crashed && <em>Crashed</em>}
                </div>
              );
            })}
          </div>
        )}
      </section>
    </>
  );
}
