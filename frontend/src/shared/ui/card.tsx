import type { HTMLAttributes } from "react";

import { cn } from "@/shared/lib/utils";

type CardProps = HTMLAttributes<HTMLDivElement> & {
  variant?: "default" | "subtle" | "elevated";
};

const variants = {
  default: "border-border-default bg-surface-2 shadow-surface",
  subtle: "border-border-subtle bg-surface-1",
  elevated: "border-border-default bg-surface-2 shadow-elevated",
} as const;

export function Card({ className, variant = "default", ...props }: CardProps) {
  return (
    <div
      className={cn("rounded-lg border text-text-primary", variants[variant], className)}
      {...props}
    />
  );
}

export function CardHeader({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("space-y-1.5 px-5 pt-5", className)} {...props} />;
}

export function CardTitle({ children, className, ...props }: HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h3 className={cn("font-display text-xl font-semibold", className)} {...props}>
      {children}
    </h3>
  );
}

export function CardDescription({ className, ...props }: HTMLAttributes<HTMLParagraphElement>) {
  return <p className={cn("text-sm text-text-muted", className)} {...props} />;
}

export function CardContent({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("px-5 py-4 text-sm", className)} {...props} />;
}

export function CardFooter({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "flex items-center justify-end gap-3 border-t border-border-subtle px-5 py-4",
        className,
      )}
      {...props}
    />
  );
}
