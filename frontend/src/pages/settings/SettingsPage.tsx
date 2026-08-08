import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Progress } from "@/shared/ui/progress";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui/select";

import { useAppShellStore } from "../../app/stores/app-shell";
import { useToastStore } from "../../app/stores/toast";
import { settingsApi } from "../../entities/settings/api";
import type { DataFolder, DataFolderProgress, Settings } from "../../entities/settings/model";
import { useSettingsQuery } from "../../entities/settings/queries";
import { errorMessage } from "../../shared/api/bridge";
import { SETTINGS_QUERY_KEY } from "../../shared/api/keys";
import { changeAppLanguage } from "../../shared/i18n";
import { normalizeLanguage, supportedLanguages } from "../../shared/i18n/languages";
import { formatBytes } from "../../shared/lib";
import { Button } from "../../shared/ui/button";
import { PageHeader } from "../../shared/ui/page-header";
import { Stepper } from "../../shared/ui/stepper";
import { Switch } from "../../shared/ui/switch";
import { EventsOn } from "../../wailsjs/runtime/runtime";

const autosaveDelayMs = 400;

function settingsEqual(left: Settings, right: Settings) {
  return (
    left.language === right.language &&
    left.downloadsParallel === right.downloadsParallel &&
    left.confirmDeletion === right.confirmDeletion &&
    left.checkForUpdates === right.checkForUpdates &&
    left.updateChannel === right.updateChannel &&
    left.skippedUpdateVersion === right.skippedUpdateVersion &&
    left.telemetryEnabled === right.telemetryEnabled &&
    left.automaticSafetySnapshots === right.automaticSafetySnapshots &&
    left.globalLaunchArguments.length === right.globalLaunchArguments.length &&
    left.globalLaunchArguments.every(
      (argument, index) => argument === right.globalLaunchArguments[index],
    )
  );
}

