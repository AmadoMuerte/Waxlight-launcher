import { Operation } from "../shared/api";
import { formatBytes, formatDate } from "../shared/lib";
import { Empty, PageHeader, StatusPill } from "../shared/ui";

interface OperationsPageProps {
  operations: Operation[];
}

export function OperationsPage({ operations }: OperationsPageProps) {
  return (
    <>
      <PageHeader
        eyebrow="Activity log"
        title="Operations"
        description="Installations and other long-running actions are kept here."
      />

      {operations.length === 0 ? (
        <Empty
          icon="⇣"
          title="No operations yet"
          description="Game and mod installation progress will appear here."
        />
      ) : (
        <div className="operationsList">
          {operations.map((operation) => (
            <article className="operation" key={operation.id}>
              <div className="operationIcon">
                {operation.type.startsWith("mod") ? "◇" : "⬡"}
              </div>

              <div className="operationDetails">
                <div className="row between">
                  <strong>{operation.title}</strong>
                  <StatusPill status={operation.status} />
                </div>

                <small>
                  {formatDate(operation.createdAt)} ·{" "}
                  {formatBytes(operation.currentBytes)}
                  {operation.totalBytes > 0
                    ? ` of ${formatBytes(operation.totalBytes)}`
                    : ""}
                </small>

                <div className="progress">
                  <i
                    style={{
                      width: `${Math.round(operation.progress * 100)}%`,
                    }}
                  />
                </div>

                {operation.errorMessage && (
                  <p className="errorText">{operation.errorMessage}</p>
                )}
              </div>
            </article>
          ))}
        </div>
      )}
    </>
  );
}
