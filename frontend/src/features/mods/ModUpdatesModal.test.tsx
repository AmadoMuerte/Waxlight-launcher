// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useToastStore } from "../../app/stores/toast";
import type { InstanceModUpdateReport, ModVersion } from "../../entities/mod/model";
import { ModUpdatesModal } from "./ModUpdatesModal";

const api = vi.hoisted(() => ({ updateInstance: vi.fn(), upgradeVersions: vi.fn() }));
vi.mock("../../shared/api/mods", () => ({ modsApi: api }));

const report: InstanceModUpdateReport = {
  gameVersion: "1.20",
  mods: [
    {
      modId: "stonequarry",
      name: "Stone Quarry",
      installedVersion: "1.2.0",
      targetVersionId: "v2",
      targetVersion: "1.3.0",
      status: "update_available",
      reason: "",
      changelog: "",
      compatible: true,
      prerelease: false,
      addedDeps: [],
      removedDeps: [],
    },
    {
      modId: "oldmod",
      name: "Old Mod",
      installedVersion: "1.0.0",
      targetVersionId: "old-v2",
      targetVersion: "2.0.0",
      status: "update_available",
      reason: "",
      changelog: "",
      compatible: false,
      prerelease: false,
      addedDeps: [],
      removedDeps: [],
    },
  ],
  summary: {
    totalMods: 2,
    upToDate: 0,
    updatesAvailable: 2,
    notUpdatableLocal: 0,
    notUpdatableAbsent: 0,
    notUpdatableCatalogError: 0,
    incompatible: 1,
  },
};

function version(id: string, value: string, gameVersions = ["1.20"]): ModVersion {
  return {
    id,
    version: value,
    gameVersions,
    releaseType: "stable",
    fileName: `${id}.zip`,
    fileSize: 1,
  };
}

function renderModal(reportValue = report) {
  const props = {
    instanceId: "instance-1",
    instanceName: "Survival",
    report: reportValue,
    onClose: vi.fn(),
    onApplied: vi.fn().mockResolvedValue(undefined),
  };
  useToastStore.setState({ notify: vi.fn() });
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <ModUpdatesModal {...props} />
    </QueryClientProvider>,
  );
  return props;
}

beforeEach(() => {
  vi.clearAllMocks();
  api.upgradeVersions.mockImplementation((_: string, modId: string) =>
    Promise.resolve([
      version(
        modId === "oldmod" ? "old-v2" : "v2",
        modId === "oldmod" ? "2.0.0" : "1.3.0",
        modId === "oldmod" ? [] : ["1.20"],
      ),
    ]),
  );
  api.updateInstance.mockResolvedValue({ updated: 1, skippedByPolicy: 0 });
});
afterEach(cleanup);

describe("ModUpdatesModal", () => {
  it("waits for fresh eligibility lists", async () => {
    let resolve!: (versions: ModVersion[]) => void;
    api.upgradeVersions.mockImplementation(
      () =>
        new Promise((done) => {
          resolve = done;
        }),
    );
    renderModal({ ...report, mods: [report.mods[0]] });
    expect(screen.getByRole("button", { name: "Loading mods" }).hasAttribute("disabled")).toBe(
      true,
    );
    resolve([version("v2", "1.3.0")]);
    expect(await screen.findByText("Stone Quarry")).toBeTruthy();
  });

  it("offers only backend-approved upgrades, including major releases", async () => {
    api.upgradeVersions.mockResolvedValue([version("v2", "1.3.0"), version("v3", "3.0.0")]);
    renderModal({ ...report, mods: [report.mods[0]] });
    const user = userEvent.setup();
    await user.click(await screen.findByRole("combobox", { name: "Update to Stone Quarry" }));
    expect(screen.getByText("1.2.0 → 3.0.0 · Stable")).toBeTruthy();
    expect(screen.queryByText("1.2.0 → 1.1.0 · Stable")).toBeNull();
  });

  it("applies selected valid target in one batch", async () => {
    api.upgradeVersions.mockResolvedValue([version("v2", "1.3.0"), version("v3", "3.0.0")]);
    const props = renderModal({ ...report, mods: [report.mods[0]] });
    const user = userEvent.setup();
    await user.click(await screen.findByRole("combobox", { name: "Update to Stone Quarry" }));
    await user.click(screen.getByText("1.2.0 → 3.0.0 · Stable"));
    await user.click(screen.getByRole("button", { name: "Update 1 mod" }));
    await waitFor(() =>
      expect(api.updateInstance).toHaveBeenCalledWith({
        instanceId: "instance-1",
        mods: [{ modId: "stonequarry", versionId: "v3" }],
        allowIncompatible: false,
      }),
    );
    expect(props.onApplied).toHaveBeenCalledOnce();
  });

  it("does not submit stale report target after successful empty response", async () => {
    api.upgradeVersions.mockResolvedValue([]);
    renderModal({ ...report, mods: [report.mods[0]] });
    expect(await screen.findByText("Stone Quarry")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Update 0 mods" }).hasAttribute("disabled")).toBe(
      true,
    );
    expect(api.updateInstance).not.toHaveBeenCalled();
  });

  it("uses only report target when eligibility fetch fails", async () => {
    api.upgradeVersions.mockRejectedValue(new Error("offline"));
    renderModal({ ...report, mods: [report.mods[0]] });
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "Update 1 mod" }));
    await waitFor(() =>
      expect(api.updateInstance).toHaveBeenCalledWith(
        expect.objectContaining({ mods: [{ modId: "stonequarry", versionId: "v2" }] }),
      ),
    );
  });

  it("keeps incompatible and unchecked rows out of batch until allowed", async () => {
    renderModal();
    const user = userEvent.setup();
    await screen.findByText("Stone Quarry");
    expect(screen.getByRole("button", { name: "Update 1 mod" })).toBeTruthy();
    await user.click(screen.getByLabelText("Old Mod"));
    expect(screen.getByRole("button", { name: "Update 1 mod" })).toBeTruthy();
    await user.click(screen.getByLabelText(/allow updates/i));
    expect(screen.getByRole("button", { name: "Update 2 mods" })).toBeTruthy();
  });
});
