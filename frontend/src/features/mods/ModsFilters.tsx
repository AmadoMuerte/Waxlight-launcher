import type { GameVersion, ModSearchQuery } from "../../shared/api";
import { Button, Field } from "../../shared/ui";
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
  const active = [
    query.gameVersion && {
      key: "gameVersion",
      label: `Game version: ${query.gameVersion}`,
    },
    query.side && { key: "side", label: `Side: ${sideLabel(query.side)}` },
    query.updatedAfter && { key: "updatedAfter", label: "Recently updated" },
  ].filter(Boolean) as { key: keyof ModSearchQuery; label: string }[];

  return (
    <>
      <Button
        variant="secondary"
        className="mobileFiltersButton"
        onClick={() => onMobileOpenChange(true)}
      >
        Filters {active.length > 0 ? `(${active.length})` : ""}
      </Button>
      {mobileOpen && (
        <button
          className="filterScrim"
          aria-label="Close filters"
          onClick={() => onMobileOpenChange(false)}
        />
      )}
      <section className={`modsFilters ${mobileOpen ? "mobileOpen" : ""}`}>
        <div className="filterPanelTitle">
          <strong>Filters and sorting</strong>
          <button
            className="iconButton mobileOnly"
            aria-label="Close filters"
            onClick={() => onMobileOpenChange(false)}
          >
            ×
          </button>
        </div>
        <Field label="Game version">
          <select
            value={query.gameVersion}
            onChange={(event) => onChange({ gameVersion: event.target.value })}
          >
            <option value="">All versions</option>
            {versions.map((version) => (
              <option key={version.id} value={version.name || version.id}>
                {version.name || version.id}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Side">
          <select
            value={query.side}
            onChange={(event) =>
              onChange({ side: event.target.value as ModSearchQuery["side"] })
            }
          >
            <option value="">All sides</option>
            <option value="client">Client</option>
            <option value="server">Server</option>
            <option value="both">Client &amp; Server</option>
          </select>
        </Field>
        <Field label="Updated">
          <select
            value={query.updatedAfter ? updatedPeriod(query.updatedAfter) : ""}
            onChange={(event) =>
              onChange({ updatedAfter: dateFromPeriod(event.target.value) })
            }
          >
            <option value="">Any time</option>
            <option value="7">Last 7 days</option>
            <option value="30">Last 30 days</option>
            <option value="90">Last 3 months</option>
            <option value="365">Last year</option>
          </select>
        </Field>
        <Field label="Sort by">
          <select
            value={query.sort}
            onChange={(event) =>
              onChange({ sort: event.target.value as ModSearchQuery["sort"] })
            }
          >
            <option value="relevance">Relevance</option>
            <option value="updated">Recently updated</option>
            <option value="newest">Newest</option>
            <option value="downloads">Most downloaded</option>
            <option value="name_asc">Name: A–Z</option>
            <option value="name_desc">Name: Z–A</option>
          </select>
        </Field>
      </section>

      {active.length > 0 && (
        <div className="filterChips" aria-label="Active filters">
          {active.map((filter) => (
            <button
              key={filter.key}
              onClick={() => onChange({ [filter.key]: "" })}
            >
              {filter.label} <span>×</span>
            </button>
          ))}
          <button className="clearFilters" onClick={onClear}>
            Clear all
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
