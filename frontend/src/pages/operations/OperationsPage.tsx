import { useQueryClient } from "@tanstack/react-query";
import { ChevronDown, ChevronUp, ListTodo } from "lucide-react";
import { lazy, Suspense, useState } from "react";
import { useTranslation } from "react-i18next";

import { useToastStore } from "../../app/stores/toast";
import { operationsApi } from "../../entities/operation/api";
import type { Operation } from "../../entities/operation/model";
import { useOperationsQuery } from "../../entities/operation/queries";
import { useSettingsQuery } from "../../entities/settings/queries";
import {
  OperationItem,
  isFinishedOperation,
  operationTitle,
} from "../../features/operations/OperationItem";
import { errorMessage } from "../../shared/api/bridge";
import { OPERATIONS_QUERY_KEY } from "../../shared/api/keys";
import { Button } from "../../shared/ui/button";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { Empty } from "../../shared/ui/empty";
import { ErrorState } from "../../shared/ui/error-state";
import { IconButton } from "../../shared/ui/icon-button";
import { LoadingState } from "../../shared/ui/loading-state";
import { Page, PageContent, PageSection } from "../../shared/ui/page";
import { PageHeader } from "../../shared/ui/page-header";
import { SectionHeader } from "../../shared/ui/section-header";

const LogConsole = lazy(() =>
  import("../../features/operations/LogConsole").then((module) => ({
    default: module.LogConsole,
  })),
);

export function OperationsPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  const { data: operations = [], isPending, isError, error, refetch } = useOperationsQuery();
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

  const activeOperations = operations.filter(
    (operation) => operation.status === "queued" || operation.status === "running",
  );
  const finishedOperations = operations.filter(isFinishedOperation);

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
      t("delete_operation_confirmation", { title: operationTitle(operation, t) }),
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
    <Page>
      <PageHeader
        eyebrow={t("activity_log")}
        title={t("operations")}
        description={t("operations_description")}
      />

      <PageContent>
        {isPending ? (
          <LoadingState>{t("loading_operations")}</LoadingState>
        ) : isError ? (
          <ErrorState
            title={t("could_not_load_operations")}
            description={errorMessage(error)}
            action={<Button onClick={() => void refetch()}>{t("retry")}</Button>}
          />
        ) : operations.length === 0 ? (
          <Empty
            icon={<ListTodo size={28} aria-hidden="true" />}
            title={t("no_operations")}
            description={t("operations_empty_description")}
          />
        ) : (
          <>
            <PageSection>
              <SectionHeader title={t("active_operations")} />
              {activeOperations.length === 0 ? (
                <p className="operationEmptyHint">{t("no_active_operations")}</p>
              ) : (
                <div className="flex flex-col gap-3">
                  {activeOperations.map((operation) => (
                    <OperationItem
                      key={operation.id}
                      operation={operation}
                      actionsDisabled={pendingAction !== undefined}
                      onCancel={() => void cancel(operation)}
                    />
                  ))}
                </div>
              )}
            </PageSection>

            {finishedOperations.length > 0 && (
              <PageSection>
                <SectionHeader
                  title={t("recent_activity")}
                  actions={
                    <Button
                      variant="secondary"
                      disabled={pendingAction !== undefined}
                      onClick={() => clearHistory()}
                    >
                      {pendingAction === "clear-history" ? t("clearing") : t("clear_history")}
                    </Button>
                  }
                />
                <div className="max-h-[40vh] overflow-y-auto pr-1">
                  <div className="flex flex-col gap-3">
                    {finishedOperations.map((operation) => (
                      <OperationItem
                        key={operation.id}
                        operation={operation}
                        actionsDisabled={pendingAction !== undefined}
                        onRemove={() => remove(operation)}
                      />
                    ))}
                  </div>
                </div>
              </PageSection>
            )}
          </>
        )}

        <PageSection>
          <SectionHeader
            title={t("logs_console")}
            actions={
              <IconButton
                variant="ghost"
                aria-label={`${consoleOpen ? t("hide") : t("show")} ${t("logs_console")}`}
                aria-expanded={consoleOpen}
                onClick={() => setConsoleOpen((open) => !open)}
              >
                {consoleOpen ? (
                  <ChevronUp size={15} aria-hidden="true" />
                ) : (
                  <ChevronDown size={15} aria-hidden="true" />
                )}
              </IconButton>
            }
          />
          <div className={`logConsoleSlot ${consoleOpen ? "" : "collapsed"}`.trim()}>
            <Suspense fallback={<div className="logConsoleBody" />}>
              <LogConsole />
            </Suspense>
          </div>
        </PageSection>
      </PageContent>

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
    </Page>
  );
}
