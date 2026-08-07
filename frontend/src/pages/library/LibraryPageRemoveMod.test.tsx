// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { GameVersion } from "../../entities/game-version/model";
import type { InstalledMod } from "../../entities/mod/model";
import { LibraryPage } from "./LibraryPage";

const modsApi = vi.hoisted(() => ({
  list: vi.fn(),
  previewDelete: vi.fn(),
  remove: vi.fn(),
  checkInstanceUpdates: vi.fn(),
  linkLocal: vi.fn(),
  install: vi.fn(),
  installMany: vi.fn(),
  toggle: vi.fn(),
}));

const instancesApi = vi.hoisted(() => ({
  create: vi.fn(),
  clone: vi.fn(),
  list: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
}));
const versionsApi = vi.hoisted(() => ({ list: vi.fn() }));
const accountsApi = vi.hoisted(() => ({ list: vi.fn() }));

vi.mock("../../shared/api/instances", () => ({ instancesApi }));
vi.mock("../../shared/api/game-versions", () => ({ versionsApi }));
vi.mock("../../shared/api/accounts", () => ({ accountsApi }));
vi.mock("../../shared/api/launcher", () => ({ launcherApi: {} }));
vi.mock("../../shared/api/mods", () => ({ modsApi }));
vi.mock("../../shared/api/settings", () => ({ settingsApi: {} }));
vi.mock("../../shared/api/instance-package", () => ({ instancePackageApi: {} }));

const versions: GameVersion[] = [
  {
    id: "1.20",
    name: "1.20",
    channel: "stable",
    platform: "linux",
    architecture: "amd64",
    installationDir: "/game",
    executablePath: "/game/Vintagestory",
    status: "installed",
    sizeBytes: 1,
    installedAt: "2026-01-01T00:00:00Z",
  },
];

const instances = [
  {
    id: "inst-1",
    name: "Warm home",
    description: "A cozy base",
    gameVersionId: "1.20",
    directory: "/instances/inst-1",
    status: "ready",
    launchArguments: [],
    createdAt: "2026-01-01T00:00:00Z",
    enabledModCount: 0,
    totalModCount: 0,
    playtimeSeconds: 0,
  },
];

const rootMod: InstalledMod = {
  id: "mod-root",
  instanceId: "inst-1",
  name: "Root Mod",
  version: "1.0.0",
  fileName: "root.zip",
  filePath: "/instances/inst-1/Mods/root.zip",
  enabled: true,
  managed: true,
  source: "moddb:1:9",
  sizeBytes: 1,
  installedAt: "2026-01-01T00:00:00Z",
};

const libMod: InstalledMod = {
  id: "mod-lib",
  instanceId: "inst-1",
  name: "Lib Mod",
  version: "2.0.0",
  fileName: "lib.zip",
  filePath: "/instances/inst-1/Mods/lib.zip",
  enabled: true,
  managed: true,
  source: "moddb:2:5",
  sizeBytes: 1,
  installedAt: "2026-01-01T00:00:00Z",
};

async function renderPage() {
  instancesApi.list.mockResolvedValue(instances);
  versionsApi.list.mockResolvedValue(versions);
  accountsApi.list.mockResolvedValue([]);
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <LibraryPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  await screen.findByText("Warm home");
}

async function openModsTab() {
  await userEvent
    .setup()
    .click(screen.getAllByRole("button", { name: "Open instance details" })[0]);
  await userEvent.setup().click(screen.getByRole("tab", { name: /mods/i }));
}

describe("library mod removal", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    modsApi.list.mockResolvedValue([rootMod, libMod]);
    modsApi.linkLocal.mockResolvedValue({ linked: [], notMatched: [], failed: [] });
    modsApi.checkInstanceUpdates.mockResolvedValue({
      gameVersion: "1.20",
      mods: [],
      summary: {
        totalMods: 2,
        upToDate: 2,
        updatesAvailable: 0,
        notUpdatableLocal: 0,
        notUpdatableAbsent: 0,
        notUpdatableCatalogError: 0,
        incompatible: 0,
      },
    });
    modsApi.remove.mockResolvedValue(undefined);
  });

  it("asks before removing a mod with unused dependencies", async () => {
    modsApi.previewDelete.mockResolvedValue({
      modId: rootMod.id,
      modName: rootMod.name,
      dependencies: [libMod],
    });
    await renderPage();
    await openModsTab();

    await userEvent.setup().click(screen.getAllByRole("button", { name: "Remove" })[0]);

    const dialog = await screen.findByRole("dialog", {
      name: `Remove “${rootMod.name}” and its dependencies?`,
    });
    expect(within(dialog).getByText(libMod.name)).toBeTruthy();

    await userEvent.setup().click(screen.getByRole("button", { name: "Mod and dependencies" }));

    await waitFor(() => {
      expect(modsApi.remove).toHaveBeenCalledWith(rootMod.id, true);
    });
    expect(modsApi.previewDelete).toHaveBeenCalledWith(rootMod.id);
  });

  it("keeps only the mod when the user chooses to delete it alone", async () => {
    modsApi.previewDelete.mockResolvedValue({
      modId: rootMod.id,
      modName: rootMod.name,
      dependencies: [libMod],
    });
    await renderPage();
    await openModsTab();

    await userEvent.setup().click(screen.getAllByRole("button", { name: "Remove" })[0]);
    await userEvent.setup().click(screen.getByRole("button", { name: "Only the mod" }));

    await waitFor(() => {
      expect(modsApi.remove).toHaveBeenCalledWith(rootMod.id, false);
    });
  });

  it("confirms removal directly when there are no dependencies", async () => {
    modsApi.previewDelete.mockResolvedValue({
      modId: rootMod.id,
      modName: rootMod.name,
      dependencies: [],
    });
    await renderPage();
    await openModsTab();

    await userEvent.setup().click(screen.getAllByRole("button", { name: "Remove" })[0]);

    expect(await screen.findByText(`Remove mod “${rootMod.name}”?`)).toBeTruthy();
    await userEvent.setup().click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => {
      expect(modsApi.remove).toHaveBeenCalledWith(rootMod.id, false);
    });
  });
});
