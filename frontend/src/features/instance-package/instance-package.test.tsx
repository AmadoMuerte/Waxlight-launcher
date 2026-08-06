// @vitest-environment jsdom

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { PackageInspection } from "../../shared/api";
import { ExportInstanceModal } from "./ExportInstanceModal";
import { ImportPackageModal } from "./ImportPackageModal";

const api = vi.hoisted(() => ({
  export: vi.fn(),
  inspect: vi.fn(),
  import: vi.fn(),
  selectExportPath: vi.fn(),
  selectPackageFile: vi.fn(),
}));

vi.mock("../../shared/api", () => ({ instancePackageApi: api }));

class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}

globalThis.ResizeObserver = globalThis.ResizeObserver ?? ResizeObserverStub;

const inspection: PackageInspection = {
  path: "/tmp/share.waxlight",
  schemaVersion: 1,
  name: "Cozy Camp",
  description: "A warm place to start.",
  author: {
    name: "Ada",
    homepage: "https://example.com/ada",
    source: "https://github.com/ada/mods",
  },
  gameVersion: { id: "1.20", name: "1.20" },
  versionStatus: "installed",
  launchArguments: [],
  mods: [
    {
      modId: "51",
      versionId: "7",
      name: "Player Corpse",
      version: "2.0.0",
      source: "moddb",
      enabled: true,
      status: "available",
    },
    {
      name: "Local Helper",
      version: "1.0.0",
      source: "embedded",
      enabled: true,
      status: "embedded",
    },
  ],
  configFiles: ["Config/x.json", "clientsettings.json"],
  hasIcon: true,
  totalSize: 2048,
  unverifiedFiles: 0,
  warnings: [],
};

const instance = {
  id: "instance-1",
  name: "My World",
  description: "My cozy world",
  gameVersionId: "1.20",
  directory: "/data",
  status: "ready",
  launchArguments: [],
  createdAt: "2026-01-01T00:00:00Z",
  enabledModCount: 1,
  totalModCount: 1,
  playtimeSeconds: 0,
};

describe("instance package modals", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    api.export.mockResolvedValue({ name: "My World" });
    api.import.mockResolvedValue({
      instanceId: "instance-9",
      instanceName: "Cozy Camp",
      gameVersionId: "1.20",
      mods: [
        { name: "Player Corpse", version: "2.0.0", status: "installed" },
        { name: "Local Helper", version: "1.0.0", status: "installed" },
      ],
      warnings: [],
    });
  });

  it("previews the package and imports it into a new instance", async () => {
    const onDone = vi.fn();
    render(
      <ImportPackageModal
        inspection={inspection}
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
        onClose={() => {}}
        onDone={onDone}
        onBackgroundDone={vi.fn().mockResolvedValue(undefined)}
        notify={vi.fn()}
      />,
    );

    expect(await screen.findByRole("dialog", { name: "Import instance" })).toBeTruthy();
    expect(screen.getByText("Cozy Camp")).toBeTruthy();
    expect(screen.getByText("Player Corpse")).toBeTruthy();
    expect(screen.getByText("Local Helper")).toBeTruthy();
    expect(screen.getByText("Downloaded from the catalog")).toBeTruthy();
    expect(screen.getByText("Installed from local files")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Author page (URL) ↗" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Source code (URL) ↗" })).toBeTruthy();

    await userEvent.setup().click(screen.getByRole("button", { name: "Import" }));

    await waitFor(() => {
      expect(api.import).toHaveBeenCalledWith(
        expect.objectContaining({
          packagePath: "/tmp/share.waxlight",
          name: "Cozy Camp",
          gameVersionId: "",
        }),
      );
    });
    expect(onDone).toHaveBeenCalledWith(expect.objectContaining({ instanceId: "instance-9" }));
  });

  it("closes immediately and imports in the background when the game must be downloaded", async () => {
    const onClose = vi.fn();
    const onBackgroundDone = vi.fn().mockResolvedValue(undefined);
    const onDone = vi.fn();
    const notify = vi.fn();

    render(
      <ImportPackageModal
        inspection={{ ...inspection, versionStatus: "available" }}
        versions={[]}
        onClose={onClose}
        onDone={onDone}
        onBackgroundDone={onBackgroundDone}
        notify={notify}
      />,
    );

    expect(await screen.findByRole("dialog", { name: "Import instance" })).toBeTruthy();

    await userEvent.setup().click(screen.getByRole("button", { name: "Import" }));

    await waitFor(() => {
      expect(api.import).toHaveBeenCalledWith(
        expect.objectContaining({
          installVersion: true,
          gameVersionId: "",
        }),
      );
    });

    expect(onClose).toHaveBeenCalled();
    expect(notify).toHaveBeenCalledWith(
      "The instance will be available once the game version finishes downloading.",
      "ok",
      5_000,
    );
    expect(onDone).not.toHaveBeenCalled();

    await waitFor(() => {
      expect(onBackgroundDone).toHaveBeenCalled();
    });
  });

  it("exports an instance after choosing a save path", async () => {
    api.selectExportPath.mockResolvedValue("/tmp/My World.waxlight");
    const onDone = vi.fn();

    render(
      <ExportInstanceModal
        instance={instance}
        onClose={() => {}}
        onDone={onDone}
        notify={vi.fn()}
      />,
    );

    expect(await screen.findByRole("dialog", { name: "Export instance" })).toBeTruthy();

    await userEvent.setup().click(screen.getByRole("button", { name: "Export" }));

    await waitFor(() => {
      expect(api.selectExportPath).toHaveBeenCalledWith("My World.waxlight");
    });
    await waitFor(() => {
      expect(api.export).toHaveBeenCalledWith(
        expect.objectContaining({
          instanceId: "instance-1",
          targetPath: "/tmp/My World.waxlight",
          name: "My World",
        }),
      );
    });
    expect(onDone).toHaveBeenCalled();
  });

  it("blocks export until author links are valid URLs", async () => {
    api.selectExportPath.mockResolvedValue("/tmp/My World.waxlight");
    const onDone = vi.fn();

    render(
      <ExportInstanceModal
        instance={instance}
        onClose={() => {}}
        onDone={onDone}
        notify={vi.fn()}
      />,
    );

    expect(await screen.findByRole("dialog", { name: "Export instance" })).toBeTruthy();

    const homepageInput = screen.getByLabelText("Author page (URL)");
    await userEvent.setup().type(homepageInput, "not-a-url");

    expect(screen.getByRole("button", { name: "Export" })).toHaveProperty("disabled", true);
    expect(screen.getByRole("alert")).toBeTruthy();

    await userEvent.setup().clear(homepageInput);
    await userEvent.setup().type(homepageInput, "https://example.com");

    expect(screen.getByRole("button", { name: "Export" })).toHaveProperty("disabled", false);

    await userEvent.setup().click(screen.getByRole("button", { name: "Export" }));

    await waitFor(() => {
      expect(api.export).toHaveBeenCalledWith(
        expect.objectContaining({
          targetPath: "/tmp/My World.waxlight",
          author: expect.objectContaining({ homepage: "https://example.com" }),
        }),
      );
    });
  });
});
