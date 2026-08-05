import { useCallback, useEffect, useRef, useState } from "react";
import { Navigate, Route, Routes } from "react-router-dom";

import { AppToast } from "../components/layout/AppToast";
import { ErrorBanner } from "../components/layout/ErrorBanner";
import { Sidebar } from "../components/layout/Sidebar";
import { UpdateNotice } from "../components/layout/UpdateNotice";
import { changeAppLanguage } from "../i18n";
import { AccountsPage } from "../pages/AccountsPage";
import { LibraryPage } from "../pages/LibraryPage";
import { ModDetailsPage } from "../pages/ModDetailsPage";
import { ModsPage } from "../pages/ModsPage";
import { OperationsPage } from "../pages/OperationsPage";
import { SettingsPage } from "../pages/SettingsPage";
import { StatisticsPage } from "../pages/StatisticsPage";
import { VersionsPage } from "../pages/VersionsPage";
import {
  accountsApi,
  instancesApi,
  operationsApi,
  settingsApi,
  statisticsApi,
  updatesApi,
  versionsApi,
  type Account,
  type GameVersion,
  type Instance,
  type LauncherUpdate,
  type LauncherUpdateProgress,
  type Operation,
  type Settings,
  type Statistics,
} from "../shared/api";
import { errorMessage } from "../shared/api/bridge";
import { Environment, EventsOn } from "../wailsjs/runtime/runtime";

type ToastType = "ok" | "error";

