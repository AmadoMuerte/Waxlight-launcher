// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { useAppShellStore } from "../../app/stores/app-shell";
import { SETTINGS_QUERY_KEY } from "../../shared/api/keys";
import type { LauncherUpdate } from "../../shared/api/types";
import { UpdateDialog } from "./UpdateDialog";

const api = vi.hoisted(() => ({
  settings: {
    theme: "dark",
    language: "en",
    downloadsParallel: 3,
    confirmDeletion: true,
    minSessionDurationSec: 10,
    globalLaunchArguments: [],
    checkForUpdates: true,
    updateChannel: "stable",
    skippedUpdateVersion: "",
  },
  update: vi.fn().mockImplementation(async (next: unknown) => next),
  currentVersion: vi.fn().mockResolvedValue("0.2.1"),
  check: vi.fn(),
  install: vi.fn(),
  openReleasePage: vi.fn(),
  openUrl: vi.fn(),
}));

const update: LauncherUpdate = {
  installedVersion: "0.2.1",
  version: "0.3.0",
  available: true,
  downgrade: false,
  prerelease: false,
  releaseNotes:
    "### What's new\n\n* Added support for custom game data locations, thanks to @AmadoMuerte.\n* Improved launcher update system.\n* See [release notes](https://github.com/AmadoMuerte/Waxlight-launcher/releases/tag/v0.3.0) for details.\n\n### Bug fixes\n\n* Fixed update issues on Linux.",
  releasePageUrl: "https://github.com/AmadoMuerte/Waxlight-launcher/releases/tag/v0.3.0",
  assetName: "Waxlight-Launcher-v0.3.0-linux-amd64.tar.gz",
  assetSize: 26_214_400,
  installationMode: "installed",
};

vi.mock("../../entities/settings/api", () => ({
  settingsApi: {
    get: vi.fn().mockResolvedValue(api.settings),
    update: api.update,
  },
}));

vi.mock("../../shared/api/settings", () => ({
  settingsApi: {
    get: vi.fn().mockResolvedValue(api.settings),
    update: api.update,
  },
}));

vi.mock("../../shared/api/updates", () => ({
  updatesApi: {
    currentVersion: api.currentVersion,
    check: api.check,
    install: api.install,
    openReleasePage: api.openReleasePage,
    openUrl: api.openUrl,
  },
}));

function renderDialog() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  queryClient.setQueryData(SETTINGS_QUERY_KEY, api.settings);
  render(
    <QueryClientProvider client={queryClient}>
      <UpdateDialog />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  useAppShellStore.setState({
    platform: "linux",
    launcherUpdate: update,
    installingUpdate: false,
    updateProgress: undefined,
  });
});

afterEach(() => {
  cleanup();
  useAppShellStore.setState({ launcherUpdate: undefined, installingUpdate: false });
});

it("renders version transition and markdown changelog", async () => {
  renderDialog();

  expect(await screen.findByText(/Current version: 0\.2\.1/)).toBeTruthy();
  expect(screen.getByText("0.3.0")).toBeTruthy();
  expect(screen.getByRole("heading", { name: "What's new" })).toBeTruthy();
  expect(screen.getByText(/Added support for custom game data locations, thanks to/)).toBeTruthy();
  expect(screen.getByText("@AmadoMuerte").className).toBe("markdownMention");
  expect(screen.getByRole("heading", { name: "Bug fixes" })).toBeTruthy();
  expect(screen.getByText("Version 0.3.0 • 25.0 MB")).toBeTruthy();
  expect(screen.getByRole("button", { name: "GitHub Release" })).toBeTruthy();
});

it("closes the dialog and keeps notifications on remind me later", async () => {
  renderDialog();
  fireEvent.click(await screen.findByRole("button", { name: "Remind me later" }));

  await waitFor(() => expect(useAppShellStore.getState().launcherUpdate).toBeUndefined());
});

it("persists only the offered version when don't remind me is clicked", async () => {
  renderDialog();
  fireEvent.click(await screen.findByRole("button", { name: "Don't remind me" }));

  await waitFor(() =>
    expect(api.update).toHaveBeenCalledWith(
      expect.objectContaining({ skippedUpdateVersion: "0.3.0" }),
    ),
  );
  await waitFor(() => expect(useAppShellStore.getState().launcherUpdate).toBeUndefined());
});

it("opens changelog links in the user's browser", async () => {
  renderDialog();

  const link = await screen.findByRole("link", { name: "release notes" });
  fireEvent.click(link);

  await waitFor(() =>
    expect(api.openUrl).toHaveBeenCalledWith(
      "https://github.com/AmadoMuerte/Waxlight-launcher/releases/tag/v0.3.0",
    ),
  );
});

it("shows download progress and hides the actions while installing", async () => {
  useAppShellStore.setState({
    installingUpdate: true,
    updateProgress: {
      phase: "downloading",
      downloadedBytes: 1_048_576,
      totalBytes: 26_214_400,
      progress: 0.04,
    },
  });
  renderDialog();

  expect(await screen.findByText("Downloading update…")).toBeTruthy();
  expect(screen.getByText(/1\.0 MB \/ 25\.0 MB/)).toBeTruthy();
  expect(screen.queryByRole("button", { name: /Remind me later/ })).toBeNull();
  expect(screen.queryByRole("button", { name: /Don't remind me/ })).toBeNull();
});
