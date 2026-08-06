// @vitest-environment jsdom

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { InstanceModUpdateReport } from "../../shared/api";
import { ModUpdatesModal } from "./ModUpdatesModal";

const api = vi.hoisted(() => ({
  download: vi.fn(),
}));

vi.mock("../../shared/api", () => ({ modCatalogApi: api }));

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
      changelog: "Fixed a crash.",
      compatible: true,
      prerelease: false,
      addedDeps: [],
      removedDeps: [],
    },
    {
      modId: "oldmod",
      name: "Old Mod",
      installedVersion: "1.0.0",
      targetVersionId: "v2",
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

function renderModal(reportValue: InstanceModUpdateReport = report) {
  const props = {
    instanceId: "instance-1",
    instanceName: "Survival",
    report: reportValue,
    onClose: vi.fn(),
    onApplied: vi.fn().mockResolvedValue(undefined),
    notify: vi.fn(),
  };
  render(<ModUpdatesModal {...props} />);
  return props;
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("ModUpdatesModal", () => {
  it("applies compatible updates on confirm", async () => {
    api.download.mockResolvedValue({});
    const user = userEvent.setup();
    const props = renderModal();

    expect(screen.getByText("Stone Quarry")).toBeTruthy();
    expect(screen.getByText("1.2.0 → 1.3.0")).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "Update 1 mod" }));
    await waitFor(() => expect(api.download).toHaveBeenCalledTimes(1));
    expect(api.download).toHaveBeenCalledWith({
      modId: "stonequarry",
      versionId: "v2",
      instanceIds: ["instance-1"],
      downloadOnly: false,
      allowIncompatible: false,
    });
    expect(props.onApplied).toHaveBeenCalledTimes(1);
    expect(props.onClose).toHaveBeenCalledTimes(1);
    expect(props.notify).toHaveBeenCalledWith("Mods updated");
  });

  it("keeps the apply button disabled until incompatible updates are allowed", async () => {
    const user = userEvent.setup();
    const onlyIncompatible: InstanceModUpdateReport = {
      ...report,
      mods: [report.mods[1]],
      summary: { ...report.summary, updatesAvailable: 1, incompatible: 1 },
    };
    renderModal(onlyIncompatible);

    const apply = screen.getByRole("button", { name: /update/i }) as HTMLButtonElement;
    expect(apply.disabled).toBe(true);

    await user.click(screen.getByLabelText(/allow updates/i));
    expect(apply.disabled).toBe(false);
  });
});
