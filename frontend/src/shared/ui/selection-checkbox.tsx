import * as CheckboxPrimitive from "@radix-ui/react-checkbox";
import { Check } from "lucide-react";
import type { MouseEvent } from "react";

import { cn } from "@/shared/lib/utils";

function stopPropagation(event: MouseEvent) {
  event.stopPropagation();
}

export function SelectionCheckbox({
  checked,
  onCheckedChange,
  label,
  disabled = false,
  className,
}: {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  label: string;
  disabled?: boolean;
  className?: string;
}) {
  return (
    <CheckboxPrimitive.Root
      className={cn("selectionCheckbox", className)}
      checked={checked}
      onCheckedChange={(next) => onCheckedChange(next === true)}
      aria-label={label}
      disabled={disabled}
      onClick={stopPropagation}
    >
      <CheckboxPrimitive.Indicator>
        <Check size={15} strokeWidth={3.5} aria-hidden="true" />
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  );
}
