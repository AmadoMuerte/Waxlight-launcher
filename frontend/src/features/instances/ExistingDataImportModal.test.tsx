// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { GameVersion } from "../../entities/game-version/model";
import type { Instance } from "../../entities/instance/model";
import { OPERATIONS_QUERY_KEY } from "../../shared/api/keys";
import type { InstalledMod, MigrationCandidate, Operation } from "../../shared/api/types";
import { ExistingDataImportModal } from "./ExistingDataImportModal";

const instancesApi = vi.hoisted(() => ({
  detectExistingData: vi.fn(),
  inspectExistingData: vi.fn(),
  importExistingData: vi.fn(),
  get: vi.fn(),
}));
const modsApi = vi.hoisted(() => ({ list: vi.fn() }));
const operationsApi = vi.hoisted(() => ({ list: vi.fn(), cancel: vi.fn() }));
const settingsApi = vi.hoisted(() => ({ selectGameDirectory: vi.fn() }));

vi.mock("../../shared/api/instances", () => ({ instancesApi }));
vi.mock("../../shared/api/mods", () => ({ modsApi }));
vi.mock("../../shared/api/operations", () => ({ operationsApi }));
vi.mock("../../shared/api/settings", () => ({ settingsApi }));

const versions: GameVersion[] = [
  {
    id: "managed-1.20",
    name: "1.20.0",
    channel: "stable",
    platform: "linux",
    architecture: "amd64",
    installationDir: "/versions/1.20",
    executablePath: "/versions/1.20/Vintagestory",
    status: "installed",
    sizeBytes: 1,
    installedAt: "2026-01-01T00:00:00Z",
  },
];

const candidate: MigrationCandidate = {
  path: "/home/player/.config/VintagestoryData",
  worldCount: 2,
  modCount: 3,
  totalBytes: 2_048,
  totalFiles: 12,
  hasClientSettings: true,
  hasModConfig: false,
  detectedGameVersion: "1.20.0",
  versionConfidence: "high",
  warnings: ["One save needs repair"],
};

const operation: Operation = {
  id: "import-1",
  type: "existing_data_import",
  title: "Import existing data",
  status: "queued",
  progress: 0,
  currentBytes: 0,
  totalBytes: 2_048,
  bytesPerSecond: 0,
  createdAt: "2026-01-01T00:00:00Z",
};

const importedInstance: Instance = {
  id: "instance-1",
  name: "VintagestoryData",
  description: "",
  gameVersionId: "managed-1.20",
  gameClient: "vanilla",
  directory: "/instances/instance-1",
  status: "ready",
  launchArguments: [],
  environmentVariables: {},
  isPinned: false,
  createdAt: "2026-01-01T00:00:00Z",
  enabledModCount: 2,
  totalModCount: 2,
  playtimeSeconds: 0,
};

function mod(id: string, managed: boolean): InstalledMod {
  return {
    id,
    instanceId: importedInstance.id,
    name: id,
    version: "1.0.0",
    fileName: `${id}.zip`,
    filePath: `/mods/${id}.zip`,
    enabled: true,
    managed,
    source: managed ? "moddb" : "local",
    updatePolicy: "automatic",
    sizeBytes: 1,
    installedAt: "2026-01-01T00:00:00Z",
  };
}

function renderModal(overrides: { onOpenInstance?: (instance: Instance) => void } = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <ExistingDataImportModal
        versions={versions}
        onClose={vi.fn()}
        onOpenVersions={vi.fn()}
        onOpenInstance={overrides.onOpenInstance ?? vi.fn()}
      />
    </QueryClientProvider>,
  );
  return queryClient;
}

async function openSummary() {
  await userEvent
    .setup()
    .click(await screen.findByRole("button", { name: new RegExp(candidate.path) }));
  await screen.findByText("One save needs repair");
}

