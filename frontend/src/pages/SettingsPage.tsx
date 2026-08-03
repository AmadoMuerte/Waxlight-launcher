import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { changeAppLanguage } from "../i18n";
import { normalizeLanguage, supportedLanguages } from "../i18n/languages";

import { Settings, settingsApi } from "../shared/api";
import { errorMessage } from "../shared/api/bridge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Button, Checkbox, Field, PageHeader, SubmitForm } from "../shared/ui";

type Notify = (message: string, type?: "ok" | "error") => void;

interface SettingsPageProps {
  settings?: Settings;
  notify: Notify;
  onSaved: (settings: Settings) => void;
}

export function SettingsPage({
  settings,
  notify,
  onSaved,
}: SettingsPageProps) {
  const { t } = useTranslation();
  const [value, setValue] = useState<Settings>();
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setValue(settings);
  }, [settings]);

  if (!value) {
    return null;
  }

  async function save() {
    if (!value) {
      return;
    }

    setBusy(true);

    try {
      const saved = await settingsApi.update({
        ...value,
        language: normalizeLanguage(value.language),
      });
      await changeAppLanguage(saved.language);
      onSaved(saved);
      notify(t("settings_saved"));
    } catch (error) {
      notify(errorMessage(error), "error");
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <PageHeader
        eyebrow={t("make_it_yours")}
        title={t("settings")}
        description={t("basic_launcher_preferences")}
      />

      <section className="settingsPanel">
        <SubmitForm className="settingsPageForm" onSubmit={save}>
          <section className="settingsPageSection">
            <header>
              <h2>{t("interface")}</h2>
              <p>{t("language_and_appearance_preferences")}</p>
            </header>
            <div className="formRow">
              <Field label={t("language")}>
                <Select value={normalizeLanguage(value.language)} onValueChange={(language) => setValue({ ...value, language: normalizeLanguage(language) })}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {supportedLanguages.map((language) => <SelectItem key={language.code} value={language.code}>{language.nativeName}</SelectItem>)}
                  </SelectContent>
                </Select>
              </Field>

              <Field label={t("theme")}>
                <Select
                  value={value.theme}
                  onValueChange={(theme) =>
                    setValue({ ...value, theme })
                  }
                >
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="dark">{t("dark")}</SelectItem>
                    <SelectItem value="system">{t("system")}</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
            </div>
          </section>

          <section className="settingsPageSection">
            <header>
              <h2>{t("downloads_and_game")}</h2>
              <p>{t("background_work_and_launch_configuration")}</p>
            </header>
            <div className="formFields">
              <div className="formRow">
                <Field label={t("parallel_downloads")}>
                  <input
                    type="number"
                    min={1}
                    max={10}
                    value={value.downloadsParallel}
                    onChange={(event) =>
                      setValue({
                        ...value,
                        downloadsParallel: Number(event.target.value),
                      })
                    }
                  />
                </Field>

                <Field label={t("minimum_session_duration_seconds")}>
                  <input
                    type="number"
                    min={0}
                    value={value.minSessionDurationSec}
                    onChange={(event) =>
                      setValue({
                        ...value,
                        minSessionDurationSec: Number(event.target.value),
                      })
                    }
                  />
                </Field>
              </div>

              <Field
                label={t("global_launch_arguments")}
                hint={t("global_launch_arguments_hint")}
              >
                <input
                  className="codeInput"
                  value={value.globalLaunchArguments.join(" ")}
                  onChange={(event) => {
                    const argumentsValue = event.target.value.trim();
                    setValue({
                      ...value,
                      globalLaunchArguments: argumentsValue
                        ? argumentsValue.split(/\s+/)
                        : [],
                    });
                  }}
                  placeholder="--debug"
                />
              </Field>

              <div className="checkboxSetting">
                <Checkbox
                  label={t("confirm_deletion")}
                  checked={value.confirmDeletion}
                  onChange={(event) =>
                    setValue({ ...value, confirmDeletion: event.target.checked })
                  }
                />
                <small>{t("confirm_before_removing_items")}</small>
              </div>
            </div>
          </section>

          <div className="settingsPageFooter">
            <Button busy={busy}>{t("save_settings")}</Button>
          </div>
        </SubmitForm>
      </section>

      <footer className="legal">
        {t("not_affiliated_notice")}
      </footer>
    </>
  );
}
