import type { ComponentProps } from "react";

import { cn } from "@/shared/lib/utils";

export function Input({ className, ...props }: ComponentProps<"input">) {
  return <input className={cn("input", className)} {...props} />;
}
