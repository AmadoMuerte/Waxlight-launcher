import type { ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

export type SegmentOption<T extends string> = {
  value: T;
  label: ReactNode;
  accessibleLabel?: string;
  disabled?: boolean;
};

type SegmentedControlProps<T extends string> = {
  label: string;
  value: T;
  options: readonly SegmentOption<T>[];
  onValueChange: (value: T) => void;
  className?: string;
};

export function SegmentedControl<T extends string>({
  className,
  label,
  onValueChange,
  options,
  value,
}: SegmentedControlProps<T>) {
  return (
    <fieldset className={cn("segmentedControl", className)}>
      <legend className="sr-only">{label}</legend>
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          aria-label={option.accessibleLabel}
          aria-pressed={option.value === value}
          disabled={option.disabled}
          onClick={() => onValueChange(option.value)}
        >
          {option.label}
        </button>
      ))}
    </fieldset>
  );
}
