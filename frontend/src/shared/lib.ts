import i18n from "../i18n";

export function formatDuration(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  if (hours > 0) {
    return i18n.t("duration_hours_minutes", { hours, minutes });
  }
  return i18n.t("duration_minutes", { minutes });
}

export function formatBytes(bytes: number): string {
  if (!bytes) {
    return "0 B";
  }

  const units = ["B", "KB", "MB", "GB"];
  const unitIndex = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** unitIndex;

  const digits = unitIndex === 0 ? 0 : 1;
  return `${new Intl.NumberFormat(i18n.resolvedLanguage, { minimumFractionDigits: digits, maximumFractionDigits: digits }).format(value)} ${units[unitIndex]}`;
}

export function formatDate(value?: string): string {
  if (!value) {
    return i18n.t("never");
  }

  return new Intl.DateTimeFormat(i18n.resolvedLanguage, {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}
