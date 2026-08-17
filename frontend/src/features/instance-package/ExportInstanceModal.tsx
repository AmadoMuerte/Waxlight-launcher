import { useState } from "react";
import { useTranslation } from "react-i18next";

import { useToastStore } from "../../app/stores/toast";
import type { Instance } from "../../entities/instance/model";
import { errorMessage } from "../../shared/api/bridge";
import { instancePackageApi } from "../../shared/api/instance-package";
import { Button } from "../../shared/ui/button";
import { DialogFooter } from "../../shared/ui/dialog";
import { Field } from "../../shared/ui/field";
import { Input } from "../../shared/ui/input";
import { Modal } from "../../shared/ui/modal";
import { SubmitForm } from "../../shared/ui/submit-form";

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
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

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
      const manifest = await instancePackageApi.export({
        instanceId: instance.id,
        targetPath,
        name: name.trim(),
        description: description.trim(),
      });
      notify(t("instance_exported", { name: manifest?.name ?? instance.name }));
      await onDone();
    } catch (exportError) {
      setError(errorMessage(exportError));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal title={t("export_instance")} className="createInstanceDialog" onClose={onClose}>
      <SubmitForm className="dialogForm" onSubmit={exportInstance}>
        <div className="modalBody formFields">
          <Field label={t("name")}>
            <Input
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

          <p className="muted">{t("export_sensitive_data_excluded")}</p>

          {error && (
            <div className="inlineError" role="alert">
              {error}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("cancel")}
          </Button>
          <Button type="submit" busy={busy}>
            {t("export")}
          </Button>
        </DialogFooter>
      </SubmitForm>
    </Modal>
  );
}
