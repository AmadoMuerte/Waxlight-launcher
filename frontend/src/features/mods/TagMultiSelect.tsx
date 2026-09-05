import { ChevronDown, Search } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";

import type { ModTag } from "../../shared/api";

interface TagMultiSelectProps {
  tags: ModTag[];
  selected: string[];
  onChange: (tags: string[]) => void;
}

export function TagMultiSelect({ tags, selected, onChange }: TagMultiSelectProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");

  const filtered = query
    ? tags.filter((tag) => tag.name.toLowerCase().includes(query.toLowerCase()))
    : tags;

  function toggle(name: string) {
    onChange(
      selected.includes(name) ? selected.filter((item) => item !== name) : [...selected, name],
    );
  }

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className="flex h-10 w-full items-center justify-between gap-2 rounded-md border border-border-default bg-surface-input px-3 py-2 text-left text-sm text-text-primary outline-none transition-[border-color,background-color,box-shadow] hover:border-border-strong hover:bg-surface-input-hover focus-visible:border-accent focus-visible:ring-[3px] focus-visible:ring-accent/15 data-[placeholder]:text-text-disabled"
          aria-haspopup="menu"
          aria-expanded={open}
        >
          {selected.length > 0 ? t("tags_count", { count: selected.length }) : t("tags")}
          <ChevronDown className="size-4 shrink-0 text-text-secondary" aria-hidden="true" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent className="w-[calc(320px*var(--ui-scale))]" align="start">
        <div className="flex items-center gap-2 rounded-md border border-border-default bg-surface-input px-2.5">
          <Search className="size-3.5 shrink-0 text-text-muted" aria-hidden="true" />
          <input
            aria-label={t("search_tags")}
            placeholder={t("search_tags")}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            className="h-9 w-full min-w-0 bg-transparent text-sm text-text-primary outline-none placeholder:text-text-disabled"
          />
        </div>
        <DropdownMenuSeparator />
        {filtered.length === 0 ? (
          <DropdownMenuLabel className="text-center normal-case tracking-normal">
            {t("no_tags_found")}
          </DropdownMenuLabel>
        ) : (
          <div className="max-h-64 overflow-y-auto">
            {filtered.map((tag) => (
              <DropdownMenuCheckboxItem
                key={tag.name}
                checked={selected.includes(tag.name)}
                className="text-[length:var(--fs-body)]"
                onSelect={(event) => {
                  event.preventDefault();
                  toggle(tag.name);
                }}
              >
                {tag.name}
                <span className="ml-auto pl-4 text-xs text-text-muted">{tag.count}</span>
              </DropdownMenuCheckboxItem>
            ))}
          </div>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
