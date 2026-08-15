import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { Navigate, Route, Routes, useNavigate } from "react-router";

import { useAccountsQuery } from "../entities/account/queries";
import { useGameVersionsQuery } from "../entities/game-version/queries";
import { useInstancesQuery } from "../entities/instance/queries";
import type { RecoverySuggestion } from "../entities/last-known-good/model";
import { useOperationsQuery } from "../entities/operation/queries";
import { useSettingsQuery } from "../entities/settings/queries";
import { useStatisticsQuery } from "../entities/statistics/queries";
import { RecoveryDialog } from "../features/recovery/RecoveryDialog";
import { AccountsPage } from "../pages/accounts/AccountsPage";
import { LibraryPage } from "../pages/library/LibraryPage";
import { ModDetailsPage } from "../pages/mod-details/ModDetailsPage";
import { ModsPage } from "../pages/mods/ModsPage";
import { OperationsPage } from "../pages/operations/OperationsPage";
import { ServersPage } from "../pages/servers/ServersPage";
import { SettingsPage } from "../pages/settings/SettingsPage";
import { StatisticsPage } from "../pages/statistics/StatisticsPage";
import { VersionsPage } from "../pages/versions/VersionsPage";
import { errorMessage } from "../shared/api/bridge";
import { deepLinksApi, type DeepLinkTarget } from "../shared/api/deep-links";
import { OPERATIONS_QUERY_KEY } from "../shared/api/keys";
import type { LauncherUpdateProgress, Operation, Settings } from "../shared/api/types";
import { updatesApi } from "../shared/api/updates";
import { changeAppLanguage } from "../shared/i18n";
import { log } from "../shared/lib/logger";
import { deepLinkPath, normalizeServerAddress } from "../shared/lib/waxlight-links";
import { Spinner } from "../shared/ui/spinner";
import { Environment, EventsOn } from "../wailsjs/runtime/runtime";
import { AppToast } from "../widgets/layout/AppToast";
import { ErrorBanner } from "../widgets/layout/ErrorBanner";
import { Sidebar } from "../widgets/layout/Sidebar";
import { UpdateDialog } from "../widgets/layout/UpdateDialog";
import { useAppShellStore } from "./stores/app-shell";
import { useRecoveryStore } from "./stores/recovery";

const POLL_INTERVAL = 8_000;

