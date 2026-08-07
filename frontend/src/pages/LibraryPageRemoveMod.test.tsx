// @vitest-environment jsdom

import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { GameVersion, InstalledMod } from "../shared/api";
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

vi.mock("../shared/api", () => ({
  instancesApi: {
    create: vi.fn(),
    clone: vi.fn(),
    list: vi.fn(),
    update: vi.fn(),
    remove: vi.fn(),
  },
  launcherApi: {},
  modsApi,
  settingsApi: {},
  instancePackageApi: {},
}));

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

function renderPage() {
  return render(
    <MemoryRouter>
      <LibraryPage
        instances={instances}
        versions={versions}
        accounts={[]}
        loading={false}
        refresh={vi.fn().mockResolvedValue(undefined)}
        notify={vi.fn()}
      />
    </MemoryRouter>,
  );
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
    renderPage();
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
    renderPage();
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
    renderPage();
    await openModsTab();

    await userEvent.setup().click(screen.getAllByRole("button", { name: "Remove" })[0]);

    expect(await screen.findByText(`Remove mod “${rootMod.name}”?`)).toBeTruthy();
    await userEvent.setup().click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => {
      expect(modsApi.remove).toHaveBeenCalledWith(rootMod.id, false);
    });
  });
});
