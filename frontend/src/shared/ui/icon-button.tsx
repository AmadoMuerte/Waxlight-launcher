import type { ButtonHTMLAttributes, ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

type IconButtonProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, "aria-label"> & {
  "aria-label": string;
  children: ReactNode;
  size?: "sm" | "md" | "lg";
  variant?: "default" | "ghost" | "danger";
};

export function IconButton({
  children,
  className,
  size = "md",
  type = "button",
  variant = "default",
  ...props
}: IconButtonProps) {
  return (
    <button
      type={type}
      className={cn("iconButton", size, variant !== "default" && variant, className)}
      {...props}
    >
      {children}
    </button>
  );
}
