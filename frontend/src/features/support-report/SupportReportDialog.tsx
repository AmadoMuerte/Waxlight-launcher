import { Check, Clipboard, Eye } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { useSupportReportStore } from "../../app/stores/support-report";
import { useToastStore } from "../../app/stores/toast";
import { errorMessage } from "../../shared/api/bridge";
import { supportReportsApi, type SupportReportResult } from "../../shared/api/support-reports";
import { Button } from "../../shared/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../../shared/ui/dialog";
import { Field } from "../../shared/ui/field";
import { ClipboardSetText } from "../../wailsjs/runtime/runtime";

const descriptionLimit = 2000;

export function SupportReportDialog() {
  const { t } = useTranslation();
  const notify = useToastStore((state) => state.notify);
  const open = useSupportReportStore((state) => state.open);
  const instanceId = useSupportReportStore((state) => state.instanceId);
  const close = useSupportReportStore((state) => state.close);
  const [description, setDescription] = useState("");
  const [error, setError] = useState("");
  const [preview, setPreview] = useState("");
  const [snapshotId, setSnapshotId] = useState("");
  const [sending, setSending] = useState(false);
  const [result, setResult] = useState<SupportReportResult>();

  const invalid = description.trim() === "";

  function dismiss() {
    if (sending) return;
    close();
    setPreview("");
    setSnapshotId("");
    setError("");
    setResult(undefined);
  }

  async function viewIncludedData() {
    if (invalid) {
      setError(t("support_report_description_required"));
      return;
    }
    setError("");
    try {
      const included = await supportReportsApi.preview(description, instanceId);
      setPreview(included.payload);
      setSnapshotId(included.snapshotId);
    } catch (previewError) {
      setError(errorMessage(previewError));
    }
  }

  async function submit() {
    if (invalid) {
      setError(t("support_report_description_required"));
      return;
    }
    setSending(true);
    setError("");
    try {
      let currentSnapshotId = snapshotId;
      if (!currentSnapshotId) {
        const included = await supportReportsApi.preview(description, instanceId);
        currentSnapshotId = included.snapshotId;
        setSnapshotId(currentSnapshotId);
      }
      setResult(await supportReportsApi.submit(currentSnapshotId));
    } catch (submitError) {
      setError(errorMessage(submitError));
    } finally {
      setSending(false);
    }
  }

  async function copyID() {
    if (!result) return;
    if (await ClipboardSetText(result.reportId)) notify(t("support_report_id_copied"));
    else notify(t("support_report_id_copy_failed"), "error");
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && dismiss()}>
      <DialogContent closable={!sending}>
        <DialogHeader>
          <div>
            <DialogTitle>{t("report_a_problem")}</DialogTitle>
            <DialogDescription>{t("support_report_intro")}</DialogDescription>
          </div>
        </DialogHeader>
        <div className="min-h-0 overflow-y-auto p-6">
          {result ? (
            <div className="flex flex-col items-center gap-5 py-5 text-center">
              <Check className="size-10 text-success" aria-hidden="true" />
              <div>
                <h3 className="font-display text-xl font-semibold">{t("support_report_sent")}</h3>
                <p className="mt-2 text-sm text-text-muted">{t("support_report_id")}</p>
                <code className="mt-1 block text-lg">{result.reportId}</code>
              </div>
              <Button variant="secondary" onClick={() => void copyID()}>
                <Clipboard className="size-4" aria-hidden="true" /> {t("copy_id")}
              </Button>
            </div>
          ) : preview ? (
            <div className="space-y-4">
              <p className="text-sm text-text-muted">{t("support_report_preview_description")}</p>
              <pre className="max-h-[45vh] overflow-auto rounded-lg bg-surface-1 p-4 text-xs whitespace-pre-wrap">
                {preview}
              </pre>
              <Button variant="secondary" onClick={() => setPreview("")}>
                {t("back")}
              </Button>
            </div>
          ) : (
            <div className="space-y-5">
              <Field
                label={t("what_happened")}
                error={error}
                hint={`${description.length}/${descriptionLimit}`}
              >
                <textarea
                  aria-label={t("what_happened")}
                  className="input min-h-28 resize-y"
                  value={description}
                  maxLength={descriptionLimit}
                  disabled={sending}
                  onChange={(event) => {
                    setDescription(event.target.value);
                    setSnapshotId("");
                  }}
                />
              </Field>
              <div>
                <h3 className="mb-2 text-sm font-semibold">{t("diagnostics")}</h3>
                <ul className="grid gap-1 text-sm text-text-secondary sm:grid-cols-2">
                  {["launcher", "system", "instance", "mods", "operations", "launch", "logs"].map(
                    (item) => (
                      <li key={item} className="flex items-center gap-2">
                        <Check className="size-3.5 text-success" />
                        {t(`support_report_${item}`)}
                      </li>
                    ),
                  )}
                </ul>
              </div>
              <p className="text-sm text-text-muted">{t("support_report_privacy")}</p>
              <Button variant="secondary" onClick={() => void viewIncludedData()}>
                <Eye className="size-4" aria-hidden="true" /> {t("view_included_data")}
              </Button>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="secondary" disabled={sending} onClick={dismiss}>
            {result ? t("close") : t("cancel")}
          </Button>
          {!result && !preview && (
            <Button busy={sending} onClick={() => void submit()}>
              {t("send_report")}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
