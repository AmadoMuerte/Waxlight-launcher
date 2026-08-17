import { Flame } from "lucide-react";
import type { HTMLAttributes } from "react";

import { cn } from "@/shared/lib/utils";

export function LoadingState({
  children = "Loading…",
  className,
  ...props
}: HTMLAttributes<HTMLOutputElement>) {
  return (
    <output
      className={cn("loadingState", className)}
      aria-live="polite"
      aria-busy="true"
      {...props}
    >
      <span className="loadingFlame" aria-hidden="true">
        <Flame size={18} fill="currentColor" />
      </span>
      <span>{children}</span>
    </output>
  );
}
