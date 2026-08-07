import { useTranslation } from "react-i18next";

import type { ImportReport } from "../../shared/api/types";
import { Button } from "../../shared/ui/button";
import { Modal } from "../../shared/ui/modal";

interface ImportResultModalProps {
  report: ImportReport;
  onClose: () => void;
}

export function ImportResultModal({ report, onClose }: ImportResultModalProps) {
  const { t } = useTranslation();
  const skipped = report.mods.filter((mod) => mod.status !== "installed");

  return (
    <Modal title={t("import_complete")} className="importResultDialog" onClose={onClose}>
      <div className="modalBody">
        <p className="muted">{t("import_result_description", { name: report.instanceName })}</p>

        <div className="installedModList">
          {report.mods.map((mod) => (
            <article className="installedModRow" key={`${mod.name}-${mod.version}`}>
              <div className="modRowIcon" aria-hidden="true">
                {mod.status === "installed" ? "✓" : "◇"}
              </div>
              <div className="modRowCopy">
                <strong>{mod.name}</strong>
                <small>{t("version_value", { version: mod.version || "?" })}</small>
                {mod.message && <code>{mod.message}</code>}
              </div>
              <span
                className={`status status-${mod.status === "installed" ? "completed" : "failed"}`}
              >
                {mod.status === "installed" ? t("installed") : t("skipped")}
              </span>
            </article>
          ))}
        </div>

        {skipped.length > 0 && report.warnings.length > 0 && (
          <div className="inlineWarning">
            {report.warnings.map((warning) => (
              <p key={warning}>⚠ {warning}</p>
            ))}
          </div>
        )}
      </div>

      <div className="dialogFooter">
        <Button variant="ghost" onClick={onClose}>
          {t("close")}
        </Button>
      </div>
    </Modal>
  );
}
