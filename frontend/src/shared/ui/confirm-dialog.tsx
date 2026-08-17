import { Info, TriangleAlert } from "lucide-react";
import { useId } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "../../shared/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "./dialog";

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  message?: string;
  warningMessage?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  destructive?: boolean;
  loading?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmDialog({
  open,
  title,
  message,
  warningMessage,
  confirmLabel,
  cancelLabel,
  destructive = false,
  loading = false,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const { t } = useTranslation();
  const descriptionId = useId();

  const hasMessage = Boolean(message && message !== title);
  const hasWarning = Boolean(warningMessage);

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen && !loading) {
          onCancel();
        }
      }}
    >
      <DialogContent
        aria-describedby={hasMessage ? descriptionId : undefined}
        style={{
          width: "min(640px, calc(100vw - 32px))",
          maxWidth: "none",
        }}
        className="gap-0 overflow-hidden p-0"
      >
        <div className="px-6 pb-2 pt-8 pr-14">
          <div className="grid grid-cols-[auto_minmax(0,1fr)] items-start gap-5">
            <div className="flex h-14 w-14 items-center justify-center rounded-full border border-accent-muted bg-accent-muted/40 text-accent-hover">
              <TriangleAlert className="size-7" aria-hidden="true" />
            </div>

            <div className="min-w-0 pt-1">
              <DialogTitle className="font-sans text-2xl font-semibold leading-snug text-text-primary">
                {title}
              </DialogTitle>

              {hasMessage && (
                <p
                  id={descriptionId}
                  className="mt-3 max-w-[470px] text-sm leading-6 text-text-secondary"
                >
                  {message}
                </p>
              )}
            </div>
          </div>
        </div>

        {hasWarning && (
          <div className="px-6 pt-6">
            <div className="grid grid-cols-[24px_minmax(0,1fr)] items-start gap-3 rounded-lg border border-warning-border bg-warning-surface px-4 py-3 text-warning">
              <div className="mt-0.5">
                <Info className="size-5" aria-hidden="true" />
              </div>

              <p className="text-[13px] leading-6">{warningMessage}</p>
            </div>
          </div>
        )}

        <DialogFooter className="px-6 pb-6 pt-7">
          <Button type="button" variant="ghost" disabled={loading} onClick={onCancel}>
            {cancelLabel ?? t("cancel")}
          </Button>

          <Button
            type="button"
            variant={destructive ? "danger" : "primary"}
            disabled={loading}
            onClick={onConfirm}
          >
            {loading
              ? t("processing", {
                  defaultValue: "Processing…",
                })
              : (confirmLabel ??
                (destructive
                  ? t("delete", {
                      defaultValue: "Delete",
                    })
                  : t("confirm", {
                      defaultValue: "Confirm",
                    })))}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
