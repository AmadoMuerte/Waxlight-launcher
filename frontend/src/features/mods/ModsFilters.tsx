import type { GameVersion, ModSearchQuery } from "../../shared/api";
import { useTranslation } from "react-i18next";
import { Button, Field, Select } from "../../shared/ui";
import { sideLabel } from "./lib";

interface ModsFiltersProps {
  query: ModSearchQuery;
  versions: GameVersion[];
  mobileOpen: boolean;
  onMobileOpenChange: (open: boolean) => void;
  onChange: (patch: Partial<ModSearchQuery>) => void;
  onClear: () => void;
}

export function ModsFilters({
  query,
  versions,
  mobileOpen,
  onMobileOpenChange,
  onChange,
  onClear,
}: ModsFiltersProps) {
  const { t } = useTranslation();
  const active = [
    query.gameVersion && {
      key: "gameVersion",
      label: t("game_version_filter", { version: query.gameVersion }),
    },
    query.side && { key: "side", label: t("side_filter", { side: sideLabel(query.side) }) },
    query.updatedAfter && { key: "updatedAfter", label: t("recently_updated") },
  ].filter(Boolean) as { key: keyof ModSearchQuery; label: string }[];

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
            value={query.gameVersion}
            onChange={(event) => onChange({ gameVersion: event.target.value })}
          >
            <option value="">{t("all_versions")}</option>
            {versions.map((version) => (
              <option key={version.id} value={version.name || version.id}>
                {version.name || version.id}
              </option>
            ))}
          </Select>
        </Field>
        <Field label={t("side")}>
          <Select
            value={query.side}
            onChange={(event) =>
              onChange({ side: event.target.value as ModSearchQuery["side"] })
            }
          >
            <option value="">{t("all_sides")}</option>
            <option value="client">{t("client")}</option>
            <option value="server">{t("server")}</option>
            <option value="both">{t("client_and_server")}</option>
          </Select>
        </Field>
        <Field label={t("updated")}>
          <Select
            value={query.updatedAfter ? updatedPeriod(query.updatedAfter) : ""}
            onChange={(event) =>
              onChange({ updatedAfter: dateFromPeriod(event.target.value) })
            }
          >
            <option value="">{t("any_time")}</option><option value="7">{t("last_7_days")}</option>
            <option value="30">{t("last_30_days")}</option><option value="90">{t("last_3_months")}</option>
            <option value="365">{t("last_year")}</option>
          </Select>
        </Field>
        <Field label={t("sort_by")}>
          <Select
            value={query.sort}
            onChange={(event) =>
              onChange({ sort: event.target.value as ModSearchQuery["sort"] })
            }
          >
            <option value="relevance">{t("relevance")}</option><option value="updated">{t("recently_updated")}</option>
            <option value="newest">{t("newest")}</option><option value="downloads">{t("most_downloaded")}</option>
            <option value="name_asc">{t("name_ascending")}</option><option value="name_desc">{t("name_descending")}</option>
          </Select>
        </Field>
      </section>

      {active.length > 0 && (
        <div className="filterChips" aria-label={t("active_filters")}>
          {active.map((filter) => (
            <button
              key={filter.key}
              onClick={() => onChange({ [filter.key]: "" })}
            >
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
  return [7, 30, 90, 365].reduce((best, period) =>
    Math.abs(period - days) < Math.abs(Number(best) - days) ? String(period) : best,
  "30");
}