export function App() {
  const navigate = useNavigate();
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
  const queryClient = useQueryClient();

  const settings = settingsQuery.data;
  const updateCheckedOnceRef = useRef(false);
  const previousChannelRef = useRef<Settings["updateChannel"] | undefined>(undefined);

  useEffect(() => {
    const open = (target: unknown) => {
      const path = deepLinkPath(target);
      const address =
        target && typeof target === "object" && Reflect.get(target, "type") === "server"
          ? normalizeServerAddress(Reflect.get(target, "address"))
          : undefined;
      if (path) void navigate(path, { state: address ? { deepLinkAddress: address } : undefined });
    };

    let unsubscribe: (() => void) | undefined;
    try {
      unsubscribe = EventsOn("deeplink:open", open);
      void deepLinksApi
        .consumePending()
        .then((targets: DeepLinkTarget[]) => targets.forEach(open))
        .catch((error) => log.warn(errorMessage(error), { source: "deep-link" }));
    } catch (error) {
      log.warn(errorMessage(error), { source: "deep-link" });
    }
    return () => unsubscribe?.();
  }, [navigate]);

  useEffect(() => {
    const navigateWithMouseButton = (event: MouseEvent) => {
      const delta = event.button === 3 ? -1 : event.button === 4 ? 1 : 0;
      if (delta === 0) {
        return;
      }

      event.preventDefault();
      event.stopImmediatePropagation();
      void navigate(delta);
    };
    const navigateWithKeyboard = (event: KeyboardEvent) => {
      if (!event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) {
        return;
      }

      const delta = event.key === "ArrowLeft" ? -1 : event.key === "ArrowRight" ? 1 : 0;
      if (delta === 0) {
        return;
      }

      event.preventDefault();
      void navigate(delta);
    };

    window.addEventListener("mousedown", navigateWithMouseButton, true);
    window.addEventListener("keydown", navigateWithKeyboard, true);
    let unsubscribeNative: (() => void) | undefined;
    try {
      unsubscribeNative = EventsOn("navigation:mouse", (direction: number) => {
        if (direction === -1 || direction === 1) {
          void navigate(direction);
        }
      });
    } catch (error) {
      log.warn(errorMessage(error), { source: "mouse-navigation" });
    }
    return () => {
      window.removeEventListener("mousedown", navigateWithMouseButton, true);
      window.removeEventListener("keydown", navigateWithKeyboard, true);
      unsubscribeNative?.();
    };
  }, [navigate]);

  useEffect(() => {
    setFatalError(fatalError);
    if (fatalError) {
      log.warn(fatalError, { source: "watcher" });
    }
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
      .catch((error) => log.warn(errorMessage(error), { source: "currentVersion" }));
    void (async () => {
      try {
        const environment = await Environment();
        setPlatform(environment.platform);
      } catch (error) {
        log.warn(errorMessage(error), { source: "environment" });
        setPlatform("");
      }
    })();
    try {
      return EventsOn("updates:progress", (progress: LauncherUpdateProgress) => {
        setUpdateProgress(progress);
      });
    } catch (error) {
      log.warn(errorMessage(error), { source: "events" });
      return undefined;
    }
  }, [setLauncherVersion, setPlatform, setUpdateProgress]);

  // Live operation updates: the backend publishes every progress change as an
  // event, so the Operations page and the sidebar badge react immediately
  // instead of waiting for the 8-second polling cycle.
  useEffect(() => {
    const applyOperation = (operation: Operation) => {
      queryClient.setQueryData<Operation[]>(OPERATIONS_QUERY_KEY, (current) => {
        const list = current ?? [];
        const index = list.findIndex((item) => item.id === operation.id);
        if (index >= 0) {
          const next = [...list];
          next[index] = { ...next[index], ...operation };
          return next;
        }
        return [operation, ...list];
      });
    };
    const removeOperation = (payload: { id?: string }) => {
      if (!payload?.id) {
        return;
      }
      queryClient.setQueryData<Operation[]>(OPERATIONS_QUERY_KEY, (current) =>
        (current ?? []).filter((item) => item.id !== payload.id),
      );
    };
    const listeners: Array<() => void> = [];
    try {
      for (const name of [
        "operation:created",
        "operation:updated",
        "operation:progress",
        "operation:completed",
        "operation:failed",
      ]) {
        listeners.push(EventsOn(name, applyOperation));
      }
      listeners.push(EventsOn("operation:removed", removeOperation));
      // The backend publishes a recovery suggestion after a failed startup.
      // The dialog decides nothing itself; it renders the suggestion and asks
      // the user whether to restore the last known working state.
      listeners.push(
        EventsOn("game:recovery-suggestion", (suggestion: RecoverySuggestion) => {
          useRecoveryStore.getState().show(suggestion);
        }),
      );
      // A Last Known Good marker was recorded or replaced.
      listeners.push(
        EventsOn("last-known-good:updated", () => {
          void queryClient.invalidateQueries({ queryKey: ["last-known-good"] });
        }),
      );
    } catch (error) {
      log.warn(errorMessage(error), { source: "operation-events" });
    }
    return () => {
      for (const unsubscribe of listeners) {
        unsubscribe();
      }
    };
  }, [queryClient]);

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
        <RecoveryDialog />

        <Routes>
          <Route path="/library" element={<LibraryPage />} />
          <Route path="/mods/:modId" element={<ModDetailsPage />} />
          <Route path="/mods" element={<ModsPage />} />
          <Route path="/versions" element={<VersionsPage />} />
          <Route path="/servers" element={<ServersPage />} />
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
