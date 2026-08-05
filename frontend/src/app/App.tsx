import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, NavLink, Route, Routes } from "react-router-dom";

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
import { Button } from "../shared/ui";
import { EventsOn } from "../wailsjs/runtime/runtime";

const navigation = [
  { to: "/library", icon: "▦", labelKey: "library" },
  { to: "/mods", icon: "◇", labelKey: "mods" },
  { to: "/versions", icon: "⬡", labelKey: "game_versions" },
  { to: "/operations", icon: "⇣", labelKey: "operations" },
  { to: "/accounts", icon: "♙", labelKey: "accounts" },
  { to: "/statistics", icon: "◷", labelKey: "statistics" },
  { to: "/settings", icon: "⚙", labelKey: "settings" },
] as const;

type ToastType = "ok" | "error";

export function App() {
  const { t } = useTranslation();
  const accountSwitcherRef = useRef<HTMLDetailsElement>(null);
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

  useEffect(() => {
    void refresh(true);
    void updatesApi
      .currentVersion()
      .then(setLauncherVersion)
      .catch(() => undefined);
    const timer = window.setInterval(() => void refresh(), 8_000);
    return () => window.clearInterval(timer);
  }, [refresh]);

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

    // Changing the channel is an explicit request, so check it even when
    // automatic startup checks are disabled. Clear the old channel notice
    // immediately and ignore any older request that finishes later.
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

  useEffect(() => {
    function closeAccountSwitcher(event: PointerEvent) {
      const switcher = accountSwitcherRef.current;
      if (switcher?.open && event.target instanceof Node && !switcher.contains(event.target)) {
        switcher.open = false;
      }
    }
    window.addEventListener("pointerdown", closeAccountSwitcher);
    return () => window.removeEventListener("pointerdown", closeAccountSwitcher);
  }, []);

  function notify(message: string, type: ToastType = "ok") {
    setToast({ message, type });
    window.setTimeout(() => setToast(undefined), 3_800);
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
      <aside className="sidebar">
        <div className="brand">
          <div className="flame">
            <i />
          </div>
          <div>
            <strong>Waxlight</strong>
            <span>{t("launcher_uppercase")}</span>
          </div>
        </div>
        <nav>
          {navigation.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) => (isActive ? "active" : "")}
            >
              <i>{item.icon}</i>
              <span>{t(item.labelKey)}</span>
              {item.to === "/operations" &&
                operations.some((operation) => operation.status === "running") && (
                  <b className="navPulse" />
                )}
            </NavLink>
          ))}
        </nav>
        <details ref={accountSwitcherRef} className="accountSwitcher">
          <summary>
            <span className="miniAvatar">
              {(accounts.find((account) => account.isDefault)?.displayName ?? "?")
                .slice(0, 1)
                .toUpperCase()}
            </span>
            <span>
              <small>{t("account")}</small>
              <strong>
                {accounts.find((account) => account.isDefault)?.displayName ??
                  t("account_not_selected")}
              </strong>
            </span>
          </summary>
          <div className="accountSwitcherMenu">
            {accounts.map((account) => (
              <button
                key={account.id}
                className={account.isDefault ? "selected" : ""}
                onClick={async () => {
                  accountSwitcherRef.current?.removeAttribute("open");
                  try {
                    await accountsApi.setDefault(account.id);
                    await refresh();
                  } catch (error) {
                    notify(errorMessage(error), "error");
                  }
                }}
              >
                <span>{account.isDefault ? "✓" : ""}</span>
                <span>
                  <strong>{account.displayName}</strong>
                  <small>{account.email}</small>
                </span>
              </button>
            ))}
            <NavLink
              to="/accounts?add=1"
              onClick={() => accountSwitcherRef.current?.removeAttribute("open")}
            >
              {t("add_account")}
            </NavLink>
            <NavLink
              to="/accounts"
              onClick={() => accountSwitcherRef.current?.removeAttribute("open")}
            >
              {t("manage_accounts")}
            </NavLink>
          </div>
        </details>
        <div className="sidebarFoot">
          <div className="warmLine" />
          <span>{t("unofficial_launcher")}</span>
          <small>{t("for_vintage_story")}</small>
        </div>
      </aside>
      <main>
        {fatalError && (
          <div className="backendError">
            <span>!</span>
            <div>
              <strong>{t("could_not_connect_to_core")}</strong>
              <p>{fatalError}</p>
            </div>
            <button onClick={() => void refresh()}>{t("retry")}</button>
          </div>
        )}
        {launcherUpdate && (
          <section className="launcherUpdateNotice" aria-label={t("update_available")}>
            <div>
              <span className="eyebrow">
                {launcherUpdate.downgrade ? t("downgrade_available") : t("update_available")}
              </span>
              <strong>
                {t("launcher_update_versions", {
                  installed: launcherUpdate.installedVersion,
                  latest: launcherUpdate.version,
                })}
              </strong>
              <p>{launcherUpdate.releaseNotes || t("release_notes_unavailable")}</p>
              {launcherUpdate.installationMode === "portable" && (
                <p className="updateHint">{t("portable_update_hint")}</p>
              )}
              {installingUpdate && updateProgress && (
                <div className="launcherUpdateProgress">
                  <progress max={1} value={updateProgress.progress} />
                  <small>{t(`update_phase_${updateProgress.phase}`)}</small>
                </div>
              )}
            </div>
            <div className="launcherUpdateActions">
              {launcherUpdate.installationMode === "portable" ? (
                <>
                  <Button onClick={() => void openLauncherRelease()}>{t("download_update")}</Button>
                </>
              ) : (
                <Button busy={installingUpdate} onClick={() => void installLauncherUpdate()}>
                  {t("download_and_install_update")}
                </Button>
              )}
              <Button
                type="button"
                variant="secondary"
                disabled={installingUpdate}
                onClick={() => void openLauncherRelease()}
              >
                {t("view_full_release")}
              </Button>
              <Button
                type="button"
                variant="ghost"
                disabled={installingUpdate}
                onClick={() => setLauncherUpdate(undefined)}
              >
                {t("later")}
              </Button>
              <Button
                type="button"
                variant="ghost"
                disabled={installingUpdate}
                onClick={() => void skipLauncherUpdate()}
              >
                {t("skip_this_version")}
              </Button>
            </div>
          </section>
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
      {toast && (
        <div className={`toast ${toast.type}`}>
          <span>{toast.type === "ok" ? "✓" : "!"}</span>
          <span>{toast.message}</span>
        </div>
      )}
    </div>
  );
}
