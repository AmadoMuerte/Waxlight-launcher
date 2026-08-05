import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

import { changeAppLanguage } from "../i18n";
import { normalizeLanguage, supportedLanguages } from "../i18n/languages";
import { Settings, settingsApi } from "../shared/api";
import { errorMessage } from "../shared/api/bridge";
import { Checkbox, Field, PageHeader } from "../shared/ui";

type Notify = (message: string, type?: "ok" | "error") => void;

interface SettingsPageProps {
  settings?: Settings;
  notify: Notify;
  onSaved: (settings: Settings) => void;
  currentVersion?: string;
}

const autosaveDelayMs = 400;

function settingsEqual(left: Settings, right: Settings) {
  return (
    left.theme === right.theme &&
    left.language === right.language &&
    left.downloadsParallel === right.downloadsParallel &&
    left.confirmDeletion === right.confirmDeletion &&
    left.minSessionDurationSec === right.minSessionDurationSec &&
    left.checkForUpdates === right.checkForUpdates &&
    left.updateChannel === right.updateChannel &&
    left.skippedUpdateVersion === right.skippedUpdateVersion &&
    left.globalLaunchArguments.length === right.globalLaunchArguments.length &&
    left.globalLaunchArguments.every(
      (argument, index) => argument === right.globalLaunchArguments[index],
    )
  );
}

export function SettingsPage({ settings, notify, onSaved, currentVersion }: SettingsPageProps) {
  const { t } = useTranslation();
  const [value, setValue] = useState<Settings>();
  const [launchArgumentsText, setLaunchArgumentsText] = useState("");
  const persistedRef = useRef<Settings | undefined>(undefined);
  const revisionRef = useRef(0);
  const saveQueueRef = useRef<Promise<void>>(Promise.resolve());
  const notifyRef = useRef(notify);
  const onSavedRef = useRef(onSaved);
  const translateRef = useRef(t);

  notifyRef.current = notify;
  onSavedRef.current = onSaved;
  translateRef.current = t;

  useEffect(() => {
    if (!settings) {
      return;
    }
    persistedRef.current = settings;
    setValue((current) =>
      current ? { ...current, skippedUpdateVersion: settings.skippedUpdateVersion } : settings,
    );
    setLaunchArgumentsText((current) => current || settings.globalLaunchArguments.join(" "));
  }, [settings]);

  useEffect(() => {
    const persisted = persistedRef.current;
    if (!value || !persisted || settingsEqual(value, persisted)) {
      return undefined;
    }

    const next = {
      ...value,
      language: normalizeLanguage(value.language),
      globalLaunchArguments: [...value.globalLaunchArguments],
    };
    const revision = ++revisionRef.current;
    const timer = window.setTimeout(() => {
      async function persist() {
        if (revision !== revisionRef.current) {
          return;
        }

        try {
          const saved = await settingsApi.update(next);
          if (revision !== revisionRef.current) {
            return;
          }
          persistedRef.current = saved;
          setValue(saved);
          onSavedRef.current(saved);
          await changeAppLanguage(saved.language);
          notifyRef.current(translateRef.current("settings_saved"));
        } catch (error) {
          if (revision !== revisionRef.current) {
            return;
          }
          const previous = persistedRef.current;
          if (previous) {
            setValue(previous);
            setLaunchArgumentsText(previous.globalLaunchArguments.join(" "));
            await changeAppLanguage(previous.language);
          }
          notifyRef.current(errorMessage(error), "error");
        }
      }

      saveQueueRef.current = saveQueueRef.current.then(persist, persist);
    }, autosaveDelayMs);

    return () => window.clearTimeout(timer);
  }, [value]);

  useEffect(
    () => () => {
      revisionRef.current += 1;
    },
    [],
  );

  if (!value) {
    return null;
  }

  return (
    <>
      <PageHeader
        eyebrow={t("make_it_yours")}
        title={t("settings")}
        description={t("basic_launcher_preferences")}
      />

      <section className="settingsPanel">
        <div className="settingsPageForm">
          <section className="settingsPageSection">
            <header>
              <h2>{t("interface")}</h2>
              <p>{t("language_and_appearance_preferences")}</p>
            </header>
            <div className="formRow">
              <Field label={t("language")}>
                <Select
                  value={normalizeLanguage(value.language)}
                  onValueChange={(language) => {
                    const normalized = normalizeLanguage(language);
                    setValue({ ...value, language: normalized });
                    void changeAppLanguage(normalized);
                  }}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {supportedLanguages.map((language) => (
                      <SelectItem key={language.code} value={language.code}>
                        {language.nativeName}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>

              <Field label={t("theme")}>
                <Select
                  value={value.theme}
                  onValueChange={(theme) => setValue({ ...value, theme })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
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

              <Field label={t("global_launch_arguments")} hint={t("global_launch_arguments_hint")}>
                <input
                  className="codeInput"
                  value={launchArgumentsText}
                  onChange={(event) => {
                    const argumentsValue = event.target.value;
                    setLaunchArgumentsText(argumentsValue);
                    const trimmedArguments = argumentsValue.trim();
                    setValue({
                      ...value,
                      globalLaunchArguments: trimmedArguments ? trimmedArguments.split(/\s+/) : [],
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

          <section className="settingsPageSection">
            <header>
              <h2>{t("launcher_updates")}</h2>
              <p>{t("launcher_updates_description")}</p>
            </header>
            <div className="formFields">
              <div className="checkboxSetting">
                <Checkbox
                  label={t("automatically_check_for_updates")}
                  checked={value.checkForUpdates}
                  onChange={(event) =>
                    setValue({ ...value, checkForUpdates: event.target.checked })
                  }
                />
                <small>{t("automatic_updates_consent_notice")}</small>
              </div>

              <div className="formRow">
                <Field label={t("update_channel")}>
                  <Select
                    value={value.updateChannel}
                    onValueChange={(updateChannel) => {
                      if (updateChannel !== "stable" && updateChannel !== "prerelease") {
                        return;
                      }
                      setValue({
                        ...value,
                        updateChannel,
                        skippedUpdateVersion: "",
                      });
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="stable">{t("stable")}</SelectItem>
                      <SelectItem value="prerelease">{t("prerelease")}</SelectItem>
                    </SelectContent>
                  </Select>
                </Field>

                <Field label={t("current_launcher_version")}>
                  <input value={currentVersion || "—"} readOnly />
                </Field>
              </div>
            </div>
          </section>
        </div>
      </section>

      <footer className="legal">{t("not_affiliated_notice")}</footer>
    </>
  );
}
