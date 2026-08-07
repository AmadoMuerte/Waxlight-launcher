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
