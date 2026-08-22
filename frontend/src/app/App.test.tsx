// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, expect, it, vi } from "vitest";

import i18n, { changeAppLanguage } from "../shared/i18n";
import { App } from "./App";
import { useAppShellStore } from "./stores/app-shell";
import { useNotificationStore } from "./stores/notifications";

const api = vi.hoisted(() => ({
  list: vi.fn().mockResolvedValue([]),
  overview: vi.fn().mockResolvedValue({
    totalPlaytimeSeconds: 0,
    launchCount: 0,
    averageSessionSeconds: 0,
    recentSessions: [],
  }),
  get: vi.fn().mockResolvedValue({
    language: "ru",
    downloadsParallel: 3,
    confirmDeletion: true,
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
vi.mock("../shared/api/deep-links", () => ({
  deepLinksApi: { consumePending: vi.fn().mockResolvedValue([]) },
}));
vi.mock("../shared/api/launcher", () => ({ launcherApi: {} }));
vi.mock("../shared/api/mods", () => ({ modsApi: {} }));
vi.mock("../shared/api/mod-catalog", () => ({ modCatalogApi: {} }));

vi.mock("@xterm/xterm", () => ({
  Terminal: class {
    write = vi.fn();
    clear = vi.fn();
    dispose = vi.fn();
    loadAddon = vi.fn();
    open = vi.fn();
    attachCustomKeyEventHandler = vi.fn();
    hasSelection = vi.fn(() => false);
    getSelection = vi.fn(() => "");
  },
}));

vi.mock("@xterm/addon-fit", () => ({
  FitAddon: class {
    fit = vi.fn();
  },
}));

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
  useAppShellStore.setState({
    launcherUpdate: undefined,
    updateDialogOpen: false,
    updateNotificationEnabled: false,
  });
  useNotificationStore.setState({ notifications: [] });
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

it("navigates between sidebar pages with mouse back and forward buttons", async () => {
  renderApp();

  const libraryLink = await screen.findByRole("link", { name: /Library|Библиотека/ });
  const settingsLink = screen.getByRole("link", { name: /Settings|Настройки/ });
  expect(document.querySelector('.sideNav a[href="/settings"]')).toBeNull();
  fireEvent.click(settingsLink);
  await waitFor(() => expect(settingsLink.className).toContain("active"));

  fireEvent.mouseDown(window, { button: 3 });
  await waitFor(() => expect(libraryLink.className).toContain("active"));

  fireEvent.mouseDown(window, { button: 4 });
  await waitFor(() => expect(settingsLink.className).toContain("active"));
});

it("supports browser navigation shortcuts sent by mouse drivers", async () => {
  renderApp();

  const libraryLink = await screen.findByRole("link", { name: /Library|Библиотека/ });
  const settingsLink = screen.getByRole("link", { name: /Settings|Настройки/ });
  fireEvent.click(settingsLink);
  await waitFor(() => expect(settingsLink.className).toContain("active"));

  fireEvent.keyDown(window, { key: "ArrowLeft", altKey: true });
  await waitFor(() => expect(libraryLink.className).toContain("active"));

  fireEvent.keyDown(window, { key: "ArrowRight", altKey: true });
  await waitFor(() => expect(settingsLink.className).toContain("active"));
});

it("exposes the UI Lab through development navigation", async () => {
  renderApp(["/dev/ui"]);

  expect(await screen.findByRole("heading", { level: 1, name: "Waxlight UI Lab" })).toBeTruthy();
  expect(screen.getByRole("link", { name: /UI Lab/ })).toBeTruthy();
});

it("navigates when the native window reports a side mouse button", async () => {
  const listeners = new Map<string, (payload: unknown) => void>();
  vi.stubGlobal("runtime", {
    EventsOnMultiple: (name: string, callback: (payload: unknown) => void) => {
      listeners.set(name, callback);
      return () => listeners.delete(name);
    },
  });
  renderApp();

  const libraryLink = await screen.findByRole("link", { name: /Library|Библиотека/ });
  const settingsLink = screen.getByRole("link", { name: /Settings|Настройки/ });
  fireEvent.click(settingsLink);
  await waitFor(() => expect(settingsLink.className).toContain("active"));

  act(() => listeners.get("navigation:mouse")?.(-1));
  await waitFor(() => expect(libraryLink.className).toContain("active"));
  vi.unstubAllGlobals();
});

it("publishes a startup update notification and opens the dialog only on selection", async () => {
  const user = userEvent.setup();
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

  const bell = await screen.findByRole("button", { name: "Уведомления" });
  expect(screen.queryByText(/0\.1\.4/)).toBeNull();
  expect(screen.getByLabelText("Непрочитанных уведомлений: 1")).toBeTruthy();

  await user.click(bell);
  const notification = await screen.findByRole("menuitem", { name: /Доступно обновление/ });
  expect(notification.textContent).toContain("Доступен Waxlight 0.1.5");
  await user.click(notification);

  expect(await screen.findByText(/0\.1\.4/)).toBeTruthy();
  expect(screen.getByText("0.1.5")).toBeTruthy();
  expect(screen.getByText("Security and compatibility fixes")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Напомнить позже" }));
  await waitFor(() => expect(screen.queryByText(/0\.1\.4/)).toBeNull());

  await user.click(bell);
  expect(await screen.findByText("Доступно обновление")).toBeTruthy();
  expect(screen.queryByLabelText("Непрочитанных уведомлений: 1")).toBeNull();
});

it("opens manual update checks directly without publishing a notification", async () => {
  const user = userEvent.setup();
  api.get.mockResolvedValueOnce({
    language: "ru",
    downloadsParallel: 3,
    confirmDeletion: true,
    globalLaunchArguments: [],
    checkForUpdates: false,
    updateChannel: "stable",
    skippedUpdateVersion: "0.1.5",
  });
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
  renderApp(["/settings"]);

  await user.click(await screen.findByRole("button", { name: "Проверить обновления" }));

  expect(await screen.findByText("Security and compatibility fixes")).toBeTruthy();
  expect(useNotificationStore.getState().notifications).toEqual([]);
  expect(screen.queryByLabelText("Непрочитанных уведомлений: 1")).toBeNull();
});

it("updates the operations list live from backend events", async () => {
  const listeners = new Map<string, (payload: unknown) => void>();
  vi.stubGlobal("runtime", {
    EventsOnMultiple: (name: string, callback: (payload: unknown) => void) => {
      listeners.set(name, callback);
      return () => {
        listeners.delete(name);
      };
    },
  });
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  );

  renderApp(["/operations"]);
  await screen.findByText("Операций пока нет");

  const operation = {
    id: "op-1",
    type: "snapshot_create",
    title: "Creating snapshot",
    titleKey: "operation_creating_snapshot",
    status: "running",
    progress: 0.25,
    currentBytes: 25,
    totalBytes: 100,
    bytesPerSecond: 0,
    createdAt: "2026-08-08T00:00:00Z",
  };

  const created = listeners.get("operation:created");
  expect(created).toBeTruthy();
  act(() => created!(operation));
  expect(await screen.findByText("Создание снимка")).toBeTruthy();

  const progress = listeners.get("operation:progress");
  expect(progress).toBeTruthy();
  act(() =>
    progress!({
      ...operation,
      progress: 0.75,
      currentBytes: 75,
    }),
  );
  await waitFor(() => {
    const bar = document.querySelector(".progressIndicator") as HTMLElement | null;
    expect(bar?.style.width).toBe("75%");
  });

  const removed = listeners.get("operation:removed");
  expect(removed).toBeTruthy();
  act(() => removed!({ id: "op-1" }));
  await waitFor(() => expect(screen.queryByText("Создание снимка")).toBeNull());
  vi.unstubAllGlobals();
});
