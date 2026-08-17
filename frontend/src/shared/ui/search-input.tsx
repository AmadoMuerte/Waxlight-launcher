import { Search, X } from "lucide-react";
import type { ComponentProps } from "react";
import { useTranslation } from "react-i18next";

import { cn } from "@/shared/lib/utils";

import { Input } from "./input";

interface SearchInputProps extends Omit<ComponentProps<"input">, "value" | "onChange"> {
  value: string;
  onValueChange: (value: string) => void;
  clearLabel?: string;
  wrapperClassName?: string;
}

export function SearchInput({
  value,
  onValueChange,
  clearLabel,
  className,
  wrapperClassName,
  ...props
}: SearchInputProps) {
  const { t } = useTranslation();
  return (
    <div className={cn("searchInput", wrapperClassName)}>
      <Search size={16} className="searchInputIcon" aria-hidden="true" />
      <Input
        className={className}
        value={value}
        onChange={(event) => onValueChange(event.target.value)}
        {...props}
      />
      {value && (
        <button
          type="button"
          className="searchInputClear"
          aria-label={clearLabel ?? t("clear_search")}
          onClick={() => onValueChange("")}
        >
          <X size={15} aria-hidden="true" />
        </button>
      )}
    </div>
  );
}
