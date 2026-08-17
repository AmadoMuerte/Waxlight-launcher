import type { HTMLAttributes } from "react";

import { cn } from "@/shared/lib/utils";

export function Toolbar({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("toolbarLayout", className)} {...props} />;
}

export function ToolbarGroup({
  align = "start",
  className,
  ...props
}: HTMLAttributes<HTMLDivElement> & { align?: "start" | "end" }) {
  return <div className={cn("toolbarGroup", align === "end" && "end", className)} {...props} />;
}
