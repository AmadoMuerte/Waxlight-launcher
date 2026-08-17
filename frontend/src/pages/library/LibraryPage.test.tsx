// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { GameVersion } from "../../entities/game-version/model";
import { LibraryPage } from "./LibraryPage";

const api = vi.hoisted(() => ({
  create: vi.fn(),
  clone: vi.fn(),
  list: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
}));

const versionsApi = vi.hoisted(() => ({ list: vi.fn() }));
const accountsApi = vi.hoisted(() => ({ list: vi.fn() }));
const instancePackageApi = vi.hoisted(() => ({ import: vi.fn(), selectPackageFile: vi.fn() }));
const launcherApi = vi.hoisted(() => ({ validate: vi.fn(), launch: vi.fn(), stop: vi.fn() }));
const modsApi = vi.hoisted(() => ({ checkInstanceUpdates: vi.fn() }));
const settingsApi = vi.hoisted(() => ({ get: vi.fn(), openDirectory: vi.fn() }));

vi.mock("../../shared/api/instances", () => ({ instancesApi: api }));
vi.mock("../../shared/api/game-versions", () => ({ versionsApi }));
vi.mock("../../shared/api/accounts", () => ({ accountsApi }));
vi.mock("../../shared/api/launcher", () => ({ launcherApi }));
vi.mock("../../shared/api/mods", () => ({ modsApi }));
vi.mock("../../shared/api/settings", () => ({ settingsApi }));
vi.mock("../../shared/api/instance-package", () => ({ instancePackageApi }));

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

async function renderPage(data = instances) {
  api.list.mockResolvedValue(data);
  versionsApi.list.mockResolvedValue(versions);
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
  await screen.findByRole("button", { name: "New instance" });
}

describe("library instance creation", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    api.create.mockResolvedValue({});
    api.clone.mockResolvedValue({ id: "inst-2", name: "Warm home copy" });
    api.remove.mockResolvedValue(undefined);
    settingsApi.get.mockResolvedValue({ confirmDeletion: true });
    instancePackageApi.selectPackageFile.mockResolvedValue("/tmp/cozy-camp.waxlight");
    instancePackageApi.import.mockResolvedValue({ id: "operation-1", status: "queued" });
  });

  it("submits an empty name so the backend generates a unique default", async () => {
    await renderPage();

    await userEvent.setup().click(screen.getByRole("button", { name: "New instance" }));
    await userEvent.setup().click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(api.create).toHaveBeenCalledWith(
        expect.objectContaining({ name: "", gameVersionId: "1.20" }),
      );
    });
  });

  it("submits the typed name when provided", async () => {
    await renderPage();

    await userEvent.setup().click(screen.getByRole("button", { name: "New instance" }));
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

    await userEvent.setup().click(screen.getByRole("button", { name: /Import instance/ }));

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

  it("shows create and import actions when the Library is empty", async () => {
    await renderPage([]);

    expect(screen.getByRole("heading", { name: "Light your first world" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Create instance" })).toBeTruthy();
    expect(screen.getAllByRole("button", { name: "Import instance" })).toHaveLength(2);
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
});
