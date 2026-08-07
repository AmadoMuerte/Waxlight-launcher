import type { ButtonHTMLAttributes, InputHTMLAttributes, FormEvent, ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { Checkbox as RadixCheckbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";

type ButtonVariant = "primary" | "secondary" | "danger" | "ghost";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  variant?: ButtonVariant;
  busy?: boolean;
}

export function Spinner({ className = "" }: { className?: string }) {
  return <span className={`spinner ${className}`.trim()} />;
}

export function Button({
  children,
  variant = "primary",
  busy = false,
  className = "",
  disabled = false,
  ...props
}: ButtonProps) {
  return (
    <button
      {...props}
      className={`button ${variant} ${busy ? "busy" : ""} ${className}`.trim()}
      disabled={busy || disabled}
    >
      <span className="buttonLabel" aria-hidden={busy}>
        {children}
      </span>
      {busy && <Spinner className="buttonSpinner" />}
    </button>
  );
}

export function Modal({
  title,
  onClose,
  children,
  className = "",
}: {
  title: string;
  onClose: () => void;
  children: ReactNode;
  className?: string;
}) {
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className={className} aria-label={title}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        {children}
      </DialogContent>
    </Dialog>
  );
}

export function Checkbox({
  label,
  className = "",
  onChange,
  checked,
}: Omit<InputHTMLAttributes<HTMLInputElement>, "type"> & {
  label: string;
}) {
  return (
    <label className={`checkboxControl ${className}`.trim()}>
      <RadixCheckbox
        checked={checked}
        onCheckedChange={(radixChecked) => {
          // Adapt Radix boolean callback to the native checkbox onChange API expected by consumers.
          // oxlint-disable-next-line typescript/no-unsafe-type-assertion -- intentional adapter between Radix and native checkbox APIs
          onChange?.({
            target: { checked: radixChecked === true },
          } as React.ChangeEvent<HTMLInputElement>);
        }}
      />
      <span>{label}</span>
    </label>
  );
}

export function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
      {hint && <small>{hint}</small>}
    </label>
  );
}

export function Empty({
  icon = "✦",
  title,
  description,
  action,
}: {
  icon?: string;
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <div className="empty">
      <div className="emptyIcon">{icon}</div>
      <h2>{title}</h2>
      <p>{description}</p>
      {action}
    </div>
  );
}

export function PageHeader({
  eyebrow,
  title,
  description,
  action,
}: {
  eyebrow?: string;
  title: string;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <header className="pageHeader">
      <div>
        {eyebrow && <span className="eyebrow">{eyebrow}</span>}
        <h1>{title}</h1>
        {description && <p>{description}</p>}
      </div>
      {action}
    </header>
  );
}

export function SubmitForm({
  onSubmit,
  children,
  className = "",
  noValidate = false,
}: {
  onSubmit: () => Promise<void>;
  children: ReactNode;
  className?: string;
  noValidate?: boolean;
}) {
  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    await onSubmit();
  }

  return (
    <form className={className} noValidate={noValidate} onSubmit={handleSubmit}>
      {children}
    </form>
  );
}

export function StatusPill({ status }: { status: string }) {
  const { t } = useTranslation();
  const labels: Record<string, string> = {
    ready: "status_ready",
    running: "status_running",
    installed: "status_installed",
    queued: "status_queued",
    completed: "status_completed",
    cancelled: "status_cancelled",
    failed: "status_failed",
    stable: "stable",
    unstable: "preview",
    unknown: "status_unknown",
    local_profile: "status_local_profile",
    valid: "status_valid",
    expired: "status_expired",
    needs_reauth: "status_needs_reauth",
  };

  return (
    <span className={`status status-${status}`}>{labels[status] ? t(labels[status]) : status}</span>
  );
}