export function SettingsPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data: settings } = useSettingsQuery();
  const notify = useToastStore((state) => state.notify);
  const currentVersion = useAppShellStore((state) => state.launcherVersion);
  const checkForUpdate = useAppShellStore((state) => state.checkForUpdate);
  const [value, setValue] = useState<Settings>();
  const [launchArgumentsText, setLaunchArgumentsText] = useState("");
  const [dataFolder, setDataFolder] = useState<DataFolder>();
  const [dataFolderProgress, setDataFolderProgress] = useState<DataFolderProgress>();
  const [moveTarget, setMoveTarget] = useState("");
  const [moveDialogOpen, setMoveDialogOpen] = useState(false);
  const [moving, setMoving] = useState(false);
  const [moveError, setMoveError] = useState("");
  const [checking, setChecking] = useState(false);
  const persistedRef = useRef<Settings | undefined>(undefined);
  const revisionRef = useRef(0);
  const saveQueueRef = useRef<Promise<void>>(Promise.resolve());
  const notifyRef = useRef(notify);
  const translateRef = useRef(t);

  notifyRef.current = notify;
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
          queryClient.setQueryData(SETTINGS_QUERY_KEY, saved);
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
  }, [queryClient, value]);

  useEffect(
    () => () => {
      revisionRef.current += 1;
    },
    [],
  );

  useEffect(() => {
    void settingsApi
      .getDataFolder()
      .then(setDataFolder)
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    try {
      return EventsOn("data-folder:progress", (progress: DataFolderProgress) => {
        setDataFolderProgress(progress);
        if (progress.phase === "relaunching") {
          setMoving(true);
        }
      });
    } catch {
      return undefined;
    }
  }, []);

  useEffect(() => {
    try {
      return EventsOn("data-folder:error", (payload: { message?: string }) => {
        const message = payload?.message ?? "";
        setMoving(false);
        setMoveError(message);
        notifyRef.current(
          message ? message : translateRef.current("data_folder_move_error"),
          "error",
        );
      });
    } catch {
      return undefined;
    }
  }, []);

  async function chooseDataFolder() {
    if (moving) {
      return;
    }
    try {
      const target = await settingsApi.selectDataFolder();
      if (!target) {
        return;
      }
      setMoveTarget(target);
      setMoveDialogOpen(true);
    } catch (error) {
      notifyRef.current(errorMessage(error), "error");
    }
  }

  async function confirmDataFolderMove() {
    if (!moveTarget) {
      return;
    }
    setMoveDialogOpen(false);
    setMoving(true);
    setMoveError("");
    setDataFolderProgress({ copiedBytes: 0, totalBytes: 0, progress: 0, phase: "preparing" });
    try {
      await settingsApi.moveDataFolder(moveTarget);
    } catch (error) {
      setMoving(false);
      notifyRef.current(errorMessage(error), "error");
    }
  }

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
              <h2>{t("downloads_and_game")}</h2>
              <p>{t("background_work_and_launch_configuration")}</p>
            </header>
            <div className="downloadsPanel">
              <div className="settingRow">
                <div className="settingRowText">
                  <span className="settingRowTitle">{t("language")}</span>
                </div>
                <div className="settingRowControl">
                  <Select
                    value={normalizeLanguage(value.language)}
                    onValueChange={(language) => {
                      const normalized = normalizeLanguage(language);
                      setValue({ ...value, language: normalized });
                      void changeAppLanguage(normalized);
                    }}
                  >
                    <SelectTrigger className="w-[220px]">
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
                </div>
              </div>

              <div className="settingRowDivider" />

              <div className="settingRow">
                <div className="settingRowText">
                  <span className="settingRowTitle">{t("parallel_downloads")}</span>
                  <small className="settingRowDescription">
                    {t("parallel_downloads_description")}
                  </small>
                </div>
                <div className="settingRowControl">
                  <Stepper
                    label={t("parallel_downloads")}
                    value={value.downloadsParallel}
                    min={1}
                    max={10}
                    decreaseLabel={t("decrease")}
                    increaseLabel={t("increase")}
                    onChange={(downloadsParallel) => setValue({ ...value, downloadsParallel })}
                  />
                </div>
              </div>

              <div className="settingRowDivider" />

              <div className="settingRow settingRowColumn">
                <div className="settingRowText">
                  <span className="settingRowTitle">{t("global_launch_arguments")}</span>
                  <small className="settingRowDescription">
                    {t("global_launch_arguments_description")}
                  </small>
                </div>
                <input
                  className="settingTileInput codeInput"
                  aria-label={t("global_launch_arguments")}
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
              </div>

              <div className="settingRowDivider" />

              <div className="settingRow">
                <div className="settingRowText">
                  <span className="settingRowTitle">{t("confirm_deletion")}</span>
                  <small className="settingRowDescription">
                    {t("confirm_before_removing_items")}
                  </small>
                </div>
                <div className="settingRowControl">
                  <Switch
                    label={t("confirm_deletion")}
                    checked={value.confirmDeletion}
                    onCheckedChange={(confirmDeletion) => setValue({ ...value, confirmDeletion })}
                  />
                </div>
              </div>
            </div>
          </section>

          <section className="settingsPageSection">
            <header>
              <h2>{t("backups")}</h2>
              <p>{t("backups_settings_description")}</p>
            </header>
            <div className="downloadsPanel">
              <div className="settingRow">
                <div className="settingRowText">
                  <span className="settingRowTitle">{t("automatic_safety_backups")}</span>
                  <small className="settingRowDescription">
                    {t("automatic_safety_backups_description")}
                  </small>
                </div>
                <div className="settingRowControl">
                  <Switch
                    label={t("automatic_safety_backups")}
                    checked={value.automaticSafetySnapshots}
                    onCheckedChange={(automaticSafetySnapshots) =>
                      setValue({ ...value, automaticSafetySnapshots })
                    }
                  />
                </div>
              </div>
            </div>
          </section>

          <section className="settingsPageSection">
            <header>
              <h2>{t("launcher_updates")}</h2>
              <p>{t("launcher_updates_description")}</p>
            </header>
            <div className="downloadsPanel">
              <div className="settingRow">
                <div className="settingRowText">
                  <span className="settingRowTitle">{t("automatically_check_for_updates")}</span>
                  <small className="settingRowDescription">
                    {t("automatic_updates_consent_notice")}
                  </small>
                </div>
                <div className="settingRowControl">
                  <Switch
                    label={t("automatically_check_for_updates")}
                    checked={value.checkForUpdates}
                    onCheckedChange={(checkForUpdates) => setValue({ ...value, checkForUpdates })}
                  />
                </div>
              </div>

              <div className="settingRowDivider" />

              <div className="settingRow">
                <div className="settingRowText">
                  <span className="settingRowTitle">{t("update_channel")}</span>
                </div>
                <div className="settingRowControl">
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
                    <SelectTrigger className="w-[220px]">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="stable">{t("stable")}</SelectItem>
                      <SelectItem value="prerelease">{t("prerelease")}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>

              <div className="settingRowDivider" />

              <div className="settingRow">
                <div className="settingRowText">
                  <span className="settingRowTitle">{t("current_launcher_version")}</span>
                </div>
                <div className="settingRowControl">
                  <input
                    className="settingTileInput w-[220px]"
                    value={currentVersion || "—"}
                    readOnly
                    aria-label={t("current_launcher_version")}
                  />
                </div>
              </div>

              <div className="settingRowDivider" />

              <div className="settingRow">
                <div className="settingRowText">
                  <span className="settingRowTitle">{t("check_for_updates")}</span>
                </div>
                <div className="settingRowControl">
                  <Button
                    type="button"
                    variant="secondary"
                    busy={checking}
                    onClick={async () => {
                      setChecking(true);
                      try {
                        await checkForUpdate(value.updateChannel, value.skippedUpdateVersion);
                        // If no update found, launcherUpdate stays undefined.
                        if (!useAppShellStore.getState().launcherUpdate) {
                          notify(t("launcher_is_up_to_date"));
                        }
                      } finally {
                        setChecking(false);
                      }
                    }}
                  >
                    {t("check_for_updates")}
                  </Button>
                </div>
              </div>
            </div>
          </section>

          <section className="settingsPageSection">
            <header>
              <h2>{t("data_folder")}</h2>
              <p>{t("data_folder_description")}</p>
            </header>
            <div className="downloadsPanel">
              {moving ? (
                <div className="settingRow settingRowColumn">
                  <div className="settingRowText">
                    <span className="settingRowTitle">{t("data_folder_moving")}</span>
                  </div>
                  <Progress value={dataFolderProgress?.progress ?? 0} />
                  {dataFolderProgress?.totalBytes ? (
                    <small className="settingRowDescription">
                      {formatBytes(dataFolderProgress.copiedBytes)} /{" "}
                      {formatBytes(dataFolderProgress.totalBytes)}
                    </small>
                  ) : null}
                </div>
              ) : (
                <>
                  <div className="settingRow settingRowColumn">
                    <div className="settingRowText">
                      <span className="settingRowTitle">{t("data_folder_current_location")}</span>
                    </div>
                    <input
                      className="settingTileInput codeInput"
                      value={dataFolder?.currentPath ?? "—"}
                      readOnly
                      aria-label={t("data_folder_current_location")}
                    />
                  </div>
                  {moveError && <p className="updateHint">{moveError}</p>}
                  {dataFolder?.lastError && (
                    <p className="updateHint">
                      {t("data_folder_previous_error", { message: dataFolder.lastError })}
                    </p>
                  )}
                  <div className="settingRowDivider" />
                  <div className="settingRow">
                    <div className="settingRowControl">
                      <Button
                        type="button"
                        variant="secondary"
                        onClick={() => void chooseDataFolder()}
                      >
                        {t("data_folder_change")}
                      </Button>
                    </div>
                  </div>
                </>
              )}
            </div>
          </section>

          <section className="settingsPageSection">
            <header>
              <h2>{t("privacy_and_telemetry")}</h2>
              <p>{t("privacy_and_telemetry_description")}</p>
            </header>
            <div className="downloadsPanel">
              <div className="settingRow">
                <div className="settingRowText">
                  <span className="settingRowTitle">{t("send_usage_analytics")}</span>
                  <small className="settingRowDescription">{t("telemetry_consent_notice")}</small>
                </div>
                <div className="settingRowControl">
                  <Switch
                    label={t("send_usage_analytics")}
                    checked={value.telemetryEnabled}
                    onCheckedChange={(telemetryEnabled) => setValue({ ...value, telemetryEnabled })}
                  />
                </div>
              </div>
            </div>
          </section>
        </div>
      </section>

      <ConfirmDialog
        open={moveDialogOpen}
        title={t("data_folder_move_confirm_title")}
        message={t("data_folder_move_confirm_message")}
        warningMessage={t("data_folder_move_confirm_warning")}
        confirmLabel={t("data_folder_move")}
        destructive
        onConfirm={() => void confirmDataFolderMove()}
        onCancel={() => setMoveDialogOpen(false)}
      />

      <footer className="legal">{t("not_affiliated_notice")}</footer>
    </>
  );
}
