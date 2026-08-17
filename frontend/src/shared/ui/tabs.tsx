import { useRef, type KeyboardEvent, type ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

export type TabOption<T extends string> = {
  value: T;
  label: ReactNode;
  disabled?: boolean;
  panelId?: string;
  tabId?: string;
};

type TabsProps<T extends string> = {
  label: string;
  value: T;
  options: readonly TabOption<T>[];
  onValueChange: (value: T) => void;
  className?: string;
};

export function Tabs<T extends string>({
  className,
  label,
  onValueChange,
  options,
  value,
}: TabsProps<T>) {
  const refs = useRef<Array<HTMLButtonElement | null>>([]);
  const selectedEnabled = options.some((option) => option.value === value && !option.disabled);
  const firstEnabledIndex = options.findIndex((option) => !option.disabled);

  const selectFromKeyboard = (event: KeyboardEvent, index: number) => {
    const enabled = options
      .map((option, optionIndex) => ({ option, optionIndex }))
      .filter(({ option }) => !option.disabled);
    const current = enabled.findIndex(({ optionIndex }) => optionIndex === index);
    if (enabled.length === 0 || current < 0) return;
    let target = current;

    if (event.key === "ArrowRight") target = (current + 1) % enabled.length;
    else if (event.key === "ArrowLeft") target = (current - 1 + enabled.length) % enabled.length;
    else if (event.key === "Home") target = 0;
    else if (event.key === "End") target = enabled.length - 1;
    else return;

    event.preventDefault();
    const next = enabled[target];
    onValueChange(next.option.value);
    refs.current[next.optionIndex]?.focus();
  };

  return (
    <div className={cn("tabsList", className)} role="tablist" aria-label={label}>
      {options.map((option, index) => {
        const selected = option.value === value;
        return (
          <button
            key={option.value}
            id={option.tabId}
            ref={(node) => {
              refs.current[index] = node;
            }}
            type="button"
            role="tab"
            aria-selected={selected}
            aria-controls={option.panelId}
            tabIndex={
              !option.disabled && (selected || (!selectedEnabled && index === firstEnabledIndex))
                ? 0
                : -1
            }
            disabled={option.disabled}
            onClick={() => onValueChange(option.value)}
            onKeyDown={(event) => selectFromKeyboard(event, index)}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
