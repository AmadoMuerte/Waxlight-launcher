import type { MouseEvent } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "../../shared/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "../../shared/ui/tooltip";

export function ServerPlayButton({
  blockedByWhitelist,
  disabled,
  busy,
  onClick,
}: {
  blockedByWhitelist: boolean;
  disabled: boolean;
  busy?: boolean;
  onClick: (event: MouseEvent<HTMLButtonElement>) => void;
}) {
  const { t } = useTranslation();
  const button = (
    <Button busy={busy} disabled={disabled || blockedByWhitelist} onClick={onClick}>
      {busy ? t("connecting") : t("play")}
    </Button>
  );
  if (!blockedByWhitelist) return button;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex">{button}</span>
      </TooltipTrigger>
      <TooltipContent>{t("server_whitelist_launch_unavailable")}</TooltipContent>
    </Tooltip>
  );
}
