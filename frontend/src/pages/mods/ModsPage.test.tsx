// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useToastStore } from "../../app/stores/toast";
import { ModsPage } from "./ModsPage";

const api = vi.hoisted(() => ({
  search: vi.fn(),
  get: vi.fn(),
  downloaded: vi.fn(),
  download: vi.fn(),
  downloadBatch: vi.fn(),
  installDownloaded: vi.fn(),
  removeDownloaded: vi.fn(),
  previewUnusedDownloaded: vi.fn(),
  removeUnusedDownloaded: vi.fn(),
  uploadMods: vi.fn(),
  uploadMod: vi.fn(),
  cancelTask: vi.fn(),
  checkUpdates: vi.fn(),
  tags: vi.fn(),
}));

const settings = vi.hoisted(() => ({
  selectModFiles: vi.fn(),
}));

const settingsQuery = vi.hoisted(() => ({ useSettingsQuery: vi.fn() }));

const instancesList = vi.hoisted(() => vi.fn());
const versionsList = vi.hoisted(() => vi.fn());
const availableVersionsList = vi.hoisted(() => vi.fn());

vi.mock("../../shared/api/mod-catalog", () => ({ modCatalogApi: api }));
vi.mock("../../shared/api/settings", () => ({ settingsApi: settings }));
vi.mock("../../entities/settings/queries", () => settingsQuery);
vi.mock("../../shared/api/instances", () => ({ instancesApi: { list: instancesList } }));
vi.mock("../../shared/api/game-versions", () => ({
  versionsApi: { list: versionsList, available: availableVersionsList },
}));

const summary = {
  id: "51",
  name: "Player Corpse",
  authorName: "Ada",
  summary: "Creates a recoverable corpse after death.",
  side: "both",
  gameVersions: [],
  downloads: 42_000,
  updatedAt: "2026-08-01T10:00:00Z",
  tags: ["Utility"],
  isDownloaded: false,
  isInstalled: false,
  updateAvailable: false,
};

const details = {
  ...summary,
  description: "<p>Full description</p>",
  screenshots: [],
  versions: [
    {
      id: "7",
      version: "2.0.0",
      gameVersions: ["1.20"],
      releaseType: "stable",
      fileName: "playercorpse.zip",
      fileSize: 100,
    },
  ],
};

