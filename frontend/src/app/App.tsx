import { useEffect, useRef } from "react";
import { Navigate, Route, Routes } from "react-router";

import { useAccountsQuery } from "../entities/account/queries";
import { useGameVersionsQuery } from "../entities/game-version/queries";
import { useInstancesQuery } from "../entities/instance/queries";
import { useOperationsQuery } from "../entities/operation/queries";
import { useSettingsQuery } from "../entities/settings/queries";
import { useStatisticsQuery } from "../entities/statistics/queries";
import { AccountsPage } from "../pages/accounts/AccountsPage";
import { LibraryPage } from "../pages/library/LibraryPage";
import { ModDetailsPage } from "../pages/mod-details/ModDetailsPage";
import { ModsPage } from "../pages/mods/ModsPage";
import { OperationsPage } from "../pages/operations/OperationsPage";
import { SettingsPage } from "../pages/settings/SettingsPage";
import { StatisticsPage } from "../pages/statistics/StatisticsPage";
import { VersionsPage } from "../pages/versions/VersionsPage";
import { errorMessage } from "../shared/api/bridge";
import type { LauncherUpdateProgress, Settings } from "../shared/api/types";
import { updatesApi } from "../shared/api/updates";
import { changeAppLanguage } from "../shared/i18n";
import { Spinner } from "../shared/ui/spinner";
import { Environment, EventsOn } from "../wailsjs/runtime/runtime";
import { AppToast } from "../widgets/layout/AppToast";
import { ErrorBanner } from "../widgets/layout/ErrorBanner";
import { Sidebar } from "../widgets/layout/Sidebar";
import { UpdateDialog } from "../widgets/layout/UpdateDialog";
import { useAppShellStore } from "./stores/app-shell";

const POLL_INTERVAL = 8_000;

export function App() {
  const accountsQuery = useAccountsQuery({ refetchInterval: POLL_INTERVAL });
  const instancesQuery = useInstancesQuery({ refetchInterval: POLL_INTERVAL });
  const versionsQuery = useGameVersionsQuery({ refetchInterval: POLL_INTERVAL });
  const operationsQuery = useOperationsQuery({ refetchInterval: POLL_INTERVAL });
  const statisticsQuery = useStatisticsQuery({ refetchInterval: POLL_INTERVAL });
  const settingsQuery = useSettingsQuery();

  const watchers = [
    accountsQuery,
    instancesQuery,
    versionsQuery,
    operationsQuery,
    statisticsQuery,
    settingsQuery,
  ];
  const loading = watchers.some((query) => query.isPending);
  const firstError = watchers.find((query) => query.error);
  const fatalError = firstError?.error ? errorMessage(firstError.error) : "";

  const setFatalError = useAppShellStore((state) => state.setFatalError);
  const setPlatform = useAppShellStore((state) => state.setPlatform);
  const setLauncherVersion = useAppShellStore((state) => state.setLauncherVersion);
  const setUpdateProgress = useAppShellStore((state) => state.setUpdateProgress);
  const checkForUpdate = useAppShellStore((state) => state.checkForUpdate);

  const settings = settingsQuery.data;
  const updateCheckedOnceRef = useRef(false);
  const previousChannelRef = useRef<Settings["updateChannel"] | undefined>(undefined);

  useEffect(() => {
    setFatalError(fatalError);
  }, [fatalError, setFatalError]);

  useEffect(() => {
    if (!settings) {
      return;
    }
    void changeAppLanguage(settings.language);
  }, [settings]);

  useEffect(() => {
    if (!settings) {
      return;
    }
    if (settings.checkForUpdates && !updateCheckedOnceRef.current) {
      updateCheckedOnceRef.current = true;
      void checkForUpdate(settings.updateChannel, settings.skippedUpdateVersion);
    }
  }, [checkForUpdate, settings]);

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

    useAppShellStore.getState().dismissUpdate();

    void checkForUpdate(current, settings.skippedUpdateVersion);
  }, [checkForUpdate, settings]);

  useEffect(() => {
    void updatesApi
      .currentVersion()
      .then(setLauncherVersion)
      .catch(() => undefined);
    void (async () => {
      try {
        const environment = await Environment();
        setPlatform(environment.platform);
      } catch {
        setPlatform("");
      }
    })();
    try {
      return EventsOn("updates:progress", (progress: LauncherUpdateProgress) => {
        setUpdateProgress(progress);
      });
    } catch {
      return undefined;
    }
  }, [setLauncherVersion, setPlatform, setUpdateProgress]);

  if (loading) {
    return (
      <div className="appLoading">
        <Spinner />
      </div>
    );
  }

  return (
    <div className="shell">
      <Sidebar />

      <main>
        <ErrorBanner />
        <UpdateDialog />

        <Routes>
          <Route path="/library" element={<LibraryPage />} />
          <Route path="/mods/:modId" element={<ModDetailsPage />} />
          <Route path="/mods" element={<ModsPage />} />
          <Route path="/versions" element={<VersionsPage />} />
          <Route path="/operations" element={<OperationsPage />} />
          <Route path="/accounts" element={<AccountsPage />} />
          <Route path="/statistics" element={<StatisticsPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="*" element={<Navigate to="/library" replace />} />
        </Routes>
      </main>

      <AppToast />
    </div>
  );
}
