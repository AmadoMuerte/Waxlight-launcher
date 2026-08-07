import { useState } from "react";
import { useTranslation } from "react-i18next";

import { useToastStore } from "../../app/stores/toast";
import type { Instance } from "../../entities/instance/model";
import { instancePackageApi } from "../../shared/api";
import { errorMessage } from "../../shared/api/bridge";
import { Button, Field, Modal, SubmitForm } from "../../shared/ui";

interface ExportInstanceModalProps {
  instance: Instance;
  onClose: () => void;
  onDone: () => Promise<void>;
}

export function ExportInstanceModal({ instance, onClose, onDone }: ExportInstanceModalProps) {
  const { t } = useTranslation();
  const notify = useToastStore((state) => state.notify);
  const [name, setName] = useState(instance.name);
  const [description, setDescription] = useState(instance.description);
  const [authorName, setAuthorName] = useState("");
  const [authorHomepage, setAuthorHomepage] = useState("");
  const [authorSource, setAuthorSource] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const homepageInvalid = authorHomepage.trim() !== "" && !isValidHttpUrl(authorHomepage);
  const sourceInvalid = authorSource.trim() !== "" && !isValidHttpUrl(authorSource);
  const linksInvalid = homepageInvalid || sourceInvalid;

  async function exportInstance() {
    setBusy(true);
    setError("");
    try {
      const targetPath = await instancePackageApi.selectExportPath(
        `${name.trim() || instance.name}.waxlight`,
      );
      if (!targetPath) {
        return;
      }
      const author =
        authorName.trim() || authorHomepage.trim() || authorSource.trim()
          ? {
              name: authorName.trim() || undefined,
              homepage: authorHomepage.trim() || undefined,
              source: authorSource.trim() || undefined,
            }
          : undefined;
      const manifest = await instancePackageApi.export({
        instanceId: instance.id,
        targetPath,
        name: name.trim(),
        description: description.trim(),
        author,
      });
      notify(t("instance_exported", { name: manifest?.name ?? instance.name }));
      await onDone();
    } catch (exportError) {
      setError(errorMessage(exportError));
      setBusy(false);
    }
  }

  return (
    <Modal title={t("export_instance")} className="createInstanceDialog" onClose={onClose}>
      <SubmitForm className="dialogForm" onSubmit={exportInstance}>
        <div className="modalBody formFields">
          <Field label={t("name")}>
            <input
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder={t("instance_name_example")}
            />
          </Field>

          <Field label={t("description")}>
            <textarea
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder={t("instance_description_prompt")}
            />
          </Field>

          <p className="muted">{t("export_author_optional_hint")}</p>

          <Field label={t("author_name")}>
            <input value={authorName} onChange={(event) => setAuthorName(event.target.value)} />
          </Field>

          <div className="formRow">
            <Field label={t("author_homepage")}>
              <input
                value={authorHomepage}
                onChange={(event) => setAuthorHomepage(event.target.value)}
                placeholder="https://"
                aria-invalid={homepageInvalid}
              />
            </Field>

            <Field label={t("author_source")}>
              <input
                value={authorSource}
                onChange={(event) => setAuthorSource(event.target.value)}
                placeholder="https://"
                aria-invalid={sourceInvalid}
              />
            </Field>
          </div>

          {linksInvalid && (
            <div className="inlineError" role="alert">
              {t("author_links_invalid")}
            </div>
          )}

          <p className="muted">{t("export_sensitive_data_excluded")}</p>

          {error && (
            <div className="inlineError" role="alert">
              {error}
            </div>
          )}
        </div>

        <div className="dialogFooter">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("cancel")}
          </Button>
          <Button busy={busy} disabled={linksInvalid}>
            {t("export")}
          </Button>
        </div>
      </SubmitForm>
    </Modal>
  );
}

function isValidHttpUrl(value: string): boolean {
  try {
    const parsed = new URL(value.trim());
    return parsed.host !== "" && (parsed.protocol === "http:" || parsed.protocol === "https:");
  } catch {
    return false;
  }
}
