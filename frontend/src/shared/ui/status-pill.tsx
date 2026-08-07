import { useTranslation } from "react-i18next";

export function StatusPill({ status }: { status: string }) {
  const { t } = useTranslation();
  const labels: Record<string, string> = {
    ready: "status_ready",
    running: "status_running",
    installed: "status_installed",
    queued: "status_queued",
    completed: "status_completed",
    cancelled: "status_cancelled",
    failed: "status_failed",
    stable: "stable",
    unstable: "preview",
    unknown: "status_unknown",
    local_profile: "status_local_profile",
    valid: "status_valid",
    expired: "status_expired",
    needs_reauth: "status_needs_reauth",
  };

  return (
    <span className={`status status-${status}`}>{labels[status] ? t(labels[status]) : status}</span>
  );
}
