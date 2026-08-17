import { Sparkles } from "lucide-react";
import type { HTMLAttributes, ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

const DEFAULT_ICON = <Sparkles size={30} aria-hidden="true" />;

export function EmptyState({
  icon = DEFAULT_ICON,
  title,
  description,
  action,
  className,
  ...props
}: HTMLAttributes<HTMLDivElement> & {
  icon?: ReactNode;
  title: string;
  description: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className={cn("empty", className)} {...props}>
      <div className="emptyIcon">{icon}</div>
      <h2>{title}</h2>
      <p>{description}</p>
      {action}
    </div>
  );
}

export { EmptyState as Empty };
