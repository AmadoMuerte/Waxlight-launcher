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
} from "@/components/ui/dropdown-menu";

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
          className="tagFilterTrigger"
          aria-haspopup="menu"
          aria-expanded={open}
        >
          {selected.length > 0 ? t("tags_count", { count: selected.length }) : t("tags")}
          <ChevronDown className="size-4 shrink-0 text-[#aaa39c]" aria-hidden="true" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent className="w-[280px]" align="start">
        <div className="tagFilterSearch">
          <Search className="size-3.5" aria-hidden="true" />
          <input
            aria-label={t("search_tags")}
            placeholder={t("search_tags")}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>
        <DropdownMenuSeparator />
        {filtered.length === 0 ? (
          <DropdownMenuLabel className="tagFilterEmpty">{t("no_tags_found")}</DropdownMenuLabel>
        ) : (
          <div className="tagFilterList">
            {filtered.map((tag) => (
              <DropdownMenuCheckboxItem
                key={tag.name}
                checked={selected.includes(tag.name)}
                onSelect={(event) => {
                  event.preventDefault();
                  toggle(tag.name);
                }}
              >
                {tag.name}
                <span className="tagFilterCount">{tag.count}</span>
              </DropdownMenuCheckboxItem>
            ))}
          </div>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
