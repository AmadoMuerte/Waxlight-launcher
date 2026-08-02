import { useState } from "react";

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
  const [pendingAction, setPendingAction] = useState<string>();
  const finishedOperations = operations.filter((operation) =>
    isFinishedOperation(operation),
  );

  async function cancel(operation: Operation) {
    setPendingAction(operation.id);
    try {
      await operationsApi.cancel(operation.id);
      await refresh();
      notify("Operation cancelled and removed");
    } catch (cancelError) {
      notify(errorMessage(cancelError), "error");
    } finally {
      setPendingAction(undefined);
    }
  }

  async function remove(operation: Operation) {
    if (!window.confirm(`Delete “${operation.title}” from operation history?`)) {
      return;
    }
    setPendingAction(operation.id);
    try {
      await operationsApi.remove(operation.id);
      await refresh();
      notify("Operation removed from history");
    } catch (removeError) {
      notify(errorMessage(removeError), "error");
    } finally {
      setPendingAction(undefined);
    }
  }

  async function clearHistory() {
    if (
      !window.confirm(
        "Clear all finished operations from history? Active operations will be kept.",
      )
    ) {
      return;
    }
    setPendingAction("clear-history");
    try {
      const removed = await operationsApi.clearHistory();
      await refresh();
      notify(
        removed === 1
          ? "1 operation removed from history"
          : `${removed} operations removed from history`,
      );
    } catch (clearError) {
      notify(errorMessage(clearError), "error");
    } finally {
      setPendingAction(undefined);
    }
  }

  return (
    <>
      <PageHeader
        eyebrow="Activity log"
        title="Operations"
        description="Installations and other long-running actions are kept here."
        action={
          finishedOperations.length > 0 ? (
            <Button
              variant="secondary"
              disabled={pendingAction !== undefined}
              onClick={() => void clearHistory()}
            >
              {pendingAction === "clear-history" ? "Clearing…" : "Clear history"}
            </Button>
          ) : undefined
        }
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
                        disabled={pendingAction !== undefined}
                        onClick={() => void cancel(operation)}
                      >
                        {pendingAction === operation.id ? "Cancelling…" : "Cancel"}
                      </Button>
                    )}
                    {isFinishedOperation(operation) && (
                      <Button
                        variant="ghost"
                        aria-label={`Delete ${operation.title}`}
                        disabled={pendingAction !== undefined}
                        onClick={() => void remove(operation)}
                      >
                        {pendingAction === operation.id ? "Deleting…" : "Delete"}
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

export function isFinishedOperation(operation: Operation): boolean {
  return (
    operation.status === "completed" ||
    operation.status === "failed" ||
    operation.status === "cancelled"
  );
}
