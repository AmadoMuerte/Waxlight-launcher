// @vitest-environment jsdom

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { DownloadedMod, GameVersion, Instance, InstalledMod } from "../../shared/api/types";
import { ModInstancesDialog } from "./ModInstancesDialog";

const api = vi.hoisted(() => ({
  list: vi.fn(),
  toggle: vi.fn(),
  previewDelete: vi.fn(),
  remove: vi.fn(),
}));
vi.mock("../../shared/api/mods", () => ({ modsApi: api }));

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
const instances: Instance[] = ["Survival", "Creative"].map((name, index) => ({
  id: `inst-${index + 1}`,
  name,
  description: "",
  gameVersionId: "1.20",
  gameClient: "vanilla",
  directory: "/data",
  status: "ready",
  launchArguments: [],
  environmentVariables: {},
  isPinned: false,
  createdAt: "2026-01-01T00:00:00Z",
  enabledModCount: 0,
  totalModCount: 0,
  playtimeSeconds: 0,
}));
const mod: DownloadedMod = {
  modId: "51",
  name: "Player Corpse",
  authorName: "Ada",
  side: "both",
  versionId: "7",
  downloadedVersion: "2.0.0",
  gameVersions: ["1.20"],
  fileName: "playercorpse.zip",
  fileSize: 100,
  downloadedAt: "2026-01-01T00:00:00Z",
  installedInstances: [],
  updateAvailable: false,
};
const installed: InstalledMod = {
  id: "installed-1",
  instanceId: "inst-1",
  name: "Player Corpse",
  version: "2.0.0",
  fileName: "playercorpse.zip",
  filePath: "/mods/playercorpse.zip",
  enabled: true,
  managed: true,
  source: "moddb:51:7",
  updatePolicy: "automatic",
  sizeBytes: 100,
  installedAt: "2026-01-01T00:00:00Z",
};

function renderDialog(confirmDeletion = true) {
  const onInstallRequest = vi.fn();
  const onMutated = vi.fn();
  render(
    <ModInstancesDialog
      mod={mod}
      instances={instances}
      gameVersions={versions}
      confirmDeletion={confirmDeletion}
      onClose={() => {}}
      onInstallRequest={onInstallRequest}
      onMutated={onMutated}
    />,
  );
  return { onInstallRequest, onMutated };
}

