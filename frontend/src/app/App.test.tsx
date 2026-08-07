// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, it, vi } from "vitest";

import i18n, { changeAppLanguage } from "../shared/i18n";
import { App } from "./App";

const api = vi.hoisted(() => ({
  list: vi.fn().mockResolvedValue([]),
  overview: vi.fn().mockResolvedValue({
    totalPlaytimeSeconds: 0,
    launchCount: 0,
    averageSessionSeconds: 0,
    recentSessions: [],
  }),
  get: vi.fn().mockResolvedValue({
    theme: "dark",
    language: "ru",
    downloadsParallel: 3,
    confirmDeletion: true,
    minSessionDurationSec: 10,
    globalLaunchArguments: [],
    checkForUpdates: true,
    updateChannel: "stable",
    skippedUpdateVersion: "",
  }),
  update: vi.fn().mockImplementation(async (settings) => settings),
  getDataFolder: vi.fn().mockResolvedValue({
    currentPath: "/data",
    defaultPath: "/data",
    lastError: "",
  }),
  checkUpdate: vi.fn().mockResolvedValue({
    installedVersion: "0.1.4",
    version: "0.1.4",
    available: false,
    prerelease: false,
    releaseNotes: "",
    releasePageUrl: "",
    assetName: "",
    assetSize: 0,
  }),
}));

vi.mock("../shared/api/instances", () => ({
  instancesApi: { list: api.list },
}));
vi.mock("../shared/api/game-versions", () => ({
  versionsApi: { list: api.list },
}));
vi.mock("../shared/api/accounts", () => ({
  accountsApi: { list: api.list },
}));
vi.mock("../shared/api/operations", () => ({
  operationsApi: { list: api.list },
}));
vi.mock("../shared/api/statistics", () => ({
  statisticsApi: { overview: api.overview },
}));
vi.mock("../shared/api/settings", () => ({
  settingsApi: { get: api.get, update: api.update, getDataFolder: api.getDataFolder },
}));
vi.mock("../shared/api/updates", () => ({
  updatesApi: {
    currentVersion: vi.fn().mockResolvedValue("0.1.4"),
    check: api.checkUpdate,
    install: vi.fn(),
    openReleasePage: vi.fn(),
  },
}));
vi.mock("../shared/api/launcher", () => ({ launcherApi: {} }));
vi.mock("../shared/api/mods", () => ({ modsApi: {} }));
vi.mock("../shared/api/mod-catalog", () => ({ modCatalogApi: {} }));

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  api.checkUpdate.mockResolvedValue({
    installedVersion: "0.1.4",
    version: "0.1.4",
    available: false,
    prerelease: false,
    releaseNotes: "",
    releasePageUrl: "",
    assetName: "",
    assetSize: 0,
  });
});

function renderApp(initialEntries?: string[]) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={initialEntries}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

it("applies persisted language before rendering and navigation reacts to changes", async () => {
  api.get.mockClear();
  api.update.mockClear();
  vi.useFakeTimers({ shouldAdvanceTime: true });
  renderApp();
  expect(await screen.findByRole("link", { name: /Библиотека/ })).toBeTruthy();
  expect(document.documentElement.lang).toBe("ru");
  expect(api.get).toHaveBeenCalledTimes(1);

  await vi.advanceTimersByTimeAsync(16_000);
  expect(api.get).toHaveBeenCalledTimes(1);

  await changeAppLanguage("en");
  await waitFor(() => expect(screen.getByRole("link", { name: /Library/ })).toBeTruthy());
  expect(i18n.resolvedLanguage).toBe("en");
});

it("does not replace autosaved settings during background refresh", async () => {
  api.get.mockClear();
  api.update.mockClear();
  vi.useFakeTimers({ shouldAdvanceTime: true });
  renderApp(["/settings"]);

  const parallelDownloads = (await screen.findAllByRole("spinbutton"))[0] as HTMLInputElement;
  fireEvent.change(parallelDownloads, { target: { value: "7" } });
  expect(parallelDownloads.value).toBe("7");

  await vi.advanceTimersByTimeAsync(16_000);
  expect(parallelDownloads.value).toBe("7");
  expect(api.get).toHaveBeenCalledTimes(1);
  expect(api.update).toHaveBeenCalledWith(expect.objectContaining({ downloadsParallel: 7 }));
});

it("shows a non-intrusive startup update notice that can be postponed", async () => {
  api.checkUpdate.mockResolvedValueOnce({
    installedVersion: "0.1.4",
    version: "0.1.5",
    available: true,
    prerelease: false,
    releaseNotes: "Security and compatibility fixes",
    releasePageUrl: "https://github.com/AmadoMuerte/Waxlight-launcher/releases/tag/v0.1.5",
    assetName: "Waxlight-Launcher-v0.1.5-linux-amd64.tar.gz",
    assetSize: 1024,
  });
  renderApp();

  expect(await screen.findByText(/0\.1\.4.*0\.1\.5/)).toBeTruthy();
  expect(screen.getByText("Security and compatibility fixes")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Позже" }));
  await waitFor(() => expect(screen.queryByText(/0\.1\.4.*0\.1\.5/)).toBeNull());
});