function renderPage(path = "/mods?q=corpse", notify = vi.fn()) {
  instancesList.mockResolvedValue([
    {
      id: "instance-1",
      name: "Survival",
      description: "",
      gameVersionId: "1.20",
      directory: "/data",
      status: "ready",
      launchArguments: [],
      createdAt: "2026-01-01T00:00:00Z",
      enabledModCount: 0,
      totalModCount: 0,
      playtimeSeconds: 0,
    },
  ]);
  versionsList.mockResolvedValue([
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
  ]);
  availableVersionsList.mockResolvedValue([
    {
      id: "1.20.2",
      name: "1.20.2",
      channel: "stable",
      platform: "linux",
      architecture: "amd64",
      downloadSize: 1,
      latest: true,
      installed: false,
    },
    {
      id: "1.20.0-rc.1",
      name: "1.20.0-rc.1",
      channel: "unstable",
      platform: "linux",
      architecture: "amd64",
      downloadSize: 1,
      latest: false,
      installed: false,
    },
    {
      id: "1.19.4",
      name: "1.19.4",
      channel: "stable",
      platform: "linux",
      architecture: "amd64",
      downloadSize: 1,
      latest: false,
      installed: false,
    },
    {
      id: "1.4.8",
      name: "1.4.8",
      channel: "stable",
      platform: "linux",
      architecture: "amd64",
      downloadSize: 1,
      latest: false,
      installed: false,
    },
  ]);
  useToastStore.setState({ notify });
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/mods" element={<ModsPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("mods browser", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    settingsQuery.useSettingsQuery.mockReturnValue({ data: undefined });
    api.search.mockResolvedValue({
      items: [summary],
      page: 1,
      pageSize: 24,
      totalItems: 1,
      totalPages: 1,
      hasNext: false,
    });
    api.get.mockResolvedValue(details);
    api.downloaded.mockResolvedValue([]);
    api.checkUpdates.mockResolvedValue([]);
    api.tags.mockResolvedValue([]);
    api.previewUnusedDownloaded.mockResolvedValue({ removedCount: 0, freedBytes: 0 });
    api.removeUnusedDownloaded.mockResolvedValue({ removedCount: 0, freedBytes: 0 });
  });

  it("loads a URL-backed search and opens the instance picker", async () => {
    renderPage();

    expect(await screen.findByText("Player Corpse")).toBeTruthy();
    expect(api.search).toHaveBeenCalledWith(expect.objectContaining({ text: "corpse", page: 1 }));

    await userEvent.setup().click(screen.getByRole("button", { name: "Download" }));
    expect(await screen.findByRole("dialog", { name: "Download “Player Corpse”" })).toBeTruthy();
    expect(screen.getByText("Survival")).toBeTruthy();
    expect(screen.getByText("Compatible")).toBeTruthy();
  });

  it("adds selected mods to an instance as a batch", async () => {
    api.downloadBatch.mockResolvedValue([
      { modId: "51", versionId: "7", result: { installations: [] } },
    ]);
    renderPage("/mods");
    const user = userEvent.setup();

    await user.click(await screen.findByRole("checkbox", { name: "Select Player Corpse" }));
    await user.click(screen.getByRole("button", { name: "Add to an instance or create one" }));
    expect(await screen.findByRole("dialog", { name: "Add mods to an instance" })).toBeTruthy();

    await user.click(screen.getByRole("radio", { name: /Survival/ }));
    await user.click(screen.getByRole("button", { name: "Add to instance" }));

    await waitFor(() =>
      expect(api.downloadBatch).toHaveBeenCalledWith({
        instanceId: "instance-1",
        targets: [{ modId: "51", versionId: "7" }],
      }),
    );
  });

  it("marks an instance as already installed in the batch dialog", async () => {
    api.downloaded.mockResolvedValue([
      {
        modId: "51",
        name: "Player Corpse",
        authorName: "Ada",
        side: "both",
        versionId: "7",
        downloadedVersion: "2.0.0",
        gameVersions: ["1.20"],
        fileName: "playercorpse.zip",
        fileSize: 100,
        downloadedAt: "2026-08-02T10:00:00Z",
        installedInstances: [
          { instanceId: "instance-1", instanceName: "Survival", version: "2.0.0", enabled: true },
        ],
        updateAvailable: false,
      },
    ]);
    renderPage("/mods");
    const user = userEvent.setup();

    await user.click(await screen.findByRole("checkbox", { name: "Select Player Corpse" }));
    await user.click(screen.getByRole("button", { name: "Add to an instance or create one" }));
    expect(await screen.findByRole("dialog", { name: "Add mods to an instance" })).toBeTruthy();

    const survival = screen.getByText("Survival").closest("label");
    expect(survival?.className).toContain("installed");
    expect(screen.queryAllByRole("radio", { name: /Survival/ })).toHaveLength(0);
    expect(screen.getAllByText("Installed")).toHaveLength(1);
  });

  it("confirms before removing downloaded mods unused by instances", async () => {
    api.previewUnusedDownloaded.mockResolvedValue({ removedCount: 2, freedBytes: 100 });
    api.removeUnusedDownloaded.mockResolvedValue({ removedCount: 2, freedBytes: 100 });
    renderPage("/mods?view=downloaded");
    const user = userEvent.setup();

    await user.click(
      await screen.findByRole("button", { name: "Remove mods not installed in instances" }),
    );
    expect(await screen.findByText("2 mods will be removed.")).toBeTruthy();
    expect(api.removeUnusedDownloaded).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Remove" }));
    await waitFor(() => expect(api.removeUnusedDownloaded).toHaveBeenCalled());
  });

  it("filters the catalog by tags from the tag dropdown", async () => {
    api.tags.mockResolvedValue([
      { name: "Utility", count: 2 },
      { name: "Graphics", count: 1 },
    ]);
    renderPage("/mods");
    await screen.findByText("Player Corpse");

    await userEvent.setup().click(screen.getByRole("button", { name: "Tags" }));
    await userEvent.setup().click(screen.getByRole("menuitemcheckbox", { name: /Utility/ }));

    await waitFor(() =>
      expect(api.search).toHaveBeenCalledWith(expect.objectContaining({ tags: ["Utility"] })),
    );
    expect(await screen.findByText("Tag: Utility")).toBeTruthy();
  });

  it("filters downloaded mods by tags from the tag dropdown", async () => {
    api.downloaded.mockResolvedValue([
      {
        modId: "51",
        name: "Player Corpse",
        authorName: "Ada",
        side: "both",
        versionId: "7",
        downloadedVersion: "2.0.0",
        gameVersions: ["1.20"],
        tags: ["Utility"],
        fileName: "playercorpse.zip",
        fileSize: 100,
        downloadedAt: "2026-08-02T10:00:00Z",
        installedInstances: [],
        updateAvailable: false,
      },
    ]);
    api.checkUpdates.mockImplementation(() => api.downloaded());
    api.tags.mockResolvedValue([{ name: "Graphics", count: 1 }]);
    renderPage("/mods?view=downloaded");
    await screen.findByText("Player Corpse");

    await userEvent.setup().click(screen.getByRole("button", { name: "Tags" }));
    await userEvent.setup().click(screen.getByRole("menuitemcheckbox", { name: /Graphics/ }));

    expect(await screen.findByText("No downloaded mods yet")).toBeTruthy();
  });

  it("lists game version series from the official catalog in the filter", async () => {
    renderPage("/mods");
    await screen.findByText("Player Corpse");

    await userEvent.setup().click(screen.getByRole("combobox", { name: /Game version/ }));

    expect(await screen.findByRole("option", { name: "1.20.x" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "1.19.x" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "1.4.x" })).toBeTruthy();
    expect(screen.queryByRole("option", { name: "1.20.2" })).toBeNull();

    await userEvent.setup().click(screen.getByRole("option", { name: "1.19.x" }));
    await waitFor(() =>
      expect(api.search).toHaveBeenCalledWith(expect.objectContaining({ gameVersion: "1.19.x" })),
    );
  });

  it("filters downloaded mods by the selected game version series", async () => {
    api.downloaded.mockResolvedValue([
      {
        modId: "51",
        name: "Player Corpse",
        authorName: "Ada",
        side: "both",
        versionId: "7",
        downloadedVersion: "2.0.0",
        gameVersions: ["1.20"],
        fileName: "playercorpse.zip",
        fileSize: 100,
        downloadedAt: "2026-08-02T10:00:00Z",
        installedInstances: [],
        updateAvailable: false,
      },
    ]);
    api.checkUpdates.mockImplementation(() => api.downloaded());
    renderPage("/mods?view=downloaded");
    await screen.findByRole("tab", { name: /Downloaded/ });
    expect(await screen.findByText("Player Corpse")).toBeTruthy();

    await userEvent.setup().click(screen.getByRole("combobox", { name: /Game version/ }));
    await userEvent.setup().click(screen.getByRole("option", { name: "1.19.x" }));

    await waitFor(() => expect(screen.queryByText("Player Corpse")).toBeNull());

    await userEvent.setup().click(screen.getByRole("combobox", { name: /Game version/ }));
    await userEvent.setup().click(screen.getByRole("option", { name: "1.20.x" }));

    expect(await screen.findByText("Player Corpse")).toBeTruthy();
  });

  it("keeps downloaded mods available through the dedicated tab", async () => {
    api.downloaded.mockResolvedValue([
      {
        modId: "51",
        name: "Player Corpse",
        authorName: "Ada",
        side: "both",
        versionId: "7",
        downloadedVersion: "2.0.0",
        gameVersions: ["1.20"],
        fileName: "playercorpse.zip",
        fileSize: 100,
        downloadedAt: "2026-08-02T10:00:00Z",
        installedInstances: [],
        updateAvailable: false,
      },
    ]);
    api.checkUpdates.mockImplementation(() => api.downloaded());
    renderPage("/mods");
    await screen.findByText("Player Corpse");

    await userEvent.setup().click(screen.getByRole("tab", { name: /Downloaded/ }));
    await waitFor(() => expect(api.downloaded).toHaveBeenCalled());
    expect(await screen.findByText("Version 2.0.0 · 100 B")).toBeTruthy();
    expect(screen.getByText("Downloaded · Not installed")).toBeTruthy();
  });

  it("uploads local mods into the library and reports the link result", async () => {
    const notify = vi.fn();
    api.downloaded.mockResolvedValue([]);
    settings.selectModFiles.mockResolvedValue(["/tmp/corpse.zip", "/tmp/mystery.zip"]);
    api.uploadMods.mockResolvedValue({
      linked: [
        {
          name: "Player Corpse",
          version: "2.0.0",
          fileName: "corpse.zip",
          modId: "51",
          versionId: "7",
          updateAvailable: false,
        },
      ],
      notMatched: [
        {
          name: "Mystery Mod",
          version: "1.0.0",
          fileName: "mystery.zip",
          path: "/tmp/mystery.zip",
          updateAvailable: false,
          reason: "not_in_catalog",
        },
      ],
      skipped: [],
      failed: [],
    });
    renderPage("/mods?view=downloaded", notify);
    await screen.findByRole("tab", { name: /Downloaded/ });

    await userEvent.setup().click(screen.getAllByRole("button", { name: /Upload mods/ })[0]);

    await waitFor(() =>
      expect(api.uploadMods).toHaveBeenCalledWith(["/tmp/corpse.zip", "/tmp/mystery.zip"]),
    );
    expect(notify).toHaveBeenCalledWith(expect.stringContaining("linked"));
    expect(notify).toHaveBeenCalledWith(expect.stringContaining("not found"));
  });
});

