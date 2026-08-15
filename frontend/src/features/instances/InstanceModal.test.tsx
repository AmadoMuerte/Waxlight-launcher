// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useToastStore } from "../../app/stores/toast";
import type { Instance } from "../../entities/instance/model";
import { InstanceModal } from "./InstanceModal";

const instancesApi = vi.hoisted(() => ({
  list: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
  selectCover: vi.fn(),
}));

const modsApi = vi.hoisted(() => ({
  list: vi.fn(),
  checkInstanceUpdates: vi.fn(),
  linkLocal: vi.fn(),
  remove: vi.fn(),
  previewDelete: vi.fn(),
  installMany: vi.fn(),
  toggle: vi.fn(),
  updateInstance: vi.fn(),
}));

const modCatalogApi = vi.hoisted(() => ({
  get: vi.fn(),
}));

const settingsApi = vi.hoisted(() => ({
  selectModFiles: vi.fn(),
  openDirectory: vi.fn(),
}));

const settingsQuery = vi.hoisted(() => ({
  useSettingsQuery: vi.fn(),
  useOptimumStatusQuery: vi.fn(),
}));

vi.mock("../../shared/api/instances", () => ({ instancesApi }));
vi.mock("../../shared/api/mods", () => ({ modsApi }));
vi.mock("../../shared/api/mod-catalog", () => ({ modCatalogApi }));
vi.mock("../../shared/api/settings", () => ({ settingsApi }));
vi.mock("../../entities/settings/queries", () => settingsQuery);

const instance: Instance = {
  id: "instance-1",
  name: "Survival",
  description: "",
  gameVersionId: "1.20",
  gameClient: "vanilla",
  directory: "/data",
  status: "ready",
  launchArguments: [],
  createdAt: "2026-01-01T00:00:00Z",
  enabledModCount: 0,
  totalModCount: 0,
  playtimeSeconds: 0,
};

const version = {
  id: "1.20",
  name: "1.20",
  channel: "stable",
  platform: "linux",
  architecture: "amd64",
  installationDir: "/game",
  executablePath: "/game/Vintagestory",
  status: "installed",
  sizeBytes: 100,
  installedAt: "2026-01-01T00:00:00Z",
};

const account = {
  id: "account-1",
  username: "Waxlighter",
  displayName: "Waxlighter",
  email: "player@example.com",
  status: "valid",
  isDefault: true,
};

const installedMod = {
  id: "mod-1",
  name: "Player Corpse",
  version: "2.0.0",
  fileName: "playercorpse.zip",
  enabled: true,
  managed: true,
  source: "moddb:playercorpse:7",
};

function renderModal() {
  const notify = vi.fn();
  const onModUpdatesChanged = vi.fn();
  useToastStore.setState({ notify });
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <InstanceModal
          instance={instance}
          versions={[version]}
          accounts={[account]}
          onClose={vi.fn()}
          onExport={vi.fn()}
          onClone={vi.fn()}
          onModUpdatesChanged={onModUpdatesChanged}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { notify, onModUpdatesChanged };
}

async function openSettingsTab() {
  const user = userEvent.setup();
  await user.click(screen.getByRole("tab", { name: "Settings" }));
  return user;
}

async function openModsTab() {
  const user = userEvent.setup();
  await user.click(screen.getByRole("tab", { name: /Mods/ }));
  return user;
}

