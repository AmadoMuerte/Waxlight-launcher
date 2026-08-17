import {
  Camera,
  Download,
  ListTodo,
  Package,
  RefreshCw,
  Trash2,
  Upload,
  X,
  type LucideIcon,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { cn } from "@/shared/lib/utils";

import type { Operation } from "../../entities/operation/model";
import { formatBytes, formatDate } from "../../shared/lib";
import { Card } from "../../shared/ui/card";
import { IconButton } from "../../shared/ui/icon-button";
import { Progress } from "../../shared/ui/progress";
import { StatusPill } from "../../shared/ui/status-pill";

interface OperationItemProps {
  operation: Operation;
  /** Disables all row actions while any operation action is in flight. */
  actionsDisabled?: boolean;
  onCancel?: () => void;
  onRemove?: () => void;
}

export function OperationItem({
  operation,
  actionsDisabled = false,
  onCancel,
  onRemove,
}: OperationItemProps) {
  const { t } = useTranslation();
  const title = operationTitle(operation, t);
  const active = operation.status === "running" || operation.status === "queued";
  const finished = isFinishedOperation(operation);
  const percent = Math.round(operation.progress * 100);

  const Icon = operationIcon(operation.type);

  return (
    <Card variant="subtle" className="flex items-start gap-3 p-4">
      <div className={cn("operationItemIcon", active && "active")}>
        <Icon size={18} aria-hidden="true" />
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex items-start justify-between gap-3">
          <strong className="operationItemTitle">{title}</strong>
          <div className="flex shrink-0 items-center gap-2">
            <StatusPill status={operation.status} />
            {active && (
              <IconButton
                variant="ghost"
                size="sm"
                aria-label={t("cancel")}
                disabled={actionsDisabled}
                onClick={onCancel}
              >
                <X size={15} aria-hidden="true" />
              </IconButton>
            )}
            {finished && (
              <IconButton
                variant="danger"
                size="sm"
                aria-label={t("delete_operation", { title })}
                disabled={actionsDisabled}
                onClick={onRemove}
              >
                <Trash2 size={15} aria-hidden="true" />
              </IconButton>
            )}
          </div>
        </div>

        <small className="operationItemMeta">
          {formatDate(operation.createdAt)}
          {active && operation.totalBytes > 0
            ? ` · ${t("bytes_of_total", {
                current: formatBytes(operation.currentBytes),
                total: formatBytes(operation.totalBytes),
              })}`
            : ""}
          {active && operation.bytesPerSecond > 0
            ? ` · ${formatBytes(operation.bytesPerSecond)}/s`
            : ""}
        </small>

        {operation.status === "running" && (
          <div className="operationItemProgress">
            <Progress value={percent} className="min-w-0 flex-1" />
            <span className="operationItemPercent">{percent}%</span>
          </div>
        )}

        {operation.status === "failed" && operation.errorMessage && (
          <p className="operationItemError">{operation.errorMessage}</p>
        )}
      </div>
    </Card>
  );
}

export function isFinishedOperation(operation: Operation): boolean {
  return (
    operation.status === "completed" ||
    operation.status === "failed" ||
    operation.status === "cancelled"
  );
}

// operationIcon picks a Lucide icon from the operation type, keeping the
// icon family small so state stays the dominant signal.
function operationIcon(type: string): LucideIcon {
  if (type.includes("restore") || type.includes("update")) return RefreshCw;
  if (type.includes("snapshot")) return Camera;
  if (type.includes("download")) return Download;
  if (type.includes("install")) return Package;
  if (type.includes("import")) return Upload;
  return ListTodo;
}

// operationTitle renders an operation title through the i18n system when the
// backend provided a translation key, falling back to the stored English
// title for legacy operations.
export function operationTitle(
  operation: Operation,
  t: (key: string, options?: Record<string, unknown>) => string,
): string {
  if (!operation.titleKey) {
    return operation.title;
  }
  return t(operation.titleKey, {
    defaultValue: operation.title,
    ...operation.titleParams,
  });
}
