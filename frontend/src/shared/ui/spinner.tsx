import type { HTMLAttributes } from "react";

import { cn } from "@/shared/lib/utils";

export function Spinner({ className, ...props }: HTMLAttributes<HTMLSpanElement>) {
  return <span className={cn("spinner", className)} aria-hidden="true" {...props} />;
}
