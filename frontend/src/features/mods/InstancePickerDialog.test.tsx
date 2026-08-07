// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { DownloadedMod, GameVersion, Instance, ModDetails } from "../../shared/api";
import { InstancePickerDialog } from "./InstancePickerDialog";

const api = vi.hoisted(() => ({
  installDownloaded: vi.fn(),
  download: vi.fn(),
  cancelTask: vi.fn(),
}));

vi.mock("../../shared/api", () => ({ modCatalogApi: api }));

const instance = (id: string, name: string): Instance => ({
  id,
  name,
  description: "",
  gameVersionId: "1.20",
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

  it("marks instances that already have the mod as installed", async () => {
    renderDialog(downloaded);

    const survival = screen.getByText("Survival").closest("label");
    const creative = screen.getByText("Creative").closest("label");
    expect(survival?.className).toContain("installed");
    expect(creative?.className).not.toContain("installed");

    // The installed instance cannot be selected again; the other can.
    const instanceChecks = document.querySelectorAll(".instanceChoice input");
    expect(instanceChecks).toHaveLength(1);
    expect(screen.getAllByText("Installed")).toHaveLength(1);
  });

  it("shows plain checkboxes when the mod is not downloaded", async () => {
    renderDialog(undefined);
    expect(document.querySelectorAll(".instanceChoice input")).toHaveLength(2);
    expect(screen.queryByText("Installed")).toBeNull();
  });
});
