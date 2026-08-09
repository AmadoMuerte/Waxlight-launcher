// @vitest-environment jsdom

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import type {
  DownloadedMod,
  GameVersion,
  Instance,
  ModDetails,
  ModVersion,
} from "../../shared/api";
import { BatchInstancePickerDialog } from "./BatchInstancePickerDialog";

const api = vi.hoisted(() => ({
  downloadBatch: vi.fn(),
}));

vi.mock("../../shared/api/mod-catalog", () => ({ modCatalogApi: api }));

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

const release = (id: string, version: string): ModVersion => ({
  id,
  version,
  gameVersions: ["1.20"],
  releaseType: "stable",
  fileName: `${version}.zip`,
  fileSize: 100,
});

const mod = (id: string, name: string): ModDetails => ({
  id,
  name,
  authorName: "Ada",
  summary: `${name} summary.`,
  side: "both",
  gameVersions: ["1.20"],
  downloads: 100,
  tags: [],
  isDownloaded: true,
  isInstalled: true,
  updateAvailable: false,
  description: "<p>Full description</p>",
  screenshots: [],
  versions: [release("7", "2.0.0")],
});

const downloaded = (modId: string, instanceIds: string[], versionId = "7"): DownloadedMod => ({
  modId,
  name: "Player Corpse",
  authorName: "Ada",
  side: "both",
  versionId,
  downloadedVersion: "2.0.0",
  gameVersions: ["1.20"],
  fileName: "playercorpse.zip",
  fileSize: 100,
  downloadedAt: "2026-08-02T10:00:00Z",
  installedInstances: instanceIds.map((instanceId) => ({
    instanceId,
    instanceName: instanceId,
    version: "2.0.0",
    enabled: true,
  })),
  updateAvailable: false,
});

function renderDialog(
  mods: {
    details: ModDetails;
    release: ModVersion;
    downloaded?: DownloadedMod;
  }[],
) {
  render(
    <BatchInstancePickerDialog
      mods={mods}
      instances={[instance("inst-1", "Survival"), instance("inst-2", "Creative")]}
      gameVersions={versions}
      accounts={[]}
      onClose={() => {}}
      onCreated={async () => {}}
      onDone={async () => {}}
    />,
  );
}

describe("batch instance picker installed indicator", () => {
  afterEach(() => cleanup());

  it("disables instances that already have every selected mod installed", () => {
    renderDialog([
      {
        details: mod("51", "Player Corpse"),
        release: release("7", "2.0.0"),
        downloaded: downloaded("51", ["inst-1"]),
      },
    ]);

    const survival = screen.getByText("Survival").closest("label");
    const creative = screen.getByText("Creative").closest("label");
    expect(survival?.className).toContain("installed");
    expect(creative?.className).not.toContain("installed");

    // The installed instance cannot be selected again; the other can.
    expect(screen.getAllByRole("radio", { name: /Creative/ })).toHaveLength(1);
    expect(screen.getAllByText("Installed")).toHaveLength(1);
    expect(screen.getByText("Installed 2.0.0")).toBeTruthy();
  });

  it("marks instances with only part of the mods installed and keeps them selectable", () => {
    renderDialog([
      {
        details: mod("51", "Player Corpse"),
        release: release("7", "2.0.0"),
        downloaded: downloaded("51", ["inst-1"]),
      },
      {
        details: mod("52", "Better Rivers"),
        release: release("8", "1.5.0"),
      },
    ]);

    const survival = screen.getByText("Survival").closest("label");
    expect(survival?.className).not.toContain("installed");
    expect(screen.getAllByRole("radio", { name: /Survival/ })).toHaveLength(1);
    expect(screen.getByText("Installed 1 of 2 mods")).toBeTruthy();
    expect(screen.queryAllByText("Installed")).toHaveLength(0);
  });

  it("shows plain radios without hints when nothing is installed", () => {
    renderDialog([
      {
        details: mod("51", "Player Corpse"),
        release: release("7", "2.0.0"),
      },
    ]);

    expect(screen.getAllByRole("radio")).toHaveLength(2);
    expect(screen.queryByText(/Installed/)).toBeNull();
  });

  it("marks the instance as installed right after a successful batch install", async () => {
    api.downloadBatch.mockResolvedValue([
      {
        modId: "51",
        versionId: "7",
        result: {
          taskId: "task-1",
          downloaded: downloaded("51", []),
          installations: [
            {
              instanceId: "inst-1",
              instanceName: "Survival",
              installed: true,
              message: "Installed",
            },
          ],
        },
      },
    ]);
    renderDialog([
      {
        details: mod("51", "Player Corpse"),
        release: release("7", "2.0.0"),
      },
    ]);
    const user = userEvent.setup();

    await user.click(screen.getByRole("radio", { name: /Survival/ }));
    await user.click(screen.getByRole("button", { name: "Add to instance" }));

    await waitFor(() => expect(api.downloadBatch).toHaveBeenCalled());
    const survival = screen.getByText("Survival").closest("label");
    expect(survival?.className).toContain("installed");
    expect(screen.queryAllByRole("radio", { name: /Survival/ })).toHaveLength(0);
    expect(screen.getAllByText("Installed")).toHaveLength(1);
  });

  it("reports the real failure message when an installation fails", async () => {
    api.downloadBatch.mockResolvedValue([
      {
        modId: "51",
        versionId: "7",
        result: {
          taskId: "task-1",
          downloaded: downloaded("51", []),
          installations: [
            {
              instanceId: "inst-1",
              instanceName: "Survival",
              installed: false,
              message: "A different mod file with this name already exists in the instance",
            },
          ],
        },
      },
    ]);
    renderDialog([
      {
        details: mod("51", "Player Corpse"),
        release: release("7", "2.0.0"),
      },
    ]);
    const user = userEvent.setup();

    await user.click(screen.getByRole("radio", { name: /Survival/ }));
    await user.click(screen.getByRole("button", { name: "Add to instance" }));

    expect(
      await screen.findByText(/A different mod file with this name already exists/),
    ).toBeTruthy();
    expect(screen.queryByText(/Downloaded and installed/)).toBeNull();
    const survival = screen.getByText("Survival").closest("label");
    expect(survival?.className).not.toContain("installed");
  });

  it("installs version selected in the batch dialog", async () => {
    api.downloadBatch.mockResolvedValue([]);
    const user = userEvent.setup();
    const selectedMod = mod("51", "Player Corpse");
    selectedMod.versions = [release("7", "2.0.0"), release("8", "2.1.0")];
    renderDialog([{ details: selectedMod, release: selectedMod.versions[0] }]);

    await user.click(screen.getByRole("combobox", { name: "Update to Player Corpse" }));
    await user.click(screen.getByText("2.1.0 · Stable"));
    await user.click(screen.getByRole("radio", { name: /Survival/ }));
    await user.click(screen.getByRole("button", { name: "Add to instance" }));

    await waitFor(() =>
      expect(api.downloadBatch).toHaveBeenCalledWith({
        instanceId: "inst-1",
        targets: [{ modId: "51", versionId: "8" }],
      }),
    );
  });
});
