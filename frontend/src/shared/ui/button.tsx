import type { ButtonHTMLAttributes, ReactNode } from "react";

import { Spinner } from "./spinner";

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
  type = "button",
  ...props
}: ButtonProps) {
  return (
    <button
      {...props}
      type={type}
      className={`button ${variant} ${busy ? "busy" : ""} ${className}`.trim()}
      aria-busy={busy || undefined}
      disabled={busy || disabled}
    >
      <span className="buttonLabel">{children}</span>
      {busy && <Spinner className="buttonSpinner" />}
    </button>
  );
}
