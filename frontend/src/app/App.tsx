import { useCallback, useEffect, useState } from "react";
import { Navigate, NavLink, Route, Routes } from "react-router-dom";

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
import { AccountsPage } from "../pages/AccountsPage";
import { LibraryPage } from "../pages/LibraryPage";
import { OperationsPage } from "../pages/OperationsPage";
import { SettingsPage } from "../pages/SettingsPage";
import { StatisticsPage } from "../pages/StatisticsPage";
import { VersionsPage } from "../pages/VersionsPage";

const navigation = [
  { to: "/library", icon: "▦", label: "Library" },
  { to: "/versions", icon: "⬡", label: "Game versions" },
  { to: "/operations", icon: "⇣", label: "Operations" },
  { to: "/accounts", icon: "♙", label: "Accounts" },
  { to: "/statistics", icon: "◷", label: "Statistics" },
  { to: "/settings", icon: "⚙", label: "Settings" },
];

type ToastType = "ok" | "error";

export function App() {
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

  function notify(message: string, type: ToastType = "ok") {
    setToast({ message, type });
    window.setTimeout(() => setToast(undefined), 3_800);
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
            <span>LAUNCHER</span>
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
              <span>{item.label}</span>
              {item.to === "/operations" &&
                operations.some((operation) => operation.status === "running") && (
                  <b className="navPulse" />
                )}
            </NavLink>
          ))}
        </nav>

        <details className="accountSwitcher">
          <summary>
            <span className="miniAvatar">
              {(accounts.find((account) => account.isDefault)?.displayName ?? "?")
                .slice(0, 1)
                .toUpperCase()}
            </span>
            <span>
              <small>Аккаунт</small>
              <strong>
                {accounts.find((account) => account.isDefault)?.displayName ??
                  "Не выбран"}
              </strong>
            </span>
          </summary>
          <div className="accountSwitcherMenu">
            {accounts.map((account) => (
              <button
                key={account.id}
                className={account.isDefault ? "selected" : ""}
                onClick={async () => {
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
            <NavLink to="/accounts?add=1">＋ Добавить аккаунт</NavLink>
            <NavLink to="/accounts">Управление аккаунтами</NavLink>
          </div>
        </details>

        <div className="sidebarFoot">
          <div className="warmLine" />
          <span>Unofficial launcher</span>
          <small>for Vintage Story</small>
        </div>
      </aside>

      <main>
        {fatalError && (
          <div className="backendError">
            <span>!</span>
            <div>
              <strong>Could not connect to the core</strong>
              <p>{fatalError}</p>
            </div>
            <button onClick={() => void refresh()}>Retry</button>
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
            path="/versions"
            element={
              <VersionsPage
                versions={versions}
                refresh={refresh}
                notify={notify}
              />
            }
          />
          <Route
            path="/operations"
            element={<OperationsPage operations={operations} />}
          />
          <Route
            path="/accounts"
            element={
              <AccountsPage
                accounts={accounts}
                refresh={refresh}
                notify={notify}
              />
            }
          />
          <Route
            path="/statistics"
            element={
              <StatisticsPage
                statistics={statistics}
                instances={instances}
              />
            }
          />
          <Route
            path="/settings"
            element={
              <SettingsPage
                settings={settings}
                notify={notify}
                onSaved={setSettings}
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
