// @vitest-environment jsdom

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AvailableVersions } from "./AvailableVersions";

const api = vi.hoisted(() => ({
  available: vi.fn(),
  installAvailable: vi.fn(),
}));

vi.mock("../../shared/api", () => ({ versionsApi: api }));

const releases = [
  {
    id: "1.22.6",
    name: "1.22.6",
    channel: "stable",
    platform: "linux",
    architecture: "amd64",
    downloadSize: 590_500_000,
    latest: true,
    installed: false,
  },
  {
    id: "1.23.0-pre.1",
    name: "1.23.0-pre.1",
    channel: "unstable",
    platform: "linux",
    architecture: "amd64",
    downloadSize: 600_000_000,
    latest: true,
    installed: false,
  },
];

describe("official version catalog", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    api.available.mockResolvedValue(releases);
    api.installAvailable.mockResolvedValue({ id: "operation" });
  });

  it("shows stable releases by default and starts a download", async () => {
    const notify = vi.fn();
    const onOperationStarted = vi.fn().mockResolvedValue(undefined);
    render(
      <AvailableVersions
        installedVersionIDs={[]}
        notify={notify}
        onOperationStarted={onOperationStarted}
      />,
    );

    expect(await screen.findByText("1.22.6")).toBeTruthy();
    expect(screen.queryByText("1.23.0-pre.1")).toBeNull();
    await userEvent.setup().click(screen.getByRole("button", { name: "Download" }));

    expect(api.installAvailable).toHaveBeenCalledWith("1.22.6");
    await waitFor(() => expect(onOperationStarted).toHaveBeenCalled());
    expect(notify).toHaveBeenCalledWith("Downloading Vintage Story 1.22.6");
  });

  it("filters preview releases and disables installed versions", async () => {
    render(
      <AvailableVersions
        installedVersionIDs={["1.23.0-pre.1"]}
        notify={vi.fn()}
        onOperationStarted={vi.fn().mockResolvedValue(undefined)}
      />,
    );
    await screen.findByText("1.22.6");
    const user = userEvent.setup();
    await user.click(screen.getByLabelText("Release channel"));
    await user.click(screen.getByRole("option", { name: /Preview/ }));

    expect(await screen.findByText("1.23.0-pre.1")).toBeTruthy();
    const installedButton = screen.getByRole("button", { name: "Installed" });
    if (!(installedButton instanceof HTMLButtonElement))
      throw new Error("installed control is not a button");
    expect(installedButton.disabled).toBe(true);
  });
});
