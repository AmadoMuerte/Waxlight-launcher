import { useState } from "react";
import { useTranslation } from "react-i18next";

import { instancesApi, type Instance } from "../../shared/api";
import { errorMessage } from "../../shared/api/bridge";
import { Button, Field, Modal, SubmitForm } from "../../shared/ui";

interface CloneInstanceModalProps {
  instance: Instance;
  onClose: () => void;
  onDone: () => Promise<void>;
  notify: (message: string, type?: "ok" | "error") => void;
}

export function CloneInstanceModal({ instance, onClose, onDone, notify }: CloneInstanceModalProps) {
  const { t } = useTranslation();
  const [name, setName] = useState(instance.name);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function cloneInstance() {
    setBusy(true);
    setError("");
    try {
      const cloned = await instancesApi.clone({ sourceId: instance.id, name: name.trim() });
      notify(t("instance_cloned", { name: cloned?.name ?? instance.name }));
      await onDone();
    } catch (cloneError) {
      setError(errorMessage(cloneError));
      setBusy(false);
    }
  }

  return (
    <Modal title={t("clone_instance")} className="createInstanceDialog" onClose={onClose}>
      <SubmitForm className="dialogForm" onSubmit={cloneInstance}>
        <div className="modalBody formFields">
          <Field label={t("name")}>
            <input
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder={t("default_instance_name")}
            />
          </Field>

          <p className="muted">{t("clone_instance_hint")}</p>

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
          <Button busy={busy}>{t("clone_instance")}</Button>
        </div>
      </SubmitForm>
    </Modal>
  );
}
