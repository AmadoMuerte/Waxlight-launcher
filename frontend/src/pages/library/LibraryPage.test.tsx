// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { GameVersion } from "../../entities/game-version/model";
import type { Instance } from "../../entities/instance/model";
import type { Settings } from "../../entities/settings/model";
import { INSTANCES_QUERY_KEY } from "../../shared/api/keys";
import { LibraryPage } from "./LibraryPage";

const api = vi.hoisted(() => ({
  create: vi.fn(),
  clone: vi.fn(),
  get: vi.fn(),
  list: vi.fn(),
  detectExistingData: vi.fn(),
  inspectExistingData: vi.fn(),
  importExistingData: vi.fn(),
  update: vi.fn(),
  setPinned: vi.fn(),
  remove: vi.fn(),
}));

const versionsApi = vi.hoisted(() => ({ list: vi.fn() }));
const accountsApi = vi.hoisted(() => ({ list: vi.fn() }));
const instancePackageApi = vi.hoisted(() => ({ import: vi.fn(), selectPackageFile: vi.fn() }));
const launcherApi = vi.hoisted(() => ({ validate: vi.fn(), launch: vi.fn(), stop: vi.fn() }));
const modsApi = vi.hoisted(() => ({ checkInstanceUpdates: vi.fn(), list: vi.fn() }));
const operationsApi = vi.hoisted(() => ({ list: vi.fn(), cancel: vi.fn() }));
const settingsApi = vi.hoisted(() => ({
  get: vi.fn(),
  update: vi.fn(),
  setLibrarySort: vi.fn(),
  openDirectory: vi.fn(),
  selectGameDirectory: vi.fn(),
}));

vi.mock("../../shared/api/instances", () => ({ instancesApi: api }));
vi.mock("../../shared/api/game-versions", () => ({ versionsApi }));
vi.mock("../../shared/api/accounts", () => ({ accountsApi }));
vi.mock("../../shared/api/launcher", () => ({ launcherApi }));
vi.mock("../../shared/api/mods", () => ({ modsApi }));
vi.mock("../../shared/api/settings", () => ({ settingsApi }));
vi.mock("../../shared/api/instance-package", () => ({ instancePackageApi }));
vi.mock("../../shared/api/operations", () => ({ operationsApi }));

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

const instances: Instance[] = [
  {
    id: "inst-1",
    name: "Warm home",
    description: "A cozy base",
    gameVersionId: "1.20",
    gameClient: "vanilla",
    directory: "/instances/inst-1",
    status: "ready",
    launchArguments: [],
    environmentVariables: {},
    isPinned: false,
    createdAt: "2026-01-01T00:00:00Z",
    enabledModCount: 0,
    totalModCount: 0,
    playtimeSeconds: 0,
  },
];

const settings: Settings = {
  language: "en",
  downloadsParallel: 3,
  confirmDeletion: true,
  globalLaunchArguments: [],
  optimumPath: "",
  checkForUpdates: true,
  updateChannel: "stable",
  skippedUpdateVersion: "",
  telemetryEnabled: false,
  richPresenceEnabled: true,
  automaticSafetySnapshots: true,
  automaticSnapshotRetention: 10,
  librarySort: "lastPlayed",
  uiScale: 1,
};

function instance(id: string, name: string, overrides: Partial<Instance> = {}): Instance {
  return { ...instances[0], id, name, directory: `/instances/${id}`, ...overrides };
}

const instancesFixture: Instance[] = [
  instance("one", "One"),
  instance("two", "Two"),
  instance("three", "Three"),
  instance("four", "Four"),
];

function cardOrder() {
  return screen.getAllByRole("heading", { level: 3 }).map((heading) => heading.textContent);
}

async function renderPage(data = instances, availableVersions = versions) {
  api.list.mockResolvedValue(data);
  versionsApi.list.mockResolvedValue(availableVersions);
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
  if (data.length > 0) {
    await screen.findByRole("heading", { level: 3, name: data[0].name });
  } else {
    await screen.findByRole("heading", { name: "Light your first world" });
  }
  return queryClient;
}

