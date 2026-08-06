import { useTranslation } from "react-i18next";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

import type { GameVersion, ModSearchQuery, ModTag } from "../../shared/api";
import { Button, Field } from "../../shared/ui";
import { sideLabel } from "./lib";
import { TagMultiSelect } from "./TagMultiSelect";

interface ModsFiltersProps {
  query: ModSearchQuery;
  versions: GameVersion[];
  tags: ModTag[];
  mobileOpen: boolean;
  onMobileOpenChange: (open: boolean) => void;
  onChange: (patch: Partial<ModSearchQuery>) => void;
  onClear: () => void;
}

export function ModsFilters({
  query,
  versions,
  tags,
  mobileOpen,
  onMobileOpenChange,
  onChange,
  onClear,
}: ModsFiltersProps) {
  const { t } = useTranslation();
  const active: { key: string; label: string; onRemove: () => void }[] = [];
  if (query.gameVersion) {
    active.push({
      key: "gameVersion",
      label: t("game_version_filter", { version: query.gameVersion }),
      onRemove: () => onChange({ gameVersion: "" }),
    });
  }
  if (query.side) {
    active.push({
      key: "side",
      label: t("side_filter", { side: sideLabel(query.side) }),
      onRemove: () => onChange({ side: "" }),
    });
  }
  if (query.updatedAfter) {
    active.push({
      key: "updatedAfter",
      label: t("recently_updated"),
      onRemove: () => onChange({ updatedAfter: "" }),
    });
  }
  for (const tag of query.tags) {
    active.push({
      key: `tag:${tag}`,
      label: t("tag_filter", { tag }),
      onRemove: () => onChange({ tags: query.tags.filter((item) => item !== tag) }),
    });
  }

  return (
    <>
      <Button
        variant="secondary"
        className="mobileFiltersButton"
        onClick={() => onMobileOpenChange(true)}
      >
        {active.length > 0 ? t("filters_count", { count: active.length }) : t("filters")}
      </Button>
      {mobileOpen && (
        <button
          className="filterScrim"
          aria-label={t("close_filters")}
          onClick={() => onMobileOpenChange(false)}
        />
      )}
      <section className={`modsFilters ${mobileOpen ? "mobileOpen" : ""}`}>
        <div className="filterPanelTitle">
          <strong>{t("filters_and_sorting")}</strong>
          <button
            className="iconButton mobileOnly"
            aria-label={t("close_filters")}
            onClick={() => onMobileOpenChange(false)}
          >
            ×
          </button>
        </div>
        <Field label={t("game_version")}>
          <Select
            value={query.gameVersion ? `version:${query.gameVersion}` : "all"}
            onValueChange={(value) =>
              onChange({ gameVersion: value === "all" ? "" : value.slice("version:".length) })
            }
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("all_versions")}</SelectItem>
              {versions.map((version) => (
                <SelectItem key={version.id} value={`version:${version.name || version.id}`}>
                  {version.name || version.id}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
        <Field label={t("side")}>
          <Select
            value={query.side || "all"}
            onValueChange={(value) => onChange({ side: modSide(value) })}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("all_sides")}</SelectItem>
              <SelectItem value="client">{t("client")}</SelectItem>
              <SelectItem value="server">{t("server")}</SelectItem>
              <SelectItem value="both">{t("client_and_server")}</SelectItem>
            </SelectContent>
          </Select>
        </Field>
        <Field label={t("updated")}>
          <Select
            value={query.updatedAfter ? updatedPeriod(query.updatedAfter) : "any"}
            onValueChange={(value) =>
              onChange({ updatedAfter: dateFromPeriod(value === "any" ? "" : value) })
            }
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="any">{t("any_time")}</SelectItem>
              <SelectItem value="7">{t("last_7_days")}</SelectItem>
              <SelectItem value="30">{t("last_30_days")}</SelectItem>
              <SelectItem value="90">{t("last_3_months")}</SelectItem>
              <SelectItem value="365">{t("last_year")}</SelectItem>
            </SelectContent>
          </Select>
        </Field>
        <Field label={t("tags")}>
          <TagMultiSelect
            tags={tags}
            selected={query.tags}
            onChange={(next) => onChange({ tags: next })}
          />
        </Field>
        <Field label={t("sort_by")}>
          <Select value={query.sort} onValueChange={(value) => onChange({ sort: modSort(value) })}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="relevance">{t("relevance")}</SelectItem>
              <SelectItem value="updated">{t("recently_updated")}</SelectItem>
              <SelectItem value="newest">{t("newest")}</SelectItem>
              <SelectItem value="downloads">{t("most_downloaded")}</SelectItem>
              <SelectItem value="name_asc">{t("name_ascending")}</SelectItem>
              <SelectItem value="name_desc">{t("name_descending")}</SelectItem>
            </SelectContent>
          </Select>
        </Field>
      </section>

      {active.length > 0 && (
        <div className="filterChips" aria-label={t("active_filters")}>
          {active.map((filter) => (
            <button key={filter.key} onClick={filter.onRemove}>
              {filter.label} <span>×</span>
            </button>
          ))}
          <button className="clearFilters" onClick={onClear}>
            {t("clear_filters")}
          </button>
        </div>
      )}
    </>
  );
}

function dateFromPeriod(value: string): string | undefined {
  if (!value) return undefined;
  const date = new Date();
  date.setUTCDate(date.getUTCDate() - Number(value));
  return date.toISOString();
}

function updatedPeriod(value: string): string {
  const days = Math.round((Date.now() - new Date(value).getTime()) / 86_400_000);
  return [7, 30, 90, 365].reduce(
    (best, period) =>
      Math.abs(period - days) < Math.abs(Number(best) - days) ? String(period) : best,
    "30",
  );
}

function modSide(value: string): ModSearchQuery["side"] {
  switch (value) {
    case "client":
    case "server":
    case "both":
    case "unknown":
      return value;
    default:
      return "";
  }
}

function modSort(value: string): ModSearchQuery["sort"] {
  switch (value) {
    case "relevance":
    case "updated":
    case "newest":
    case "downloads":
    case "name_asc":
    case "name_desc":
      return value;
    default:
      return "updated";
  }
}
