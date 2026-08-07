// @vitest-environment jsdom

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ModsPage } from "./ModsPage";

const api = vi.hoisted(() => ({
  search: vi.fn(),
  get: vi.fn(),
  downloaded: vi.fn(),
  download: vi.fn(),
  installDownloaded: vi.fn(),
  removeDownloaded: vi.fn(),
  uploadMods: vi.fn(),
  uploadMod: vi.fn(),
  cancelTask: vi.fn(),
  checkUpdates: vi.fn(),
  tags: vi.fn(),
}));

const settings = vi.hoisted(() => ({
  selectModFiles: vi.fn(),
}));

vi.mock("../shared/api", () => ({ modCatalogApi: api, settingsApi: settings }));

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
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route
          path="/mods"
          element={
            <ModsPage
              instances={[
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
              ]}
              versions={[
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
              ]}
              notify={notify}
            />
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe("mods browser", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
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