describe("confirmDeletion gate", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  beforeEach(() => {
    vi.clearAllMocks();
    settingsQuery.useSettingsQuery.mockReturnValue({ data: undefined });
    settingsQuery.useOptimumStatusQuery.mockReturnValue({ data: undefined });
    vi.stubGlobal(
      "ResizeObserver",
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      },
    );
    vi.stubGlobal("runtime", {
      EventsOn: () => () => undefined,
      EventsOnMultiple: () => () => undefined,
      EventsEmit: () => undefined,
    });
    instancesApi.remove.mockResolvedValue(undefined);
    instancesApi.update.mockResolvedValue(instance);
    instancesApi.selectCover.mockResolvedValue("/pictures/cover.png");
    modsApi.list.mockResolvedValue([installedMod]);
    modsApi.checkInstanceUpdates.mockResolvedValue({
      gameVersion: "1.20",
      summary: { updatesAvailable: 0, notUpdatableLocal: 0, notUpdatableAbsent: 0 },
      mods: [],
    });
    modsApi.linkLocal.mockResolvedValue({ linked: [], notMatched: [] });
    modsApi.remove.mockResolvedValue(undefined);
    modsApi.previewDelete.mockResolvedValue({ dependencies: [] });
    modsApi.updateInstance.mockResolvedValue({ updated: 1 });
    modCatalogApi.get.mockResolvedValue({
      versions: [
        {
          id: "7",
          version: "2.0.0",
          gameVersions: ["1.20"],
          releaseType: "stable",
          fileName: "playercorpse.zip",
          fileSize: 1,
        },
        {
          id: "8",
          version: "2.1.0",
          gameVersions: ["1.20"],
          releaseType: "stable",
          fileName: "playercorpse.zip",
          fileSize: 1,
        },
      ],
    });
    settingsApi.selectModFiles.mockResolvedValue([]);
    settingsApi.openDirectory.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("deletes an instance directly when confirmDeletion is false", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: { confirmDeletion: false } });
    renderModal();
    const user = await openSettingsTab();

    await user.click(screen.getByRole("button", { name: "Delete instance" }));

    await waitFor(() => expect(instancesApi.remove).toHaveBeenCalledWith("instance-1", true));
    expect(screen.queryByText(/Delete “Survival”/)).toBeNull();
  });

  it("shows a confirm dialog before deleting an instance when confirmDeletion is true", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: { confirmDeletion: true } });
    renderModal();
    const user = await openSettingsTab();

    await user.click(screen.getByRole("button", { name: "Delete instance" }));
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(instancesApi.remove).not.toHaveBeenCalled();

    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(instancesApi.remove).toHaveBeenCalledWith("instance-1", true));
  });

  it("shows a confirm dialog when settings are still loading", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: undefined });
    renderModal();
    const user = await openSettingsTab();

    await user.click(screen.getByRole("button", { name: "Delete instance" }));
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(instancesApi.remove).not.toHaveBeenCalled();
  });

  it("removes a mod directly when confirmDeletion is false and it has no dependencies", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: { confirmDeletion: false } });
    renderModal();
    const user = await openModsTab();
    await screen.findByText("Player Corpse");

    await user.click(screen.getByRole("button", { name: "Remove" }));

    await waitFor(() => expect(modsApi.remove).toHaveBeenCalledWith("mod-1", false));
    expect(screen.queryByText(/Remove mod “Player Corpse”/)).toBeNull();
  });

  it("installs a selected catalog version from the instance mods tab", async () => {
    const { onModUpdatesChanged } = renderModal();
    const user = await openModsTab();

    await user.click(await screen.findByRole("combobox", { name: "Update to Player Corpse" }));
    await user.click(await screen.findByText("Version 2.1.0"));

    await waitFor(() =>
      expect(modsApi.updateInstance).toHaveBeenCalledWith({
        instanceId: "instance-1",
        mods: [{ modId: "playercorpse", versionId: "8" }],
        allowIncompatible: false,
      }),
    );
    await waitFor(() =>
      expect(onModUpdatesChanged).toHaveBeenCalledWith(
        "instance-1",
        expect.objectContaining({ gameVersion: "1.20" }),
      ),
    );
  });

  it("shows version installation errors beside the mod selector", async () => {
    modsApi.updateInstance.mockRejectedValue(new Error("Selected version is incompatible"));
    renderModal();
    const user = await openModsTab();

    await user.click(await screen.findByRole("combobox", { name: "Update to Player Corpse" }));
    await user.click(await screen.findByText("Version 2.1.0"));

    expect((await screen.findByRole("alert")).textContent).toContain(
      "Selected version is incompatible",
    );
  });

  it("shows a confirm dialog before removing a mod when confirmDeletion is true", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: { confirmDeletion: true } });
    renderModal();
    const user = await openModsTab();
    await screen.findByText("Player Corpse");

    await user.click(screen.getByRole("button", { name: "Remove" }));
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(modsApi.remove).not.toHaveBeenCalled();

    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(modsApi.remove).toHaveBeenCalledWith("mod-1", false));
  });

  it("shows a confirm dialog when settings are still loading for mod removal", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: undefined });
    renderModal();
    const user = await openModsTab();
    await screen.findByText("Player Corpse");

    await user.click(screen.getByRole("button", { name: "Remove" }));
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(modsApi.remove).not.toHaveBeenCalled();
  });

  it("saves Optimum as the instance game client", async () => {
    settingsQuery.useOptimumStatusQuery.mockReturnValue({
      data: {
        path: "/optimum",
        executable: "/optimum/run.sh",
        gameVersion: "1.20",
        ready: true,
        message: "",
      },
    });
    renderModal();
    const user = await openSettingsTab();

    await user.click(screen.getByRole("combobox", { name: /Game client/ }));
    await user.click(screen.getByRole("option", { name: "Optimum" }));
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    await waitFor(() =>
      expect(instancesApi.update).toHaveBeenCalledWith(
        expect.objectContaining({ id: "instance-1", gameClient: "optimum" }),
      ),
    );
  });

  it("saves a selected cover only after submitting settings", async () => {
    renderModal();
    const user = await openSettingsTab();

    await user.click(screen.getByText("Instance cover"));
    expect(instancesApi.selectCover).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Select" }));
    expect(instancesApi.selectCover).toHaveBeenCalledOnce();
    expect(instancesApi.update).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() =>
      expect(instancesApi.update).toHaveBeenCalledWith(
        expect.objectContaining({ coverSourcePath: "/pictures/cover.png" }),
      ),
    );
  });

  it("warns when Optimum targets a different game version", async () => {
    settingsQuery.useOptimumStatusQuery.mockReturnValue({
      data: {
        path: "/optimum",
        executable: "/optimum/run.sh",
        gameVersion: "1.22.5",
        ready: true,
        message: "",
      },
    });
    renderModal();
    const user = await openSettingsTab();

    await user.click(screen.getByRole("combobox", { name: /Game client/ }));
    await user.click(screen.getByRole("option", { name: "Optimum" }));

    expect((await screen.findByRole("alert")).textContent).toContain(
      "This Optimum installation targets Vintage Story 1.22.5, but this instance uses 1.20.",
    );
  });
});
