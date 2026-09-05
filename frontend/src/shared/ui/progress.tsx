import * as ProgressPrimitive from "@radix-ui/react-progress";
import * as React from "react";

import { cn } from "@/shared/lib/utils";

type ProgressProps = React.ComponentProps<typeof ProgressPrimitive.Root> & {
  /** Determinate progress as a percentage of `max` (0-100 by default). */
  value?: number;
  indeterminate?: boolean;
  compact?: boolean;
};

function Progress({
  className,
  value,
  max = 100,
  indeterminate = false,
  compact = false,
  ...props
}: ProgressProps) {
  const clamped = Math.min(max, Math.max(0, value ?? 0));
  const percent = max <= 0 ? 0 : Math.round((clamped / max) * 100);

  return (
    <ProgressPrimitive.Root
      data-slot="progress"
      value={indeterminate ? undefined : clamped}
      max={max}
      className={cn(
        "relative w-full overflow-hidden rounded-full bg-surface-3",
        compact ? "h-1" : "h-[calc(7px*var(--ui-scale))]",
        className,
      )}
      {...props}
    >
      <ProgressPrimitive.Indicator
        className={cn(
          "h-full rounded-full transition-[width] duration-200 ease-out",
          indeterminate ? "progressIndicatorIndeterminate" : "progressIndicator",
        )}
        style={indeterminate ? undefined : { width: `${percent}%` }}
      />
    </ProgressPrimitive.Root>
  );
}

export { Progress };
