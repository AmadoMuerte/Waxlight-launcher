import type { ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

export function Field({
  label,
  hint,
  error,
  className,
  children,
}: {
  label: string;
  hint?: string;
  error?: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <label className={cn("field", className)}>
      <span>{label}</span>
      {children}
      {error && <small className="fieldError">{error}</small>}
      {!error && hint && <small>{hint}</small>}
    </label>
  );
}
