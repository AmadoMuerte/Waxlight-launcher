// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { useToastStore } from "../../app/stores/toast";
import type { Settings } from "../../entities/settings/model";
import { SettingsPage } from "./SettingsPage";

const settings: Settings = {
  language: "en",
  downloadsParallel: 3,
  confirmDeletion: true,
  globalLaunchArguments: [],
  checkForUpdates: true,
  updateChannel: "stable",
  skippedUpdateVersion: "0.2.0",
  telemetryEnabled: true,
  automaticSafetySnapshots: true,
};

const settingsQuery = vi.hoisted(() => ({ useSettingsQuery: vi.fn() }));

const settingsApi = vi.hoisted(() => ({
  get: vi.fn(),
  update: vi.fn(),
  getDataFolder: vi.fn(),
  selectDataFolder: vi.fn(),
  moveDataFolder: vi.fn(),
}));

interface AppShellState {
  launcherVersion: string;
  launcherUpdate: unknown;
  checkForUpdate: ReturnType<typeof vi.fn>;
}

const appShell = vi.hoisted(() => {
  const state: AppShellState = {
    launcherVersion: "0.2.1",
    launcherUpdate: undefined,
    checkForUpdate: vi.fn().mockResolvedValue(undefined),
  };
  const useAppShellStore = vi.fn((selector: (state: AppShellState) => unknown) => selector(state));
  (useAppShellStore as unknown as { getState: () => AppShellState }).getState = vi.fn(() => state);
  return { useAppShellStore, state };
});

vi.mock("../../entities/settings/queries", () => settingsQuery);

vi.mock("../../entities/settings/api", () => ({
  settingsApi,
}));

vi.mock("../../app/stores/app-shell", () => appShell);

vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: () => () => undefined,
}));

vi.mock("../../shared/i18n", () => ({
  changeAppLanguage: vi.fn().mockResolvedValue("en"),
  normalizeLanguage: (language: string) => language,
  supportedLanguages: [{ code: "en", nativeName: "English" }],
}));

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <SettingsPage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  settingsQuery.useSettingsQuery.mockReturnValue({ data: settings });
  settingsApi.get.mockResolvedValue(settings);
  settingsApi.update.mockImplementation(async (next: unknown) => next);
  settingsApi.getDataFolder.mockResolvedValue({
    currentPath: "/data",
    defaultPath: "/data",
    lastError: "",
  });
  settingsApi.selectDataFolder.mockResolvedValue("");
  settingsApi.moveDataFolder.mockResolvedValue(undefined);
  appShell.state.launcherUpdate = undefined;
  appShell.state.checkForUpdate.mockReset();
  appShell.state.checkForUpdate.mockResolvedValue(undefined);
  useToastStore.setState({ notify: vi.fn() });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

it("calls checkForUpdate with the update channel and skipped version", async () => {
  renderPage();

  const button = await screen.findByRole("button", { name: "Check for updates" });
  fireEvent.click(button);

  await waitFor(() =>
    expect(appShell.state.checkForUpdate).toHaveBeenCalledWith(
      settings.updateChannel,
      settings.skippedUpdateVersion,
    ),
  );
});

it("persists the automatic safety backups switch", async () => {
  renderPage();

  const toggle = await screen.findByRole("switch", { name: "Automatic safety backups" });
  expect(toggle.getAttribute("aria-checked")).toBe("true");
  fireEvent.click(toggle);

  await waitFor(() =>
    expect(settingsApi.update).toHaveBeenCalledWith(
      expect.objectContaining({ automaticSafetySnapshots: false }),
    ),
  );
});
