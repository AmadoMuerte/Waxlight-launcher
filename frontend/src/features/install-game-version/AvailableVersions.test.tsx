// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useToastStore } from "../../app/stores/toast";
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

function renderCatalog(installed: string[] = []) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <AvailableVersions installedVersionIDs={installed} />
    </QueryClientProvider>,
  );
}

describe("official version catalog", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    api.available.mockResolvedValue(releases);
    api.installAvailable.mockResolvedValue({ id: "operation" });
  });

  it("shows stable releases by default and starts a download", async () => {
    const notify = vi.fn();
    useToastStore.setState({ notify });
    renderCatalog();

    expect(await screen.findByText("1.22.6")).toBeTruthy();
    expect(screen.queryByText("1.23.0-pre.1")).toBeNull();
    await userEvent.setup().click(screen.getByRole("button", { name: "Download" }));

    expect(api.installAvailable).toHaveBeenCalledWith("1.22.6");
    await waitFor(() => expect(notify).toHaveBeenCalled());
    expect(notify).toHaveBeenCalledWith("Downloading Vintage Story 1.22.6");
  });

  it("filters preview releases and disables installed versions", async () => {
    renderCatalog(["1.23.0-pre.1"]);
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
