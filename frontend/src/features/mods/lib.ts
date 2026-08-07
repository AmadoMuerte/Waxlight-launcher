import type { GameVersion, Instance, ModSide, ModVersion } from "../../shared/api";
import i18n from "../../shared/i18n";

export type Compatibility = "compatible" | "possibly_compatible" | "incompatible" | "unknown";

export function sideLabel(side: ModSide): string {
  return {
    client: i18n.t("client"),
    server: i18n.t("server"),
    both: i18n.t("client_and_server"),
    unknown: i18n.t("unknown_side"),
  }[side];
}

export function formatDownloads(value: number): string {
  return new Intl.NumberFormat(i18n.resolvedLanguage, {
    notation: value >= 1000 ? "compact" : "standard",
    maximumFractionDigits: 1,
  }).format(value);
}

export function formatBytes(value: number): string {
  if (!value) return i18n.t("size_unavailable");
  const units = ["B", "KB", "MB", "GB"];
  let amount = value;
  let unit = 0;
  while (amount >= 1024 && unit < units.length - 1) {
    amount /= 1024;
    unit += 1;
  }
  return `${new Intl.NumberFormat(i18n.resolvedLanguage, { maximumFractionDigits: amount >= 10 || unit === 0 ? 0 : 1 }).format(amount)} ${units[unit]}`;
}

export function relativeDate(value?: string): string {
  if (!value) return i18n.t("update_date_unknown");
  const date = new Date(value);
  const days = Math.round((date.getTime() - Date.now()) / 86_400_000);
  const formatter = new Intl.RelativeTimeFormat(i18n.resolvedLanguage, { numeric: "auto" });
  if (Math.abs(days) < 30) return formatter.format(days, "day");
  const months = Math.round(days / 30);
  if (Math.abs(months) < 12) return formatter.format(months, "month");
  return formatter.format(Math.round(months / 12), "year");
}

export function plainText(html: string): string {
  if (typeof DOMParser !== "undefined") {
    const document = new DOMParser().parseFromString(html, "text/html");
    document
      .querySelectorAll("script, iframe, style, object, embed")
      .forEach((node) => node.remove());
    return document.body.textContent?.replace(/\n{3,}/g, "\n\n").trim() ?? "";
  }
  return html
    .replace(/<[^>]*>/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

export function instanceGameVersion(instance: Instance, versions: GameVersion[]): string {
  const version = versions.find((item) => item.id === instance.gameVersionId);
  return version?.name || version?.id || instance.gameVersionId;
}

const SERIES_PATTERN = /^(\d+)\.(\d+)/;

export function gameVersionSeriesOf(versionID: string): string {
  const match = SERIES_PATTERN.exec(versionID);
  return match ? `${match[1]}.${match[2]}.x` : "";
}

export function gameVersionSeries(versions: { id: string }[]): string[] {
  const seen = new Set<string>();
  const series: string[] = [];
  for (const version of versions) {
    const value = gameVersionSeriesOf(version.id);
    if (value && !seen.has(value)) {
      seen.add(value);
      series.push(value);
    }
  }
  return series.toSorted((left, right) => {
    const leftParts = left.split(".");
    const rightParts = right.split(".");
    for (let index = 0; index < leftParts.length && index < rightParts.length; index++) {
      const difference = Number(rightParts[index]) - Number(leftParts[index]);
      if (difference !== 0) return difference;
    }
    return 0;
  });
}

export function matchesGameVersionSeries(supported: string, series: string): boolean {
  const prefix = series.replace(/\.x$/, "");
  return supported === prefix || supported.startsWith(`${prefix}.`);
}

export function compatibilityFor(
  instance: Instance,
  gameVersions: GameVersion[],
  release: ModVersion,
): Compatibility {
  if (release.gameVersions.length === 0) return "unknown";
  const current = instanceGameVersion(instance, gameVersions);
  if (release.gameVersions.includes(current)) return "compatible";
  const currentParts = current.split(".");
  const sameSeries = release.gameVersions.some((supported) => {
    const parts = supported.split(".");
    return parts[0] === currentParts[0] && parts[1] === currentParts[1];
  });
  return sameSeries ? "possibly_compatible" : "incompatible";
}

export function compatibilityLabel(value: Compatibility): string {
  return {
    compatible: i18n.t("compatible"),
    possibly_compatible: i18n.t("possibly_compatible"),
    incompatible: i18n.t("incompatible"),
    unknown: i18n.t("unknown_compatibility"),
  }[value];
}

export function releaseTypeLabel(value: ModVersion["releaseType"]): string {
  return {
    stable: i18n.t("stable"),
    beta: i18n.t("beta"),
    alpha: i18n.t("alpha"),
    unknown: i18n.t("status_unknown"),
  }[value];
}

export function formatGameVersions(versions: string[], maxVisible = 4): string {
  if (versions.length === 0) return "";
  const visible = versions.slice(0, maxVisible);
  const hidden = versions.length - visible.length;
  if (hidden <= 0) return visible.join(", ");
  return `${visible.join(", ")}… +${hidden}`;
}

export function chooseRelease(
  releases: ModVersion[],
  gameVersion?: string,
): ModVersion | undefined {
  if (gameVersion) {
    const exact = releases.find(
      (release) => release.releaseType === "stable" && release.gameVersions.includes(gameVersion),
    );
    if (exact) return exact;
    const parts = gameVersion.split(".");
    const series = releases.find((release) =>
      release.gameVersions.some((supported) => {
        const supportedParts = supported.split(".");
        return supportedParts[0] === parts[0] && supportedParts[1] === parts[1];
      }),
    );
    if (series) return series;
  }
  return releases.find((release) => release.releaseType === "stable") ?? releases[0];
}
