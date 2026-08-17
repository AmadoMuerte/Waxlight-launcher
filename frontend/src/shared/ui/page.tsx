import type { HTMLAttributes } from "react";

import { cn } from "@/shared/lib/utils";

export function Page({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("page", className)} {...props} />;
}

export function PageContent({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("pageContent", className)} {...props} />;
}

export function PageSection({ className, ...props }: HTMLAttributes<HTMLElement>) {
  return <section className={cn("pageSection", className)} {...props} />;
}
