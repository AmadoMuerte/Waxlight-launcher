import { useId } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "../../shared/ui";
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

function WarningIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" className="h-8 w-8" fill="none">
      <path d="M12 7.25v6.25" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />

      <circle cx="12" cy="17" r="1.15" fill="currentColor" />
    </svg>
  );
}

function InfoIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" className="h-6 w-6" fill="none">
      <path
        d="M12 3.5 19.25 6.4v5.2c0 4.1-2.58 7.2-7.25 9.15-4.67-1.95-7.25-5.05-7.25-9.15V6.4L12 3.5Z"
        stroke="currentColor"
        strokeWidth="1.7"
        strokeLinejoin="round"
      />

      <path d="M12 8.2v4.9" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />

      <circle cx="12" cy="16.15" r="1" fill="currentColor" />
    </svg>
  );
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
        className="
          gap-0
          overflow-hidden
          rounded-[28px]
          border
          border-white/[0.08]
          bg-[linear-gradient(145deg,#211e1d_0%,#191819_58%,#151416_100%)]
          p-0
          text-[var(--text-primary)]
          shadow-[0_36px_110px_rgba(0,0,0,0.68),0_0_70px_rgba(201,103,43,0.08)]

          [&>button]:right-5
          [&>button]:top-5
          [&>button]:flex
          [&>button]:h-11
          [&>button]:w-11
          [&>button]:items-center
          [&>button]:justify-center
          [&>button]:rounded-[14px]
          [&>button]:border
          [&>button]:border-white/[0.06]
          [&>button]:bg-white/[0.035]
          [&>button]:text-[var(--text-muted)]
          [&>button]:opacity-100
          [&>button]:transition
          [&>button]:duration-150
          [&>button:hover]:border-white/[0.11]
          [&>button:hover]:bg-white/[0.075]
          [&>button:hover]:text-white
          [&>button:disabled]:pointer-events-none
          [&>button:disabled]:opacity-40
        "
      >
        <div className="px-8 pb-0 pt-8 pr-20">
          <div className="grid grid-cols-[64px_minmax(0,1fr)] items-start gap-5">
            <div
              className="
                flex
                h-16
                w-16
                items-center
                justify-center
                rounded-full
                border
                border-[#8b5229]
                bg-[radial-gradient(circle_at_50%_35%,rgba(238,145,71,0.21),rgba(91,49,25,0.13))]
                text-[#eda05e]
                shadow-[inset_0_1px_0_rgba(255,255,255,0.05),0_0_32px_rgba(222,115,48,0.08)]
              "
            >
              <WarningIcon />
            </div>

            <div className="min-w-0 pt-1">
              <DialogTitle
                className="
                  font-sans
                  text-[26px]
                  font-semibold
                  leading-[1.28]
                  tracking-[-0.025em]
                  text-[#f3efec]
                "
              >
                {title}
              </DialogTitle>

              {hasMessage && (
                <p
                  id={descriptionId}
                  className="
                    mt-4
                    max-w-[470px]
                    text-[15px]
                    leading-7
                    text-[#aaa4a1]
                  "
                >
                  {message}
                </p>
              )}
            </div>
          </div>
        </div>

        {hasWarning && (
          <div className="px-8 pt-7">
            <div
              className="
                grid
                grid-cols-[28px_minmax(0,1fr)]
                items-start
                gap-4
                rounded-[18px]
                border
                border-[#714429]/70
                bg-[linear-gradient(135deg,rgba(106,59,30,0.27),rgba(62,39,27,0.18))]
                px-5
                py-4
                text-[#e6a16a]
                shadow-[inset_0_1px_0_rgba(255,255,255,0.025)]
              "
            >
              <div className="mt-0.5">
                <InfoIcon />
              </div>

              <p className="text-[14px] leading-6">{warningMessage}</p>
            </div>
          </div>
        )}

        <DialogFooter
          className="
            flex
            flex-row
            justify-end
            gap-3
            border-none
            bg-transparent
            px-8
            pb-8
            pt-7
          "
        >
          <Button
            type="button"
            variant="ghost"
            disabled={loading}
            onClick={onCancel}
            className="
              h-12
              min-w-[120px]
              rounded-[14px]
              border
              border-white/[0.09]
              bg-white/[0.025]
              px-6
              text-[15px]
              font-medium
              text-[#d2ccca]
              shadow-none
              transition
              hover:border-white/[0.14]
              hover:bg-white/[0.07]
              hover:text-white
            "
          >
            {cancelLabel ?? t("cancel")}
          </Button>

          <Button
            type="button"
            variant={destructive ? "danger" : "primary"}
            disabled={loading}
            onClick={onConfirm}
            className="
              h-12
              min-w-[140px]
              rounded-[14px]
              px-6
              text-[15px]
              font-semibold
              shadow-[0_10px_28px_rgba(151,54,39,0.2)]
              transition
            "
          >
            {loading
              ? t("processing", {
                  defaultValue: "Подождите…",
                })
              : (confirmLabel ??
                (destructive
                  ? t("delete", {
                      defaultValue: "Удалить",
                    })
                  : t("confirm", {
                      defaultValue: "Подтвердить",
                    })))}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