describe("ModInstancesDialog", () => {
  afterEach(cleanup);
  beforeEach(() => {
    api.list.mockReset();
    api.toggle.mockReset();
    api.previewDelete.mockReset();
    api.remove.mockReset();
  });

  it("matches installed mods by exact ModDB release source", async () => {
    api.list.mockImplementation((id: string) =>
      Promise.resolve(
        id === "inst-1"
          ? [
              { ...installed, name: "Wrong name" },
              { ...installed, id: "other", source: "moddb:51:8" },
            ]
          : [],
      ),
    );
    const { onInstallRequest } = renderDialog();
    await waitFor(() => expect(screen.getByText("Installed 2.0.0")).toBeTruthy());
    await userEvent.setup().click(screen.getByRole("button", { name: "Add to instance Creative" }));
    expect(onInstallRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        modId: mod.modId,
        versionId: mod.versionId,
        installedInstances: [
          { instanceId: "inst-1", instanceName: "Survival", version: "2.0.0", enabled: true },
        ],
      }),
      instances[1],
    );
  });

  it("toggles one row and reports successful mutation", async () => {
    api.list.mockResolvedValue([installed]);
    api.toggle.mockResolvedValue({ ...installed, enabled: false });
    const { onMutated } = renderDialog();
    await waitFor(() => expect(screen.getAllByText("Installed 2.0.0")).toHaveLength(2));
    await userEvent.setup().click(screen.getAllByRole("checkbox")[0]);
    await waitFor(() => expect(api.toggle).toHaveBeenCalledWith("installed-1", false));
    expect(onMutated).toHaveBeenCalledOnce();
  });

  it("serializes rapid row actions while a toggle is pending", async () => {
    let finishToggle!: (value: InstalledMod) => void;
    api.list.mockResolvedValue([installed]);
    api.toggle.mockReturnValue(
      new Promise((resolve) => {
        finishToggle = resolve;
      }),
    );
    const { onMutated } = renderDialog(false);
    await waitFor(() => expect(screen.getAllByText("Installed 2.0.0")).toHaveLength(2));
    const user = userEvent.setup();
    await user.click(screen.getAllByRole("checkbox")[0]);
    await user.click(screen.getByRole("button", { name: "Remove Survival" }));
    expect(api.toggle).toHaveBeenCalledTimes(1);
    expect(api.previewDelete).not.toHaveBeenCalled();
    expect(api.remove).not.toHaveBeenCalled();
    finishToggle({ ...installed, enabled: false });
    await waitFor(() => expect(onMutated).toHaveBeenCalledOnce());
  });

  it("removes without confirmation only when no dependencies and disabled preference", async () => {
    api.list.mockResolvedValue([installed]);
    api.previewDelete.mockResolvedValue({
      modId: installed.id,
      modName: installed.name,
      dependencies: [],
    });
    api.remove.mockResolvedValue(undefined);
    const { onMutated } = renderDialog(false);
    await waitFor(() => expect(screen.getAllByText("Installed 2.0.0")).toHaveLength(2));
    await userEvent.setup().click(screen.getByRole("button", { name: "Remove Survival" }));
    await waitFor(() => expect(api.remove).toHaveBeenCalledWith("installed-1", false));
    expect(onMutated).toHaveBeenCalledOnce();
  });

  it("passes current installed instances when adding after removal", async () => {
    const staleMod = {
      ...mod,
      installedInstances: [
        { instanceId: "inst-1", instanceName: "Survival", version: "2.0.0", enabled: true },
      ],
    };
    api.list.mockImplementation((id: string) =>
      Promise.resolve(id === "inst-1" ? [installed] : []),
    );
    api.previewDelete.mockResolvedValue({
      modId: installed.id,
      modName: installed.name,
      dependencies: [],
    });
    api.remove.mockResolvedValue(undefined);
    const onInstallRequest = vi.fn();
    render(
      <ModInstancesDialog
        mod={staleMod}
        instances={instances}
        gameVersions={versions}
        confirmDeletion={false}
        onClose={() => {}}
        onInstallRequest={onInstallRequest}
        onMutated={vi.fn()}
      />,
    );
    const user = userEvent.setup();
    await screen.findByText("Installed 2.0.0");
    await user.click(screen.getByRole("button", { name: "Remove Survival" }));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Add to instance Survival" })).toBeTruthy(),
    );
    await user.click(screen.getByRole("button", { name: "Add to instance Survival" }));
    expect(onInstallRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        modId: mod.modId,
        versionId: mod.versionId,
        installedInstances: [],
      }),
      instances[0],
    );
  });

  it("keeps installed instances first after removal", async () => {
    api.list.mockImplementation((id: string) =>
      Promise.resolve(
        id === "inst-1" ? [installed] : [{ ...installed, id: "installed-2", instanceId: id }],
      ),
    );
    api.previewDelete.mockResolvedValue({
      modId: installed.id,
      modName: installed.name,
      dependencies: [],
    });
    api.remove.mockResolvedValue(undefined);
    renderDialog(false);
    await screen.findAllByText("Installed 2.0.0");
    await userEvent.setup().click(screen.getByRole("button", { name: "Remove Survival" }));
    await waitFor(() =>
      expect(
        [...document.querySelectorAll("article strong")].map((element) => element.textContent),
      ).toEqual(["Creative", "Survival"]),
    );
  });

  it("always offers dependency removal choices", async () => {
    api.list.mockResolvedValue([installed]);
    api.previewDelete.mockResolvedValue({
      modId: installed.id,
      modName: installed.name,
      dependencies: [{ ...installed, id: "dep", name: "Dependency" }],
    });
    renderDialog(false);
    await waitFor(() => expect(screen.getAllByText("Installed 2.0.0")).toHaveLength(2));
    await userEvent.setup().click(screen.getByRole("button", { name: "Remove Survival" }));
    expect(await screen.findByRole("button", { name: "Only the mod" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Mod and dependencies" })).toBeTruthy();
    expect(api.remove).not.toHaveBeenCalled();
  });
});
