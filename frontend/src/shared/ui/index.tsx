import type {
  ButtonHTMLAttributes,
  InputHTMLAttributes,
  FormEvent,
  ReactNode,
} from "react";
import { useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";

type ButtonVariant = "primary" | "secondary" | "danger" | "ghost";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  variant?: ButtonVariant;
  busy?: boolean;
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
      className={`button ${variant} ${className}`.trim()}
      disabled={busy || disabled}
    >
      {busy ? <span className="spinner" /> : children}
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
  const { t } = useTranslation();
  const dialogRef = useRef<HTMLElement>(null);
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  useEffect(() => {
    const origin = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : undefined;
    const dialog = dialogRef.current;
    const focusable = dialog?.querySelector<HTMLElement>(
      "button:not([disabled]), input:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex='-1'])",
    );
      focusable?.focus();
    function keydown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        // Radix Select renders its open menu in a portal and owns this Escape key.
        if (document.querySelector('[data-slot="select-content"][data-state="open"]')) {
          return;
        }
        event.preventDefault();
        onCloseRef.current();
        return;
      }
      if (event.key !== "Tab" || !dialog) return;
      const items = [...dialog.querySelectorAll<HTMLElement>(
        "button:not([disabled]), input:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex='-1'])",
      )];
      if (items.length === 0) return;
      const first = items[0];
      const last = items[items.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }
    document.addEventListener("keydown", keydown);
    return () => {
      document.removeEventListener("keydown", keydown);
      origin?.focus();
    };
  }, []);

  return (
    <div
      className="modalBackdrop"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) {
          onClose();
        }
      }}
    >
      <section
        ref={dialogRef}
        className={`modal ${className}`.trim()}
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
        <div className="modalHeader">
          <h2>{title}</h2>
          <button
            type="button"
            className="iconButton"
            aria-label={t("close")}
            onClick={onClose}
          >
            ×
          </button>
        </div>
        {children}
      </section>
    </div>
  );
}

export function Checkbox({
  label,
  className = "",
  ...props
}: Omit<InputHTMLAttributes<HTMLInputElement>, "type"> & {
  label: string;
}) {
  return (
    <label className={`checkboxControl ${className}`.trim()}>
      <input type="checkbox" {...props} />
      <span className="checkboxBox" aria-hidden="true" />
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
    ready: "status_ready", running: "status_running", installed: "status_installed",
    queued: "status_queued", completed: "status_completed", cancelled: "status_cancelled",
    failed: "status_failed", stable: "stable", unstable: "preview", unknown: "status_unknown",
    local_profile: "status_local_profile",
    valid: "status_valid", expired: "status_expired", needs_reauth: "status_needs_reauth",
  };

  return (
    <span className={`status status-${status}`}>
      {labels[status] ? t(labels[status]) : status}
    </span>
  );
}
