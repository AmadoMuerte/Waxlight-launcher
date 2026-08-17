import type { ButtonHTMLAttributes } from "react";

import { cn } from "@/shared/lib/utils";

interface SwitchProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  label: string;
}

export function Switch({
  checked,
  onCheckedChange,
  label,
  disabled = false,
  className = "",
  ...props
}: SwitchProps) {
  return (
    <button
      {...props}
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      data-state={checked ? "checked" : "unchecked"}
      className={cn(
        "relative inline-flex h-[22px] w-[40px] shrink-0 cursor-pointer items-center rounded-full border border-border-strong bg-surface-input transition-colors focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2 data-[state=checked]:border-accent data-[state=checked]:bg-accent disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
    >
      <span
        data-state={checked ? "checked" : "unchecked"}
        className="pointer-events-none block h-[18px] w-[18px] translate-x-[2px] rounded-full bg-text-secondary shadow transition-transform data-[state=checked]:translate-x-[18px] data-[state=checked]:bg-accent-foreground"
      />
    </button>
  );
}
