import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

import { Card } from "../../shared/ui/card";

interface StatCardProps {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  icon?: LucideIcon;
  variant?: "primary" | "secondary";
}

export function StatCard({ label, value, hint, icon: Icon, variant = "primary" }: StatCardProps) {
  return (
    <Card className="p-5">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate text-xs font-semibold tracking-widest text-text-muted uppercase">
            {label}
          </p>
          <p
            className={cn(
              "statCardValue mt-2 overflow-hidden text-ellipsis whitespace-nowrap",
              variant === "primary" ? "text-2xl" : "text-lg",
            )}
          >
            {value}
          </p>
          {hint && <p className="mt-1 text-xs text-text-muted">{hint}</p>}
        </div>
        {Icon && (
          <div className="statCardIcon">
            <Icon size={18} aria-hidden="true" />
          </div>
        )}
      </div>
    </Card>
  );
}