describe("confirmDeletion gate", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    settingsQuery.useSettingsQuery.mockReturnValue({ data: undefined });
    api.search.mockResolvedValue({
      items: [summary],
      page: 1,
      pageSize: 24,
      totalItems: 1,
      totalPages: 1,
      hasNext: false,
    });
    api.get.mockResolvedValue(details);
    api.downloaded.mockResolvedValue([
      {
        modId: "51",
        name: "Player Corpse",
        authorName: "Ada",
        side: "both",
        versionId: "7",
        downloadedVersion: "2.0.0",
        gameVersions: ["1.20"],
        fileName: "playercorpse.zip",
        fileSize: 100,
        downloadedAt: "2026-08-02T10:00:00Z",
        installedInstances: [],
        updateAvailable: false,
      },
    ]);
    api.checkUpdates.mockImplementation(() => api.downloaded());
    api.removeDownloaded.mockResolvedValue(undefined);
    api.tags.mockResolvedValue([]);
  });

  it("removes a downloaded mod directly when confirmDeletion is false", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: { confirmDeletion: false } });
    const user = userEvent.setup();
    renderPage("/mods?view=downloaded");
    await screen.findByRole("tab", { name: /Downloaded/ });
    await screen.findByText("Player Corpse");

    await user.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => expect(api.removeDownloaded).toHaveBeenCalledWith("51", "7"));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("shows a confirm dialog before removing when confirmDeletion is true", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: { confirmDeletion: true } });
    const user = userEvent.setup();
    renderPage("/mods?view=downloaded");
    await screen.findByRole("tab", { name: /Downloaded/ });
    await screen.findByText("Player Corpse");

    await user.click(screen.getByRole("button", { name: "Delete" }));
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(api.removeDownloaded).not.toHaveBeenCalled();

    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(api.removeDownloaded).toHaveBeenCalledWith("51", "7"));
  });

  it("shows a confirm dialog when settings are still loading", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: undefined });
    const user = userEvent.setup();
    renderPage("/mods?view=downloaded");
    await screen.findByRole("tab", { name: /Downloaded/ });
    await screen.findByText("Player Corpse");

    await user.click(screen.getByRole("button", { name: "Delete" }));
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(api.removeDownloaded).not.toHaveBeenCalled();
  });
});