describe("ExistingDataImportModal", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    instancesApi.detectExistingData.mockResolvedValue([candidate]);
    instancesApi.inspectExistingData.mockResolvedValue(candidate);
    instancesApi.importExistingData.mockResolvedValue(operation);
    operationsApi.list.mockResolvedValue([]);
    operationsApi.cancel.mockResolvedValue(undefined);
  });

  it("keeps manual inspection available when detection finds no candidates", async () => {
    instancesApi.detectExistingData.mockResolvedValue([]);
    settingsApi.selectGameDirectory.mockResolvedValue("/manual/data");
    instancesApi.inspectExistingData.mockResolvedValue({ ...candidate, path: "/manual/data" });
    renderModal();

    expect(await screen.findByText(/No existing Vintage Story data was detected/)).toBeTruthy();
    await userEvent.setup().click(screen.getByRole("button", { name: "Choose data folder" }));

    await screen.findByText("/manual/data");
    expect(settingsApi.selectGameDirectory).toHaveBeenCalledOnce();
    expect(instancesApi.inspectExistingData).toHaveBeenCalledWith("/manual/data");
  });

  it("inspects any detected candidate and allows an installed version override", async () => {
    const second = { ...candidate, path: "/other/data", detectedGameVersion: "1.19.8" };
    instancesApi.detectExistingData.mockResolvedValue([candidate, second]);
    instancesApi.inspectExistingData.mockResolvedValue(second);
    renderModal();

    await userEvent.setup().click(await screen.findByRole("button", { name: /\/other\/data/ }));

    expect(await screen.findByText("1.19.8")).toBeTruthy();
    expect(screen.getByText("2.0 KB · 12 files")).toBeTruthy();
    expect(screen.getByText(/Vintage Story 1.19.8 is not installed/)).toBeTruthy();

    await userEvent.setup().click(screen.getByRole("combobox"));
    await userEvent.setup().click(await screen.findByRole("option", { name: "1.20.0" }));
    await userEvent.setup().clear(screen.getByLabelText("Name"));
    await userEvent.setup().type(screen.getByLabelText("Name"), "Imported home");
    await userEvent.setup().click(screen.getByRole("button", { name: "Start import" }));

    await waitFor(() =>
      expect(instancesApi.importExistingData).toHaveBeenCalledWith({
        sourcePath: "/other/data",
        name: "Imported home",
        description: "",
        gameVersionId: "managed-1.20",
      }),
    );
  });

  it("cancels the tracked import operation", async () => {
    renderModal();
    await openSummary();
    await userEvent.setup().click(screen.getByRole("button", { name: "Start import" }));
    await userEvent.setup().click(await screen.findByRole("button", { name: "Cancel import" }));

    await waitFor(() => expect(operationsApi.cancel).toHaveBeenCalledWith("import-1"));
    expect(await screen.findByText("The import was cancelled.")).toBeTruthy();
  });

  it("uses the completed operation resource to show results and open the instance", async () => {
    const onOpenInstance = vi.fn();
    const queryClient = renderModal({ onOpenInstance });
    instancesApi.get.mockResolvedValue(importedInstance);
    modsApi.list.mockResolvedValue([mod("linked", true), mod("local", false)]);
    await openSummary();
    await userEvent.setup().click(screen.getByRole("button", { name: "Start import" }));

    queryClient.setQueryData<Operation[]>(OPERATIONS_QUERY_KEY, [
      { ...operation, status: "running", progress: 0.42 },
    ]);
    expect(await screen.findByText("42% complete")).toBeTruthy();

    queryClient.setQueryData<Operation[]>(OPERATIONS_QUERY_KEY, [
      { ...operation, status: "completed", progress: 1, resourceId: importedInstance.id },
    ]);

    expect(await screen.findByText("Mods imported: 2 · Linked: 1 · Local: 1")).toBeTruthy();
    expect(screen.getByText("Your original data was left untouched.")).toBeTruthy();
    expect(instancesApi.get).toHaveBeenCalledWith(importedInstance.id);
    expect(modsApi.list).toHaveBeenCalledWith(importedInstance.id);

    await userEvent.setup().click(screen.getByRole("button", { name: "Open instance" }));
    expect(onOpenInstance).toHaveBeenCalledWith(importedInstance);
  });
});
