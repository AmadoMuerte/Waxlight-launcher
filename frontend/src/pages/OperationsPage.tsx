import { useState } from "react";
import { useTranslation } from "react-i18next";

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
  const { t } = useTranslation();
  const [pendingAction, setPendingAction] = useState<string>();
  const finishedOperations = operations.filter((operation) =>
    isFinishedOperation(operation),
  );

  async function cancel(operation: Operation) {
    setPendingAction(operation.id);
    try {
      await operationsApi.cancel(operation.id);
      await refresh();
      notify(t("operation_cancelled_removed"));
    } catch (cancelError) {
      notify(errorMessage(cancelError), "error");
    } finally {
      setPendingAction(undefined);
    }
  }

  async function remove(operation: Operation) {
    if (!window.confirm(t("delete_operation_confirmation", { title: operation.title }))) {
      return;
    }
    setPendingAction(operation.id);
    try {
      await operationsApi.remove(operation.id);
      await refresh();
      notify(t("operation_removed"));
    } catch (removeError) {
      notify(errorMessage(removeError), "error");
    } finally {
      setPendingAction(undefined);
    }
  }

  async function clearHistory() {
    if (
      !window.confirm(
        t("clear_history_confirmation"),
      )
    ) {
      return;
    }
    setPendingAction("clear-history");
    try {
      const removed = await operationsApi.clearHistory();
      await refresh();
      notify(t("operations_removed", { count: removed }));
    } catch (clearError) {
      notify(errorMessage(clearError), "error");
    } finally {
      setPendingAction(undefined);
    }
  }

  return (
    <>
      <PageHeader
        eyebrow={t("activity_log")}
        title={t("operations")}
        description={t("operations_description")}
        action={
          finishedOperations.length > 0 ? (
            <Button
              variant="secondary"
              disabled={pendingAction !== undefined}
              onClick={() => void clearHistory()}
            >
              {pendingAction === "clear-history" ? t("clearing") : t("clear_history")}
            </Button>
          ) : undefined
        }
      />

      {operations.length === 0 ? (
        <Empty
          icon="⇣"
          title={t("no_operations")}
          description={t("operations_empty_description")}
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
                        {pendingAction === operation.id ? t("cancelling") : t("cancel")}
                      </Button>
                    )}
                    {isFinishedOperation(operation) && (
                      <Button
                        variant="ghost"
                        aria-label={t("delete_operation", { title: operation.title })}
                        disabled={pendingAction !== undefined}
                        onClick={() => void remove(operation)}
                      >
                        {pendingAction === operation.id ? t("deleting") : t("delete")}
                      </Button>
                    )}
                  </div>
                </div>

                <small>
                  {formatDate(operation.createdAt)} ·{" "}
                  {operation.totalBytes > 0
                    ? t("bytes_of_total", { current: formatBytes(operation.currentBytes), total: formatBytes(operation.totalBytes) })
                    : formatBytes(operation.currentBytes)}
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
