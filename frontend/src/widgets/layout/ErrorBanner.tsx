import { useQueryClient } from "@tanstack/react-query";
import { CircleAlert } from "lucide-react";
import { useTranslation } from "react-i18next";

import { useAppShellStore } from "../../app/stores/app-shell";
import { WATCHED_QUERY_KEYS } from "../../shared/api/keys";
import { Button } from "../../shared/ui/button";

export function ErrorBanner() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const message = useAppShellStore((state) => state.fatalError);
  const setFatalError = useAppShellStore((state) => state.setFatalError);

  function handleRetry() {
    setFatalError("");
    for (const key of WATCHED_QUERY_KEYS) {
      void queryClient.refetchQueries({ queryKey: key });
    }
  }

  if (!message) {
    return null;
  }

  return (
    <div className="backendError">
      <span>
        <CircleAlert size={13} aria-hidden="true" />
      </span>
      <div>
        <strong>{t("could_not_connect_to_core")}</strong>
        <p>{message}</p>
      </div>
      <Button type="button" variant="ghost" onClick={handleRetry}>
        {t("retry")}
      </Button>
    </div>
  );
}
