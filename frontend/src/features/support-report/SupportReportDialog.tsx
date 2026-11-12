import { useTranslation } from "react-i18next";

import { useSupportReportStore } from "../../app/stores/support-report";
import { DISCORD_URL } from "../../shared/lib/links";
import { Button } from "../../shared/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../../shared/ui/dialog";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";

export function SupportReportDialog() {
  const { t } = useTranslation();
  const open = useSupportReportStore((state) => state.open);
  const close = useSupportReportStore((state) => state.close);

  function openDiscord() {
    try {
      BrowserOpenURL(DISCORD_URL);
    } catch {
      window.open(DISCORD_URL, "_blank", "noopener,noreferrer");
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && close()}>
      <DialogContent>
        <DialogHeader>
          <div>
            <DialogTitle>{t("report_a_problem")}</DialogTitle>
            <DialogDescription>{t("support_report_intro")}</DialogDescription>
          </div>
        </DialogHeader>
        <DialogFooter>
          <Button variant="secondary" onClick={close}>
            {t("close")}
          </Button>
          <Button onClick={openDiscord}>{t("official_waxlight_discord")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
