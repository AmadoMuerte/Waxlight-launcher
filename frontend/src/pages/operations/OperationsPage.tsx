import { useQueryClient } from "@tanstack/react-query";
import { ChevronDown, ChevronUp } from "lucide-react";
import { lazy, Suspense, useState } from "react";
import { useTranslation } from "react-i18next";

import { useToastStore } from "../../app/stores/toast";
import { operationsApi } from "../../entities/operation/api";
import type { Operation } from "../../entities/operation/model";
import { useOperationsQuery } from "../../entities/operation/queries";
import { useSettingsQuery } from "../../entities/settings/queries";
import { errorMessage } from "../../shared/api/bridge";
import { OPERATIONS_QUERY_KEY } from "../../shared/api/keys";
import { formatBytes, formatDate } from "../../shared/lib";
import { Button } from "../../shared/ui/button";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { Empty } from "../../shared/ui/empty";
import { PageHeader } from "../../shared/ui/page-header";
import { StatusPill } from "../../shared/ui/status-pill";

const LogConsole = lazy(() =>
  import("../../features/operations/LogConsole").then((module) => ({
    default: module.LogConsole,
  })),
);

export function OperationsPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  const { data: operations = [] } = useOperationsQuery();
  const { data: settings } = useSettingsQuery();
  const [pendingAction, setPendingAction] = useState<string>();
  const [consoleOpen, setConsoleOpen] = useState(true);
  const [confirmState, setConfirmState] = useState<{
    open: boolean;
    title: string;
    message?: string;
    destructive?: boolean;
    onConfirm: () => void;
  }>({ open: false, title: "", onConfirm: () => {} });
  const finishedOperations = operations.filter((operation) => isFinishedOperation(operation));

  function askConfirm(title: string, onConfirm: () => void, destructive = false, message?: string) {
    setConfirmState({ open: true, title, message, destructive, onConfirm });
  }

  async function cancel(operation: Operation) {
    setPendingAction(operation.id);
    try {
      await operationsApi.cancel(operation.id);
      await queryClient.invalidateQueries({ queryKey: OPERATIONS_QUERY_KEY });
      notify(t("operation_cancelled_removed"));
    } catch (cancelError) {
      notify(errorMessage(cancelError), "error");
    } finally {
      setPendingAction(undefined);
    }
  }

  function remove(operation: Operation) {
    if (settings?.confirmDeletion === false) {
      void (async () => {
        setPendingAction(operation.id);
        try {
          await operationsApi.remove(operation.id);
          await queryClient.invalidateQueries({ queryKey: OPERATIONS_QUERY_KEY });
          notify(t("operation_removed"));
        } catch (removeError) {
          notify(errorMessage(removeError), "error");
        } finally {
          setPendingAction(undefined);
        }
      })();
      return;
    }
    askConfirm(
      t("delete_operation_confirmation", { title: operation.title }),
      async () => {
        setPendingAction(operation.id);
        try {
          await operationsApi.remove(operation.id);
          await queryClient.invalidateQueries({ queryKey: OPERATIONS_QUERY_KEY });
          notify(t("operation_removed"));
        } catch (removeError) {
          notify(errorMessage(removeError), "error");
        } finally {
          setPendingAction(undefined);
        }
      },
      true,
    );
  }

  function clearHistory() {
    if (settings?.confirmDeletion === false) {
      void (async () => {
        setPendingAction("clear-history");
        try {
          const removed = await operationsApi.clearHistory();
          await queryClient.invalidateQueries({ queryKey: OPERATIONS_QUERY_KEY });
          notify(t("operations_removed", { count: removed }));
        } catch (clearError) {
          notify(errorMessage(clearError), "error");
        } finally {
          setPendingAction(undefined);
        }
      })();
      return;
    }
    askConfirm(
      t("clear_history"),
      async () => {
        setPendingAction("clear-history");
        try {
          const removed = await operationsApi.clearHistory();
          await queryClient.invalidateQueries({ queryKey: OPERATIONS_QUERY_KEY });
          notify(t("operations_removed", { count: removed }));
        } catch (clearError) {
          notify(errorMessage(clearError), "error");
        } finally {
          setPendingAction(undefined);
        }
      },
      true,
    );
  }

  return (
    <>
      <PageHeader
        eyebrow={t("activity_log")}
        title={t("operations")}
        description={t("operations_description")}
      />

      <div className="operationsSectionHead">
        <h2 className="sectionTitle">{t("activity_log")}</h2>
        {finishedOperations.length > 0 ? (
          <Button
            variant="secondary"
            disabled={pendingAction !== undefined}
            onClick={() => clearHistory()}
          >
            {pendingAction === "clear-history" ? t("clearing") : t("clear_history")}
          </Button>
        ) : undefined}
      </div>
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
              <div className="operationIcon">{operation.type.startsWith("mod") ? "◇" : "⬡"}</div>

              <div className="operationDetails">
                <div className="row between">
                  <strong>{operation.title}</strong>
                  <div className="row">
                    <StatusPill status={operation.status} />
                    {(operation.status === "queued" || operation.status === "running") && (
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
                        onClick={() => remove(operation)}
                      >
                        {pendingAction === operation.id ? t("deleting") : t("delete")}
                      </Button>
                    )}
                  </div>
                </div>

                <small>
                  {formatDate(operation.createdAt)} ·{" "}
                  {operation.totalBytes > 0
                    ? t("bytes_of_total", {
                        current: formatBytes(operation.currentBytes),
                        total: formatBytes(operation.totalBytes),
                      })
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

                {operation.errorMessage && <p className="errorText">{operation.errorMessage}</p>}
              </div>
            </article>
          ))}
        </div>
      )}

      <div className="operationsSection">
        <div className="operationsSectionHead">
          <h2 className="sectionTitle">{t("logs_console")}</h2>
          <Button
            variant="ghost"
            aria-expanded={consoleOpen}
            onClick={() => setConsoleOpen((open) => !open)}
          >
            {consoleOpen ? <ChevronUp size={15} /> : <ChevronDown size={15} />}
          </Button>
        </div>
        <div className={`logConsoleSlot ${consoleOpen ? "" : "collapsed"}`.trim()}>
          <Suspense fallback={<div className="logConsoleBody" />}>
            <LogConsole />
          </Suspense>
        </div>
      </div>

      <ConfirmDialog
        open={confirmState.open}
        title={confirmState.title}
        message={confirmState.message}
        destructive={confirmState.destructive}
        onConfirm={() => {
          setConfirmState((s) => ({ ...s, open: false }));
          confirmState.onConfirm();
        }}
        onCancel={() => setConfirmState((s) => ({ ...s, open: false }))}
      />
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
