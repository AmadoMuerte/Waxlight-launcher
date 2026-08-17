// @vitest-environment jsdom

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { DownloadedMod, GameVersion, Instance, ModDetails } from "../../shared/api";
import { InstancePickerDialog } from "./InstancePickerDialog";

const api = vi.hoisted(() => ({
  installDownloaded: vi.fn(),
  download: vi.fn(),
  cancelTask: vi.fn(),
}));

vi.mock("../../shared/api/mod-catalog", () => ({ modCatalogApi: api }));

const instance = (id: string, name: string): Instance => ({
  id,
  name,
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
});

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

const mod: ModDetails = {
  id: "51",
  name: "Player Corpse",
  authorName: "Ada",
  summary: "Creates a recoverable corpse after death.",
  side: "both",
  gameVersions: ["1.20"],
  downloads: 100,
  tags: [],
  isDownloaded: true,
  isInstalled: true,
  updateAvailable: false,
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

const downloaded: DownloadedMod = {
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
    { instanceId: "inst-1", instanceName: "Survival", version: "2.0.0", enabled: true },
  ],
  updateAvailable: false,
};

function renderDialog(downloadedMod?: DownloadedMod) {
  render(
    <InstancePickerDialog
      mod={mod}
      downloaded={downloadedMod}
      instances={[instance("inst-1", "Survival"), instance("inst-2", "Creative")]}
      gameVersions={versions}
      onClose={() => {}}
      onDone={async () => {}}
    />,
  );
}

describe("instance picker installed indicator", () => {
  afterEach(() => cleanup());
  beforeEach(() => {
    api.download.mockReset();
    api.installDownloaded.mockReset();
    api.cancelTask.mockReset();
  });

  it("marks instances that already have the mod as installed", async () => {
    renderDialog(downloaded);

    // The installed instance cannot be selected again; the other can.
    expect(
      screen.queryByRole("checkbox", { name: "Select Survival for Player Corpse" }),
    ).toBeNull();
    expect(
      screen.getByRole("checkbox", { name: "Select Creative for Player Corpse" }),
    ).toBeTruthy();
    expect(screen.getAllByText("Installed")).toHaveLength(1);
  });

  it("shows plain checkboxes when the mod is not downloaded", async () => {
    renderDialog(undefined);
    expect(screen.getAllByRole("checkbox", { name: /for Player Corpse/ })).toHaveLength(2);
    expect(screen.queryByText("Installed")).toBeNull();
  });

  it("does not cancel a task that already finished", async () => {
    api.download.mockResolvedValue({
      taskId: "task-1",
      downloaded: {
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
      installations: [],
    });
    const onClose = vi.fn();
    render(
      <InstancePickerDialog
        mod={mod}
        instances={[instance("inst-1", "Survival")]}
        gameVersions={versions}
        onClose={onClose}
        onDone={async () => {}}
      />,
    );

    const user = userEvent.setup();
    await user.click(screen.getByText("Download only"));
    await waitFor(() => expect(screen.getByText("Done")).toBeTruthy());

    // The task completed; closing the dialog (here via Escape through the
    // modal chrome) must not ask the backend to cancel it (a finished task
    // would answer "Mod task not found").
    await user.keyboard("{Escape}");
    expect(api.cancelTask).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });
});
