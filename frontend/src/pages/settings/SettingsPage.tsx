import { useQueryClient } from "@tanstack/react-query";
import {
  Archive,
  CircleHelp,
  Download,
  HardDrive,
  Package,
  RefreshCw,
  ShieldCheck,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { useAppShellStore } from "../../app/stores/app-shell";
import { useSupportReportStore } from "../../app/stores/support-report";
import { useToastStore } from "../../app/stores/toast";
import { modCatalogApi } from "../../entities/mod/api";
import { settingsApi } from "../../entities/settings/api";
import type { DataFolder, DataFolderProgress, Settings } from "../../entities/settings/model";
import { useOptimumStatusQuery, useSettingsQuery } from "../../entities/settings/queries";
import { errorCode, errorMessage } from "../../shared/api/bridge";
import {
  DOWNLOADED_MODS_QUERY_KEY,
  OPTIMUM_STATUS_QUERY_KEY,
  SETTINGS_QUERY_KEY,
} from "../../shared/api/keys";
import type { DownloadedModCleanupResult } from "../../shared/api/types";
import { changeAppLanguage } from "../../shared/i18n";
import { normalizeLanguage, supportedLanguages } from "../../shared/i18n/languages";
import { formatBytes } from "../../shared/lib";
import { Button } from "../../shared/ui/button";
import { Card } from "../../shared/ui/card";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { Input } from "../../shared/ui/input";
import { LoadingState } from "../../shared/ui/loading-state";
import { Page, PageContent, PageSection } from "../../shared/ui/page";
import { PageHeader } from "../../shared/ui/page-header";
import { Progress } from "../../shared/ui/progress";
import { SectionHeader } from "../../shared/ui/section-header";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../shared/ui/select";
import { SettingRow } from "../../shared/ui/setting-row";
import { StatusPill } from "../../shared/ui/status-pill";
import { Stepper } from "../../shared/ui/stepper";
import { Switch } from "../../shared/ui/switch";
import { EventsOn } from "../../wailsjs/runtime/runtime";

const autosaveDelayMs = 400;

const SETTINGS_SECTIONS = [
  { id: "downloads", icon: Download, labelKey: "downloads_and_game" },
  { id: "backups", icon: Archive, labelKey: "backups" },
  { id: "updates", icon: RefreshCw, labelKey: "launcher_updates" },
  { id: "optimum", icon: Package, labelKey: "" },
  { id: "data-folder", icon: HardDrive, labelKey: "data_folder" },
  { id: "privacy", icon: ShieldCheck, labelKey: "privacy_and_telemetry" },
  { id: "support", icon: CircleHelp, labelKey: "support" },
] as const;

function settingsEqual(left: Settings, right: Settings) {
  return (
    left.language === right.language &&
    left.downloadsParallel === right.downloadsParallel &&
    left.confirmDeletion === right.confirmDeletion &&
    left.optimumPath === right.optimumPath &&
    left.checkForUpdates === right.checkForUpdates &&
    left.updateChannel === right.updateChannel &&
    left.skippedUpdateVersion === right.skippedUpdateVersion &&
    left.telemetryEnabled === right.telemetryEnabled &&
    left.automaticSafetySnapshots === right.automaticSafetySnapshots &&
    left.automaticSnapshotRetention === right.automaticSnapshotRetention &&
    left.librarySort === right.librarySort &&
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
  const { data: queriedOptimumStatus } = useOptimumStatusQuery();
  const notify = useToastStore((state) => state.notify);
  const currentVersion = useAppShellStore((state) => state.launcherVersion);
  const checkForUpdate = useAppShellStore((state) => state.checkForUpdate);
  const [value, setValue] = useState<Settings>();
  const [launchArgumentsText, setLaunchArgumentsText] = useState("");
  const [dataFolder, setDataFolder] = useState<DataFolder>();
  const [dataFolderProgress, setDataFolderProgress] = useState<DataFolderProgress>();
  const [moveTarget, setMoveTarget] = useState("");
  const [moveDialogOpen, setMoveDialogOpen] = useState(false);
  const [moveTargetBlocked, setMoveTargetBlocked] = useState(false);
  const [moving, setMoving] = useState(false);
  const [moveError, setMoveError] = useState("");
  const [checking, setChecking] = useState(false);
  const [checkingOptimum, setCheckingOptimum] = useState(false);
  const [cleanupPreview, setCleanupPreview] = useState<DownloadedModCleanupResult>();
  const [cleaning, setCleaning] = useState(false);
  const [activeSection, setActiveSection] = useState<string>(SETTINGS_SECTIONS[0].id);
  const persistedRef = useRef<Settings | undefined>(undefined);
  const revisionRef = useRef(0);
  const saveQueueRef = useRef<Promise<void>>(Promise.resolve());
  const notifyRef = useRef(notify);
  const translateRef = useRef(t);
  const optimumRevisionRef = useRef(0);

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
          persistedRef.current = saved;
          if (revision !== revisionRef.current) {
            return;
          }
          setValue(saved);
          queryClient.setQueryData(SETTINGS_QUERY_KEY, saved);
          await queryClient.invalidateQueries({ queryKey: OPTIMUM_STATUS_QUERY_KEY });
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
          await queryClient.invalidateQueries({ queryKey: OPTIMUM_STATUS_QUERY_KEY });
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

  async function detectOptimum() {
    const revision = ++optimumRevisionRef.current;
    setCheckingOptimum(true);
    try {
      await queryClient.cancelQueries({ queryKey: OPTIMUM_STATUS_QUERY_KEY });
      const status = await settingsApi.detectOptimum();
      if (revision !== optimumRevisionRef.current) {
        return;
      }
      queryClient.setQueryData(OPTIMUM_STATUS_QUERY_KEY, status);
      setValue((current) => (current ? { ...current, optimumPath: "" } : current));
    } catch (error) {
      if (revision === optimumRevisionRef.current) {
        notify(errorMessage(error), "error");
      }
    } finally {
      if (revision === optimumRevisionRef.current) {
        setCheckingOptimum(false);
      }
    }
  }

  async function browseOptimum() {
    const revision = ++optimumRevisionRef.current;
    try {
      await queryClient.cancelQueries({ queryKey: OPTIMUM_STATUS_QUERY_KEY });
      const path = await settingsApi.selectOptimumInstallation();
      if (!path || revision !== optimumRevisionRef.current) {
        return;
      }
      const status = await settingsApi.inspectOptimum(path);
      if (revision !== optimumRevisionRef.current) {
        return;
      }
      queryClient.setQueryData(OPTIMUM_STATUS_QUERY_KEY, status);
      setValue((current) => (current ? { ...current, optimumPath: status.path } : current));
    } catch (error) {
      if (revision === optimumRevisionRef.current) {
        notify(errorMessage(error), "error");
      }
    }
  }

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
      setMoveTargetBlocked(false);
      try {
        await settingsApi.validateDataFolderTarget(target);
        setMoveTarget(target);
        setMoveDialogOpen(true);
      } catch (validationError) {
        setMoveTarget(target);
        if (errorCode(validationError) === "FILE_PERMISSION_DENIED") {
          setMoveTargetBlocked(true);
          setMoveDialogOpen(true);
        } else {
          notifyRef.current(errorMessage(validationError), "error");
        }
      }
    } catch (error) {
      notifyRef.current(errorMessage(error), "error");
    }
  }

  function closeMoveDialog() {
    setMoveDialogOpen(false);
    setMoveTarget("");
    setMoveTargetBlocked(false);
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
      notifyRef.current(
        errorCode(error) === "FILE_PERMISSION_DENIED"
          ? t("data_folder_target_not_writable_title")
          : errorMessage(error),
        "error",
      );
    }
  }

  async function previewUnusedDownloadedMods() {
    try {
      const preview = await modCatalogApi.previewUnusedDownloaded();
      if (preview.removedCount === 0) {
        notify(t("no_unused_downloaded_mods"));
        return;
      }
      setCleanupPreview(preview);
    } catch (previewError) {
      notify(errorMessage(previewError), "error");
    }
  }

  async function removeUnusedDownloadedMods() {
    setCleaning(true);
    try {
      const result = await modCatalogApi.removeUnusedDownloaded();
      setCleanupPreview(undefined);
      await queryClient.invalidateQueries({ queryKey: DOWNLOADED_MODS_QUERY_KEY });
      notify(t("unused_downloaded_mods_removed", { count: result.removedCount }));
    } catch (cleanupError) {
      notify(errorMessage(cleanupError), "error");
    } finally {
      setCleaning(false);
    }
  }

  const sectionsReady = Boolean(value);
  useEffect(() => {
    if (!sectionsReady || typeof IntersectionObserver === "undefined") {
      return undefined;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setActiveSection(entry.target.id.replace("settings-", ""));
          }
        }
      },
      { rootMargin: "-15% 0px -70% 0px" },
    );
    for (const section of SETTINGS_SECTIONS) {
      const element = document.getElementById(`settings-${section.id}`);
      if (element) {
        observer.observe(element);
      }
    }
    // Short trailing sections can never reach the observer band; when the
    // scroller hits the bottom, activate the last section instead.
    const scroller = document.querySelector(".appMain");
    const lastSection = SETTINGS_SECTIONS[SETTINGS_SECTIONS.length - 1].id;
    const handleScroll = () => {
      if (scroller && scroller.scrollTop + scroller.clientHeight >= scroller.scrollHeight - 8) {
        setActiveSection(lastSection);
      }
    };
    scroller?.addEventListener("scroll", handleScroll, { passive: true });
    return () => {
      observer.disconnect();
      scroller?.removeEventListener("scroll", handleScroll);
    };
  }, [sectionsReady]);

  if (!value) {
    return (
      <Page>
        <PageHeader
          eyebrow={t("make_it_yours")}
          title={t("settings")}
          description={t("basic_launcher_preferences")}
        />
        <PageContent>
          <LoadingState />
        </PageContent>
      </Page>
    );
  }

  const optimumStatus = queriedOptimumStatus;
  const folderError = moveError
    ? moveError
    : dataFolder?.lastError
      ? t("data_folder_previous_error", { message: dataFolder.lastError })
      : undefined;

  return (
    <Page>
      <PageHeader
        eyebrow={t("make_it_yours")}
        title={t("settings")}
        description={t("basic_launcher_preferences")}
      />

      <PageContent>
        <div className="settingsLayout">
          <nav className="settingsNav" aria-label={t("settings")}>
            {SETTINGS_SECTIONS.map((section) => (
              <button
                key={section.id}
                type="button"
                className={activeSection === section.id ? "active" : ""}
                aria-current={activeSection === section.id ? "true" : undefined}
                onClick={() => {
                  setActiveSection(section.id);
                  document
                    .getElementById(`settings-${section.id}`)
                    ?.scrollIntoView({ behavior: "smooth", block: "start" });
                }}
              >
                <section.icon size={15} aria-hidden="true" />
                {section.labelKey ? t(section.labelKey) : "Optimum"}
              </button>
            ))}
          </nav>

          <div className="settingsSections">
            <PageSection id="settings-downloads">
              <SectionHeader
                variant="compact"
                title={t("downloads_and_game")}
                description={t("background_work_and_launch_configuration")}
              />
              <Card variant="subtle" className="divide-y divide-border-subtle">
                <SettingRow title={t("language")}>
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
                </SettingRow>

                <SettingRow
                  title={t("parallel_downloads")}
                  description={t("parallel_downloads_description")}
                >
                  <Stepper
                    label={t("parallel_downloads")}
                    value={value.downloadsParallel}
                    min={1}
                    max={10}
                    decreaseLabel={t("decrease")}
                    increaseLabel={t("increase")}
                    onChange={(downloadsParallel) => setValue({ ...value, downloadsParallel })}
                  />
                </SettingRow>

                <SettingRow
                  column
                  title={t("global_launch_arguments")}
                  description={t("global_launch_arguments_description")}
                >
                  <Input
                    className="codeInput"
                    aria-label={t("global_launch_arguments")}
                    value={launchArgumentsText}
                    onChange={(event) => {
                      const argumentsValue = event.target.value;
                      setLaunchArgumentsText(argumentsValue);
                      const trimmedArguments = argumentsValue.trim();
                      setValue({
                        ...value,
                        globalLaunchArguments: trimmedArguments
                          ? trimmedArguments.split(/\s+/)
                          : [],
                      });
                    }}
                    placeholder="--debug"
                  />
                </SettingRow>

                <SettingRow
                  title={t("confirm_deletion")}
                  description={t("confirm_before_removing_items")}
                >
                  <Switch
                    label={t("confirm_deletion")}
                    checked={value.confirmDeletion}
                    onCheckedChange={(confirmDeletion) => setValue({ ...value, confirmDeletion })}
                  />
                </SettingRow>
              </Card>
            </PageSection>

            <PageSection id="settings-backups">
              <SectionHeader
                variant="compact"
                title={t("backups")}
                description={t("backups_settings_description")}
              />
              <Card variant="subtle" className="divide-y divide-border-subtle">
                <SettingRow
                  title={t("automatic_safety_backups")}
                  description={t("automatic_safety_backups_description")}
                >
                  <Switch
                    label={t("automatic_safety_backups")}
                    checked={value.automaticSafetySnapshots}
                    onCheckedChange={(automaticSafetySnapshots) =>
                      setValue({ ...value, automaticSafetySnapshots })
                    }
                  />
                </SettingRow>
                <SettingRow title={t("automatic_snapshots_to_keep")}>
                  <Stepper
                    label={t("automatic_snapshots_to_keep")}
                    value={value.automaticSnapshotRetention}
                    min={1}
                    max={100}
                    decreaseLabel={t("decrease")}
                    increaseLabel={t("increase")}
                    onChange={(automaticSnapshotRetention) =>
                      setValue({ ...value, automaticSnapshotRetention })
                    }
                  />
                </SettingRow>
              </Card>
            </PageSection>

            <PageSection id="settings-updates">
              <SectionHeader
                variant="compact"
                title={t("launcher_updates")}
                description={t("launcher_updates_description")}
              />
              <Card variant="subtle" className="divide-y divide-border-subtle">
                <SettingRow
                  title={t("automatically_check_for_updates")}
                  description={t("automatic_updates_consent_notice")}
                >
                  <Switch
                    label={t("automatically_check_for_updates")}
                    checked={value.checkForUpdates}
                    onCheckedChange={(checkForUpdates) => setValue({ ...value, checkForUpdates })}
                  />
                </SettingRow>

                <SettingRow title={t("update_channel")}>
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
                </SettingRow>

                <SettingRow title={t("current_launcher_version")}>
                  <Input
                    className="w-[220px]"
                    value={currentVersion || "—"}
                    readOnly
                    aria-label={t("current_launcher_version")}
                  />
                </SettingRow>

                <SettingRow title={t("check_for_updates")}>
                  <Button
                    type="button"
                    variant="secondary"
                    busy={checking}
                    onClick={async () => {
                      setChecking(true);
                      try {
                        await checkForUpdate(value.updateChannel, true);
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
                </SettingRow>
              </Card>
            </PageSection>

            <PageSection id="settings-optimum">
              <SectionHeader
                variant="compact"
                title="Optimum"
                description={
                  <>
                    {t("optimum_settings_description")}{" "}
                    <button
                      type="button"
                      className="linkButton"
                      onClick={() => void settingsApi.openOptimumInstallationGuide()}
                    >
                      {t("optimum_installation_guide")}
                    </button>
                  </>
                }
              />
              <Card variant="subtle" className="divide-y divide-border-subtle">
                <SettingRow column title={t("installation")}>
                  <Input
                    className="codeInput"
                    value={optimumStatus?.path || t("optimum_not_detected")}
                    readOnly
                    aria-label={t("optimum_installation")}
                  />
                </SettingRow>
                <SettingRow
                  title={t("status")}
                  description={
                    optimumStatus?.ready
                      ? optimumStatus.gameVersion
                        ? t("optimum_vintage_story_version", {
                            version: optimumStatus.gameVersion,
                          })
                        : t("optimum_ready")
                      : optimumStatus?.message || t("optimum_not_configured_description")
                  }
                >
                  <StatusPill status={optimumStatus?.ready ? "ready" : "unknown"} />
                </SettingRow>
                <SettingRow>
                  <div className="row">
                    <Button
                      type="button"
                      variant="secondary"
                      busy={checkingOptimum}
                      onClick={() => void detectOptimum()}
                    >
                      {t("detect")}
                    </Button>
                    <Button type="button" variant="secondary" onClick={() => void browseOptimum()}>
                      {t("browse")}
                    </Button>
                  </div>
                </SettingRow>
              </Card>
            </PageSection>

            <PageSection id="settings-data-folder">
              <SectionHeader
                variant="compact"
                title={t("data_folder")}
                description={t("data_folder_description")}
              />
              <Card variant="subtle" className="divide-y divide-border-subtle">
                {moving ? (
                  <SettingRow column title={t("data_folder_moving")}>
                    <Progress value={Math.round((dataFolderProgress?.progress ?? 0) * 100)} />
                    {dataFolderProgress?.totalBytes ? (
                      <small className="settingRowDescription">
                        {formatBytes(dataFolderProgress.copiedBytes)} /{" "}
                        {formatBytes(dataFolderProgress.totalBytes)}
                      </small>
                    ) : null}
                  </SettingRow>
                ) : (
                  <>
                    <SettingRow
                      column
                      title={t("data_folder_current_location")}
                      warning={folderError}
                    >
                      <Input
                        className="codeInput"
                        value={dataFolder?.currentPath ?? "—"}
                        readOnly
                        aria-label={t("data_folder_current_location")}
                      />
                    </SettingRow>
                    <SettingRow>
                      <Button
                        type="button"
                        variant="secondary"
                        onClick={() => void chooseDataFolder()}
                      >
                        {t("data_folder_change")}
                      </Button>
                    </SettingRow>
                    <SettingRow>
                      <Button
                        type="button"
                        variant="danger"
                        onClick={() => void previewUnusedDownloadedMods()}
                      >
                        {t("remove_unused_downloaded_mods")}
                      </Button>
                    </SettingRow>
                  </>
                )}
              </Card>
            </PageSection>

            <PageSection id="settings-privacy">
              <SectionHeader
                variant="compact"
                title={t("privacy_and_telemetry")}
                description={t("privacy_and_telemetry_description")}
              />
              <Card variant="subtle" className="divide-y divide-border-subtle">
                <SettingRow
                  title={t("send_usage_analytics")}
                  description={t("telemetry_consent_notice")}
                >
                  <Switch
                    label={t("send_usage_analytics")}
                    checked={value.telemetryEnabled}
                    onCheckedChange={(telemetryEnabled) => setValue({ ...value, telemetryEnabled })}
                  />
                </SettingRow>
              </Card>
            </PageSection>

            <PageSection id="settings-support">
              <SectionHeader
                variant="compact"
                title={t("support")}
                description={t("support_description")}
              />
              <Card variant="subtle">
                <SettingRow
                  title={t("report_a_problem")}
                  description={t("support_report_setting_description")}
                >
                  <Button
                    variant="secondary"
                    onClick={() => useSupportReportStore.getState().show()}
                  >
                    {t("report_a_problem")}
                  </Button>
                </SettingRow>
              </Card>
            </PageSection>

            <p className="px-1 text-[12px] leading-relaxed text-text-disabled">
              {t("not_affiliated_notice")}
            </p>
          </div>
        </div>
      </PageContent>

      <ConfirmDialog
        open={moveDialogOpen}
        title={t(
          moveTargetBlocked
            ? "data_folder_target_not_writable_title"
            : "data_folder_move_confirm_title",
        )}
        message={moveTargetBlocked ? undefined : t("data_folder_move_confirm_message")}
        warningMessage={moveTargetBlocked ? undefined : t("data_folder_move_confirm_warning")}
        confirmLabel={t("data_folder_move")}
        destructive
        hideConfirm={moveTargetBlocked}
        onConfirm={() => void confirmDataFolderMove()}
        onCancel={closeMoveDialog}
      >
        {moveTargetBlocked && (
          <div className="px-6 pt-6">
            <div
              role="alert"
              className="rounded-lg border border-danger-border bg-danger-surface px-4 py-3 text-danger"
            >
              <p className="text-[13px] leading-6">{t("data_folder_target_not_writable_hint")}</p>
            </div>
          </div>
        )}
      </ConfirmDialog>

      <ConfirmDialog
        open={cleanupPreview !== undefined}
        title={t("remove_unused_downloaded_mods")}
        message={t("unused_downloaded_mods_confirm", { count: cleanupPreview?.removedCount ?? 0 })}
        warningMessage={t("unused_downloaded_mods_warning")}
        confirmLabel={t("remove")}
        destructive
        loading={cleaning}
        onConfirm={() => void removeUnusedDownloadedMods()}
        onCancel={() => setCleanupPreview(undefined)}
      />
    </Page>
  );
}
