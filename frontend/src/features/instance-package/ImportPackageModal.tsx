import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

import { ConfirmDialog } from "../../components/ui/confirm-dialog";
import {
  instancePackageApi,
  type GameVersion,
  type ImportReport,
  type PackageInspection,
} from "../../shared/api";
import { errorMessage } from "../../shared/api/bridge";
import { Button, Checkbox, Field, Modal, SubmitForm } from "../../shared/ui";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";

interface ImportPackageModalProps {
  inspection: PackageInspection;
  versions: GameVersion[];
  onClose: () => void;
  onDone: (report: ImportReport) => Promise<void>;
  onBackgroundDone: () => Promise<void>;
  notify: (message: string, type?: "ok" | "error", duration?: number) => void;
}

export function ImportPackageModal({
  inspection,
  versions,
  onClose,
  onDone,
  onBackgroundDone,
  notify,
}: ImportPackageModalProps) {
  const { t } = useTranslation();
  const [name, setName] = useState(inspection.name);
  const [description, setDescription] = useState(inspection.description ?? "");
  const [versionChoice, setVersionChoice] = useState("");
  const [installVersion, setInstallVersion] = useState(inspection.versionStatus === "available");
  const [allowIncompatible, setAllowIncompatible] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [pendingExternalUrl, setPendingExternalUrl] = useState<string>();

  const problemMods = useMemo(
    () =>
      inspection.mods.filter((mod) => mod.status === "missing" || mod.status === "incompatible"),
    [inspection.mods],
  );

  const requiresVersionChoice = inspection.versionStatus !== "installed";
  const useRequiredVersion = requiresVersionChoice && versionChoice === "" && installVersion;

  function requestOpenExternal(url: string) {
    if (!url.startsWith("https://")) {
      return;
    }
    setPendingExternalUrl(url);
  }

  function confirmExternalUrl() {
    if (pendingExternalUrl) {
      try {
        BrowserOpenURL(pendingExternalUrl);
      } catch {
        window.open(pendingExternalUrl, "_blank", "noopener,noreferrer");
      }
    }
    setPendingExternalUrl(undefined);
  }

  async function importPackage() {
    setBusy(true);
    setError("");
    const request = {
      packagePath: inspection.path,
      name,
      description: description.trim(),
      directory: "",
      gameVersionId: versionChoice,
      installVersion: useRequiredVersion,
      allowIncompatible,
      skipUnavailable: true,
    };
    try {
      if (useRequiredVersion) {
        // The game version download runs in the background and its progress is
        // visible on the Operations page. Close the dialog immediately.
        onClose();
        notify(t("import_waiting_for_game_download"), "ok", 5_000);
        void instancePackageApi
          .import(request)
          .then(async () => {
            await onBackgroundDone();
            return undefined;
          })
          .catch((importError) => {
            notify(errorMessage(importError), "error", 5_000);
          });
        return;
      }
      const report = await instancePackageApi.import(request);
      notify(t("instance_imported", { name: report.instanceName }));
      await onDone(report);
    } catch (importError) {
      setError(errorMessage(importError));
      setBusy(false);
    }
  }

  return (
    <Modal title={t("import_instance")} className="importPreviewDialog" onClose={onClose}>
      <SubmitForm className="dialogForm" onSubmit={importPackage}>
        <div className="modalBody importPreviewBody">
          <div className="importPreviewHeader">
            <div className="heroMark" aria-hidden="true">
              {inspection.hasIcon ? "▦" : "W"}
            </div>
            <div className="instanceHeroCopy">
              <div className="instanceHeroTitle">
                <h2>{inspection.name}</h2>
              </div>
              <p className={inspection.description ? "" : "placeholderCopy"}>
                {inspection.description || t("no_description_yet")}
              </p>
              {inspection.author?.name && (
                <small className="muted">{t("by_author", { name: inspection.author.name })}</small>
              )}
              {(inspection.author?.homepage || inspection.author?.source) && (
                <div className="authorLinks">
                  {inspection.author.homepage && (
                    <button
                      type="button"
                      className="linkButton"
                      onClick={() => requestOpenExternal(inspection.author!.homepage!)}
                    >
                      {t("author_homepage")} ↗
                    </button>
                  )}
                  {inspection.author.source && (
                    <button
                      type="button"
                      className="linkButton"
                      onClick={() => requestOpenExternal(inspection.author!.source!)}
                    >
                      {t("author_source")} ↗
                    </button>
                  )}
                </div>
              )}
            </div>
          </div>

          <div className="instanceStats" aria-label={t("instance_statistics")}>
            <article>
              <span>{t("game_version")}</span>
              <strong>{inspection.gameVersion.name || inspection.gameVersion.id}</strong>
              <small>
                {inspection.versionStatus === "installed"
                  ? t("version_installed")
                  : inspection.versionStatus === "available"
                    ? t("version_available_to_download")
                    : t("version_unavailable")}
              </small>
            </article>
            <article>
              <span>{t("mods")}</span>
              <strong>{inspection.mods.length}</strong>
            </article>
            <article>
              <span>{t("config_files")}</span>
              <strong>{inspection.configFiles.length}</strong>
            </article>
          </div>

          {inspection.mods.length > 0 && (
            <section className="settingsSection">
              <header>
                <h3>{t("included_mods")}</h3>
                <p>{t("included_mods_description")}</p>
              </header>
              <div className="installedModList">
                {inspection.mods.map((mod) => (
                  <article className="installedModRow" key={`${mod.name}-${mod.version}`}>
                    <div className="modRowIcon" aria-hidden="true">
                      ◇
                    </div>
                    <div className="modRowCopy">
                      <strong>{mod.name}</strong>
                      <small>{t("version_value", { version: mod.version || "?" })}</small>
                      {mod.message && <code>{mod.message}</code>}
                    </div>
                    <span className={`status status-${modStatusClass(mod.status)}`}>
                      {t(`mod_status_${mod.status}`)}
                    </span>
                  </article>
                ))}
              </div>
            </section>
          )}

          {inspection.unverifiedFiles > 0 && (
            <div className="inlineWarning" role="alert">
              <p>{t("package_unverified_files_warning")}</p>
            </div>
          )}

          {inspection.versionStatus !== "installed" && (
            <section className="settingsSection">
              <header>
                <h3>{t("game_version")}</h3>
                <p>{t("import_version_choice_description")}</p>
              </header>
              <div className="formFields">
                {inspection.versionStatus === "available" && (
                  <Checkbox
                    label={t("install_required_version", {
                      version: inspection.gameVersion.name || inspection.gameVersion.id,
                    })}
                    checked={installVersion}
                    onChange={(event) => {
                      setInstallVersion(event.target.checked);
                      if (event.target.checked) {
                        setVersionChoice("");
                      }
                    }}
                  />
                )}
                <Field label={t("use_installed_version")}>
                  <Select
                    value={versionChoice}
                    onValueChange={(value) => {
                      setVersionChoice(value);
                      if (value) {
                        setInstallVersion(false);
                      }
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder={t("select_installed_version_placeholder")} />
                    </SelectTrigger>
                    <SelectContent>
                      {versions.map((version) => (
                        <SelectItem key={version.id} value={version.id}>
                          {version.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </Field>
              </div>
            </section>
          )}

          {problemMods.length > 0 && (
            <section className="settingsSection">
              <header>
                <h3>{t("problem_mods")}</h3>
                <p>{t("problem_mods_description")}</p>
              </header>
              <div className="formFields">
                <Checkbox
                  label={t("allow_incompatible_mods")}
                  checked={allowIncompatible}
                  onChange={(event) => setAllowIncompatible(event.target.checked)}
                />
              </div>
            </section>
          )}

          <section className="settingsSection">
            <header>
              <h3>{t("general")}</h3>
            </header>
            <div className="formFields">
              <Field label={t("name")}>
                <input required value={name} onChange={(event) => setName(event.target.value)} />
              </Field>

              <Field label={t("description")}>
                <textarea
                  value={description}
                  onChange={(event) => setDescription(event.target.value)}
                />
              </Field>
            </div>
          </section>

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
          <Button
            busy={busy}
            disabled={requiresVersionChoice && !useRequiredVersion && versionChoice === ""}
          >
            {t("import")}
          </Button>
        </div>
      </SubmitForm>

      <ConfirmDialog
        open={pendingExternalUrl !== undefined}
        title={t("open_external_link_confirmation")}
        message={t("open_external_link_url", { url: pendingExternalUrl ?? "" })}
        onConfirm={confirmExternalUrl}
        onCancel={() => setPendingExternalUrl(undefined)}
      />
    </Modal>
  );
}

function modStatusClass(status: string): string {
  switch (status) {
    case "available":
      return "installed";
    case "embedded":
      return "ready";
    case "incompatible":
      return "warning";
    case "missing":
      return "failed";
    default:
      return "unknown";
  }
}