export function App() {
  const updateCheckSequenceRef = useRef(0);

  const [instances, setInstances] = useState<Instance[]>([]);
  const [versions, setVersions] = useState<GameVersion[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [operations, setOperations] = useState<Operation[]>([]);
  const [statistics, setStatistics] = useState<Statistics>();
  const [settings, setSettings] = useState<Settings>();
  const [launcherVersion, setLauncherVersion] = useState("");
  const [launcherUpdate, setLauncherUpdate] = useState<LauncherUpdate>();
  const [updateProgress, setUpdateProgress] = useState<LauncherUpdateProgress>();
  const [installingUpdate, setInstallingUpdate] = useState(false);
  const [loading, setLoading] = useState(true);
  const [fatalError, setFatalError] = useState("");
  const [toast, setToast] = useState<{
    message: string;
    type: ToastType;
  }>();
  const [platform, setPlatform] = useState("");

  const checkLauncherUpdate = useCallback(
    async (channel: Settings["updateChannel"], skippedVersion: string) => {
      const sequence = ++updateCheckSequenceRef.current;

      try {
        const update = await updatesApi.check(channel);

        if (sequence !== updateCheckSequenceRef.current) {
          return;
        }

        setLauncherVersion(update.installedVersion);

        if (update.available && update.version !== skippedVersion) {
          setLauncherUpdate(update);
        } else {
          setLauncherUpdate(undefined);
        }
      } catch {
        if (sequence === updateCheckSequenceRef.current) {
          setLauncherUpdate(undefined);
        }
      }
    },
    [],
  );

  const refresh = useCallback(
    async (includeSettings = false) => {
      try {
        const [
          instanceItems,
          versionItems,
          accountItems,
          operationItems,
          statisticsOverview,
          applicationSettings,
        ] = await Promise.all([
          instancesApi.list(),
          versionsApi.list(),
          accountsApi.list(),
          operationsApi.list(),
          statisticsApi.overview(),
          includeSettings ? settingsApi.get() : Promise.resolve(undefined),
        ]);

        setInstances(instanceItems ?? []);
        setVersions(versionItems ?? []);
        setAccounts(accountItems ?? []);
        setOperations(operationItems ?? []);
        setStatistics(statisticsOverview);

        if (applicationSettings) {
          await changeAppLanguage(applicationSettings.language);

          setSettings(applicationSettings);

          if (applicationSettings.checkForUpdates) {
            void checkLauncherUpdate(
              applicationSettings.updateChannel,
              applicationSettings.skippedUpdateVersion,
            );
          }
        }

        setFatalError("");
      } catch (error) {
        setFatalError(errorMessage(error));
      } finally {
        setLoading(false);
      }
    },
    [checkLauncherUpdate],
  );

  useEffect(() => { void refresh(true); void updatesApi .currentVersion() .then(setLauncherVersion) .catch(() => undefined); void (async () => { try { const environment = await Environment(); setPlatform(environment.platform); } catch { setPlatform(""); } })(); const timer = window.setInterval(() => { void refresh(); }, 8_000); return () => { window.clearInterval(timer); }; }, [refresh]);

  const previousChannelRef = useRef<Settings["updateChannel"] | undefined>(undefined);

  useEffect(() => {
    if (!settings) {
      return;
    }

    const previous = previousChannelRef.current;
    const current = settings.updateChannel;

    previousChannelRef.current = current;

    if (previous === undefined || previous === current) {
      return;
    }

    setLauncherUpdate(undefined);

    void checkLauncherUpdate(current, settings.skippedUpdateVersion);
  }, [checkLauncherUpdate, settings]);

  useEffect(() => {
    try {
      return EventsOn("updates:progress", (progress: LauncherUpdateProgress) => {
        setUpdateProgress(progress);
      });
    } catch {
      return undefined;
    }
  }, []);

  function notify(message: string, type: ToastType = "ok") {
    setToast({ message, type });

    window.setTimeout(() => {
      setToast(undefined);
    }, 3_800);
  }

  const handleSettingsSaved = useCallback((saved: Settings) => {
    setSettings(saved);
  }, []);

  async function installLauncherUpdate() {
    if (!settings) {
      return;
    }

    setInstallingUpdate(true);

    setUpdateProgress({
      phase: "downloading",
      downloadedBytes: 0,
      totalBytes: launcherUpdate?.assetSize ?? 0,
      progress: 0,
    });

    try {
      await updatesApi.install(settings.updateChannel);
    } catch (error) {
      setInstallingUpdate(false);
      setUpdateProgress(undefined);
      notify(errorMessage(error), "error");
    }
  }

  async function skipLauncherUpdate() {
    if (!settings || !launcherUpdate) {
      return;
    }

    try {
      const saved = await settingsApi.update({
        ...settings,
        skippedUpdateVersion: launcherUpdate.version,
      });

      setSettings(saved);
      setLauncherUpdate(undefined);
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  async function openLauncherRelease() {
    try {
      await updatesApi.openReleasePage(settings?.updateChannel ?? "stable");
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  if (loading) {
    return (
      <div className="appLoading">
        <span className="spinner" />
      </div>
    );
  }

  return (
    <div className="shell">
      <Sidebar accounts={accounts} operations={operations} refresh={refresh} notify={notify} />

      <main>
        {fatalError && <ErrorBanner message={fatalError} onRetry={refresh} />}

        {launcherUpdate && (
          <UpdateNotice
            platform={platform}
            update={launcherUpdate}
            installingUpdate={installingUpdate}
            updateProgress={updateProgress}
            onInstall={() => {
              void installLauncherUpdate();
            }}
            onOpenRelease={() => {
              void openLauncherRelease();
            }}
            onSkip={() => {
              void skipLauncherUpdate();
            }}
            onDismiss={() => {
              setLauncherUpdate(undefined);
            }}
          />
        )}

        <Routes>
          <Route
            path="/library"
            element={
              <LibraryPage
                instances={instances}
                versions={versions}
                accounts={accounts}
                loading={loading}
                refresh={refresh}
                notify={notify}
              />
            }
          />

          <Route
            path="/mods/:modId"
            element={<ModDetailsPage instances={instances} versions={versions} notify={notify} />}
          />

          <Route
            path="/mods"
            element={<ModsPage instances={instances} versions={versions} notify={notify} />}
          />

          <Route
            path="/versions"
            element={<VersionsPage versions={versions} refresh={refresh} notify={notify} />}
          />

          <Route
            path="/operations"
            element={<OperationsPage operations={operations} refresh={refresh} notify={notify} />}
          />

          <Route
            path="/accounts"
            element={<AccountsPage accounts={accounts} refresh={refresh} notify={notify} />}
          />

          <Route
            path="/statistics"
            element={<StatisticsPage statistics={statistics} instances={instances} />}
          />

          <Route
            path="/settings"
            element={
              <SettingsPage
                settings={settings}
                notify={notify}
                onSaved={handleSettingsSaved}
                currentVersion={launcherVersion}
              />
            }
          />

          <Route path="*" element={<Navigate to="/library" replace />} />
        </Routes>
      </main>

      <AppToast
        toast={toast}
        onDismiss={() => {
          setToast(undefined);
        }}
      />
    </div>
  );
}
