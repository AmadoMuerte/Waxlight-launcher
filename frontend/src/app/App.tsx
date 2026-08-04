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
  versionsApi,
  type Account,
  type GameVersion,
  type Instance,
  type Operation,
  type Settings,
  type Statistics,
} from "../shared/api";
import { errorMessage } from "../shared/api/bridge";

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
  const [instances, setInstances] = useState<Instance[]>([]);
  const [versions, setVersions] = useState<GameVersion[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [operations, setOperations] = useState<Operation[]>([]);
  const [statistics, setStatistics] = useState<Statistics>();
  const [settings, setSettings] = useState<Settings>();
  const [loading, setLoading] = useState(true);
  const [fatalError, setFatalError] = useState("");
  const [toast, setToast] = useState<{
    message: string;
    type: ToastType;
  }>();

  const refresh = useCallback(async () => {
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
        settingsApi.get(),
      ]);

      setInstances(instanceItems ?? []);
      setVersions(versionItems ?? []);
      setAccounts(accountItems ?? []);
      setOperations(operationItems ?? []);
      setStatistics(statisticsOverview);
      await changeAppLanguage(applicationSettings.language);
      setSettings(applicationSettings);
      setFatalError("");
    } catch (error) {
      setFatalError(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
    const timer = window.setInterval(() => void refresh(), 8_000);
    return () => window.clearInterval(timer);
  }, [refresh]);

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
            element={<SettingsPage settings={settings} notify={notify} onSaved={setSettings} />}
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
