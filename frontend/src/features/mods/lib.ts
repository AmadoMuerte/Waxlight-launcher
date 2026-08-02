import type {
  GameVersion,
  Instance,
  ModSide,
  ModVersion,
} from "../../shared/api";

export type Compatibility =
  | "compatible"
  | "possibly_compatible"
  | "incompatible"
  | "unknown";

export function sideLabel(side: ModSide): string {
  return {
    client: "Client",
    server: "Server",
    both: "Client & Server",
    unknown: "Unknown side",
  }[side];
}

export function formatDownloads(value: number): string {
  return new Intl.NumberFormat("en", {
    notation: value >= 1000 ? "compact" : "standard",
    maximumFractionDigits: 1,
  }).format(value);
}

export function formatBytes(value: number): string {
  if (!value) return "Size unavailable";
  const units = ["B", "KB", "MB", "GB"];
  let amount = value;
  let unit = 0;
  while (amount >= 1024 && unit < units.length - 1) {
    amount /= 1024;
    unit += 1;
  }
  return `${amount.toFixed(amount >= 10 || unit === 0 ? 0 : 1)} ${units[unit]}`;
}

export function relativeDate(value?: string): string {
  if (!value) return "Update date unknown";
  const date = new Date(value);
  const days = Math.round((date.getTime() - Date.now()) / 86_400_000);
  const formatter = new Intl.RelativeTimeFormat("en", { numeric: "auto" });
  if (Math.abs(days) < 30) return formatter.format(days, "day");
  const months = Math.round(days / 30);
  if (Math.abs(months) < 12) return formatter.format(months, "month");
  return formatter.format(Math.round(months / 12), "year");
}

export function plainText(html: string): string {
  if (typeof DOMParser !== "undefined") {
    const document = new DOMParser().parseFromString(html, "text/html");
    document.querySelectorAll("script, iframe, style, object, embed").forEach((node) =>
      node.remove(),
    );
    return document.body.textContent?.replace(/\n{3,}/g, "\n\n").trim() ?? "";
  }
  return html.replace(/<[^>]*>/g, " ").replace(/\s+/g, " ").trim();
}

export function instanceGameVersion(
  instance: Instance,
  versions: GameVersion[],
): string {
  const version = versions.find((item) => item.id === instance.gameVersionId);
  return version?.name || version?.id || instance.gameVersionId;
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
    compatible: "Compatible",
    possibly_compatible: "Possibly compatible",
    incompatible: "Incompatible",
    unknown: "Unknown compatibility",
  }[value];
}

export function chooseRelease(
  releases: ModVersion[],
  gameVersion?: string,
): ModVersion | undefined {
  if (gameVersion) {
    const exact = releases.find(
      (release) =>
        release.releaseType === "stable" &&
        release.gameVersions.includes(gameVersion),
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
  return (
    releases.find((release) => release.releaseType === "stable") ?? releases[0]
  );
}