describe("library instance creation", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    api.create.mockResolvedValue({});
    api.clone.mockResolvedValue({ id: "inst-2", name: "Warm home copy" });
    api.remove.mockResolvedValue(undefined);
    api.update.mockResolvedValue({});
    api.setPinned.mockResolvedValue({});
    modsApi.checkInstanceUpdates.mockResolvedValue({ summary: { updatesAvailable: 0 } });
    settingsApi.get.mockResolvedValue(settings);
    settingsApi.update.mockImplementation(async (value) => value);
    settingsApi.setLibrarySort.mockImplementation(async (librarySort) => ({
      ...settings,
      librarySort,
    }));
    instancePackageApi.selectPackageFile.mockResolvedValue("/tmp/cozy-camp.waxlight");
    instancePackageApi.import.mockResolvedValue({ id: "operation-1", status: "queued" });
    operationsApi.list.mockResolvedValue([]);
  });

  it("submits an empty name so the backend generates a unique default", async () => {
    await renderPage();

    await userEvent.setup().click(screen.getByRole("button", { name: "Add instance" }));
    await userEvent.setup().click(screen.getByRole("button", { name: /Create new instance/ }));
    await userEvent.setup().click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(api.create).toHaveBeenCalledWith(
        expect.objectContaining({ name: "", gameVersionId: "1.20" }),
      );
    });
  });

  it("submits the typed name when provided", async () => {
    await renderPage();

    await userEvent.setup().click(screen.getByRole("button", { name: "Add instance" }));
    await userEvent.setup().click(screen.getByRole("button", { name: /Create new instance/ }));
    await userEvent.setup().type(screen.getByLabelText("Name"), "My cozy world");
    await userEvent.setup().click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(api.create).toHaveBeenCalledWith(expect.objectContaining({ name: "My cozy world" }));
    });
  });

  it("clones an instance with the typed name", async () => {
    await renderPage();

    await userEvent.setup().click(screen.getByRole("button", { name: "Open instance details" }));
    await userEvent.setup().click(screen.getByRole("button", { name: /Clone/ }));
    await userEvent.setup().clear(screen.getByLabelText("Name"));
    await userEvent.setup().type(screen.getByLabelText("Name"), "Warm home copy");

    const cloneButtons = screen.getAllByRole("button", { name: /Clone/ });
    await userEvent.setup().click(cloneButtons[cloneButtons.length - 1]);

    await waitFor(() => {
      expect(api.clone).toHaveBeenCalledWith({ sourceId: "inst-1", name: "Warm home copy" });
    });

    await waitFor(() => {
      expect(screen.queryByRole("dialog", { name: "Warm home" })).toBeNull();
    });
  });

  it("starts import immediately after selecting a package", async () => {
    await renderPage();

    await userEvent.setup().click(screen.getByRole("button", { name: "Add instance" }));
    await userEvent.setup().click(screen.getByRole("button", { name: /Import Waxlight package/ }));

    await waitFor(() => {
      expect(instancePackageApi.import).toHaveBeenCalledWith({
        packagePath: "/tmp/cozy-camp.waxlight",
        name: "",
        description: "",
        directory: "",
        gameVersionId: "",
        installVersion: true,
        allowIncompatible: false,
        skipUnavailable: true,
      });
    });
  });

  it("checks mod updates after render with at most three requests in flight", async () => {
    const resolvers: Array<(value: { summary: { updatesAvailable: number } }) => void> = [];
    modsApi.checkInstanceUpdates.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolvers.push(resolve);
        }),
    );
    await renderPage([
      instance("one", "One"),
      instance("two", "Two"),
      instance("three", "Three"),
      instance("four", "Four"),
    ]);

    await waitFor(() => expect(modsApi.checkInstanceUpdates).toHaveBeenCalledTimes(3));
    resolvers[0]({ summary: { updatesAvailable: 1 } });

    await waitFor(() => expect(modsApi.checkInstanceUpdates).toHaveBeenCalledTimes(4));
    expect(await screen.findByText("1 mod update available")).toBeTruthy();
  });

  it("keeps the in-flight mod update cap across effect re-runs", async () => {
    let inFlight = 0;
    let maxInFlight = 0;
    const releases: Array<() => void> = [];

    modsApi.checkInstanceUpdates.mockImplementation(() => {
      inFlight += 1;
      maxInFlight = Math.max(maxInFlight, inFlight);
      return new Promise((resolve) => {
        releases.push(() => {
          inFlight -= 1;
          resolve({ summary: { updatesAvailable: 0 }, updates: [] });
        });
      });
    });

    const queryClient = await renderPage(instancesFixture.slice(0, 4));

    await waitFor(() => expect(modsApi.checkInstanceUpdates).toHaveBeenCalledTimes(3));

    await act(async () => {
      const refreshed = instancesFixture.slice(0, 4);
      const renamed = { ...refreshed[0] };
      renamed.name = `${renamed.name} (renamed)`;
      refreshed[0] = renamed;
      queryClient.setQueryData(INSTANCES_QUERY_KEY, refreshed);
      await new Promise((resolve) => setTimeout(resolve, 30));
    });

    expect(maxInFlight).toBeLessThanOrEqual(3);

    const firstBatch = releases.splice(0, releases.length);
    await act(async () => {
      for (const release of firstBatch) release();
      await new Promise((resolve) => setTimeout(resolve, 30));
    });

    await act(async () => {
      const refreshed = instancesFixture.slice(0, 4);
      const renamed = { ...refreshed[1] };
      renamed.name = `${renamed.name} (renamed)`;
      refreshed[1] = renamed;
      queryClient.setQueryData(INSTANCES_QUERY_KEY, refreshed);
    });

    await waitFor(() => expect(modsApi.checkInstanceUpdates).toHaveBeenCalledTimes(4));
    await act(async () => {
      for (const release of releases.splice(0, releases.length)) release();
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
  });

  it("shows the add chooser when the Library is empty", async () => {
    await renderPage([]);

    expect(screen.getByRole("heading", { name: "Light your first world" })).toBeTruthy();
    expect(screen.getAllByRole("button", { name: "Add instance" })).toHaveLength(2);

    await userEvent.setup().click(screen.getAllByRole("button", { name: "Add instance" })[0]);
    expect(screen.getByRole("button", { name: /Create new instance/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: /Import Waxlight package/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: /Import existing Vintage Story data/ })).toBeTruthy();
  });

  it("confirms destructive overflow actions before deleting an instance", async () => {
    await renderPage();
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "Actions for Warm home" }));
    await user.click(screen.getByRole("menuitem", { name: "Delete instance" }));

    const dialog = await screen.findByRole("dialog", {
      name: "Delete “Warm home” and all of its files?",
    });
    expect(api.remove).not.toHaveBeenCalled();
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));

    await waitFor(() => expect(api.remove).toHaveBeenCalledWith("inst-1", true));
  });

  it("restores name sorting within pinned and unpinned groups", async () => {
    settingsApi.get.mockResolvedValue({ ...settings, librarySort: "name" });
    await renderPage([
      instance("unpinned-d", "D"),
      instance("pinned-b", "B", { isPinned: true }),
      instance("unpinned-c", "C"),
      instance("pinned-a", "A", { isPinned: true }),
    ]);

    expect(cardOrder()).toEqual(["A", "B", "C", "D"]);
  });

  it("sorts played instances newest first and never-played instances last", async () => {
    await renderPage([
      instance("never-a", "A"),
      instance("older", "Older", { lastPlayedAt: "2026-01-01T00:00:00Z" }),
      instance("newer", "Newer", { lastPlayedAt: "2026-02-01T00:00:00Z" }),
      instance("never-b", "B"),
    ]);

    expect(cardOrder()).toEqual(["Newer", "Older", "A", "B"]);
  });

  it("keeps pinning and sorting inside an active search", async () => {
    settingsApi.get.mockResolvedValue({ ...settings, librarySort: "name" });
    await renderPage([
      instance("test-b", "Test B"),
      instance("other", "Other", { isPinned: true }),
      instance("test-c", "Test C"),
      instance("test-a", "Test A", { isPinned: true }),
    ]);

    await userEvent.setup().type(screen.getByRole("textbox", { name: "Search instances" }), "test");

    expect(cardOrder()).toEqual(["Test A", "Test B", "Test C"]);
  });

  it("moves a pinned instance immediately and persists the pin", async () => {
    await renderPage([
      instance("played", "Played", { lastPlayedAt: "2026-02-01T00:00:00Z" }),
      instance("favorite", "Favorite"),
    ]);
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "Pin instance: Favorite" }));

    expect(cardOrder()).toEqual(["Favorite", "Played"]);
    await waitFor(() => expect(api.setPinned).toHaveBeenCalledWith("favorite", true));
  });

  it("persists sort changes and reorders immediately", async () => {
    await renderPage([instance("b", "B"), instance("a", "A")]);
    const user = userEvent.setup();

    await user.click(screen.getByRole("combobox", { name: "Sort by" }));
    await user.click(await screen.findByRole("option", { name: "Name" }));

    expect(cardOrder()).toEqual(["A", "B"]);
    await waitFor(() => expect(settingsApi.setLibrarySort).toHaveBeenCalledWith("name"));
  });

  it("sorts game versions numerically instead of lexically", async () => {
    await renderPage([
      instance("old", "Old", { gameVersionId: "1.9" }),
      instance("new", "New", { gameVersionId: "1.10" }),
    ]);
    const user = userEvent.setup();

    await user.click(screen.getByRole("combobox", { name: "Sort by" }));
    await user.click(await screen.findByRole("option", { name: "Game version" }));

    expect(cardOrder()).toEqual(["New", "Old"]);
  });

  it("sorts stable game versions above matching prereleases", async () => {
    settingsApi.get.mockResolvedValue({ ...settings, librarySort: "gameVersion" });
    await renderPage([
      instance("preview", "Preview", { gameVersionId: "1.20.0-rc.1" }),
      instance("stable", "Stable", { gameVersionId: "1.20.0" }),
    ]);

    expect(cardOrder()).toEqual(["Stable", "Preview"]);
  });

  it("sorts by displayed game version names when IDs differ", async () => {
    settingsApi.get.mockResolvedValue({ ...settings, librarySort: "gameVersion" });
    await renderPage(
      [
        instance("old", "Old", { gameVersionId: "custom-z" }),
        instance("new", "New", { gameVersionId: "custom-a" }),
      ],
      [
        { ...versions[0], id: "custom-z", name: "1.9" },
        { ...versions[0], id: "custom-a", name: "1.10" },
      ],
    );

    expect(cardOrder()).toEqual(["New", "Old"]);
  });
});
