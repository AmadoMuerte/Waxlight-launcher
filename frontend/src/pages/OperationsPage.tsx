import { Operation, operationsApi } from "../shared/api";
import { errorMessage } from "../shared/api/bridge";
import { formatBytes, formatDate } from "../shared/lib";
import { Button, Empty, PageHeader, StatusPill } from "../shared/ui";

interface OperationsPageProps {
  operations: Operation[];
  refresh: () => Promise<void>;
  notify: (message: string, type?: "ok" | "error") => void;
}

export function OperationsPage({
  operations,
  refresh,
  notify,
}: OperationsPageProps) {
  async function cancel(operation: Operation) {
    try {
      await operationsApi.cancel(operation.id);
      await refresh();
      notify("Operation cancelled");
    } catch (cancelError) {
      notify(errorMessage(cancelError), "error");
    }
  }

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
                  <div className="row">
                    <StatusPill status={operation.status} />
                    {(operation.status === "queued" ||
                      operation.status === "running") && (
                      <Button
                        variant="ghost"
                        onClick={() => void cancel(operation)}
                      >
                        Cancel
                      </Button>
                    )}
                  </div>
                </div>

                <small>
                  {formatDate(operation.createdAt)} ·{" "}
                  {formatBytes(operation.currentBytes)}
                  {operation.totalBytes > 0
                    ? ` of ${formatBytes(operation.totalBytes)}`
                    : ""}
                  {operation.bytesPerSecond > 0
                    ? ` · ${formatBytes(operation.bytesPerSecond)}/s`
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
