import type { HTMLAttributes, ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

type SectionHeaderProps = HTMLAttributes<HTMLElement> & {
  title: ReactNode;
  description?: ReactNode;
  eyebrow?: ReactNode;
  actions?: ReactNode;
  variant?: "default" | "compact";
};

export function SectionHeader({
  actions,
  className,
  description,
  eyebrow,
  title,
  variant = "default",
  ...props
}: SectionHeaderProps) {
  return (
    <header
      className={cn("sectionHeader", variant === "compact" && "compact", className)}
      {...props}
    >
      <div className="sectionHeaderCopy">
        {eyebrow && <span className="sectionHeaderEyebrow">{eyebrow}</span>}
        <h2>{title}</h2>
        {description && <p>{description}</p>}
      </div>
      {actions && <div className="sectionHeaderActions">{actions}</div>}
    </header>
  );
}
