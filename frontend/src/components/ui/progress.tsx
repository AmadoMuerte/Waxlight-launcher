import * as ProgressPrimitive from "@radix-ui/react-progress";
import * as React from "react";

import { cn } from "@/lib/utils";

function Progress({
  className,
  value,
  ...props
}: React.ComponentProps<typeof ProgressPrimitive.Root>) {
  return (
    <ProgressPrimitive.Root
      data-slot="progress"
      className={cn("relative h-[7px] w-full overflow-hidden rounded-full bg-[#29262a]", className)}
      {...props}
    >
      <ProgressPrimitive.Indicator
        className="h-full rounded-full bg-gradient-to-r from-[#c47d2c] to-[#f1bd61] transition-[width] duration-200"
        style={{ width: `${(value ?? 0) * 100}%` }}
      />
    </ProgressPrimitive.Root>
  );
}

export { Progress };
