// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { LastKnownGood } from "../../entities/last-known-good/model";
import { LastKnownGoodSection } from "./LastKnownGoodSection";

const api = vi.hoisted(() => ({
  get: vi.fn(),
}));

vi.mock("../../shared/api/last-known-good", () => ({ lastKnownGoodApi: api }));

const matching: LastKnownGood = {
  recordedAt: "2026-08-08T15:42:00Z",
  gameVersion: "1.21.6",
  modCount: 84,
  snapshotId: "snap-1",
  snapshotExists: true,
  matchesCurrent: true,
  changeCount: 0,
  changes: { updated: [], added: [], removed: [] },
};

const changed: LastKnownGood = {
  ...matching,
  matchesCurrent: false,
  changeCount: 3,
  changes: {
    gameVersionFrom: "1.21.5",
    gameVersionTo: "1.21.6",
    updated: [{ name: "BetterRuins", from: "0.9.7", to: "0.9.8" }],
    added: [{ name: "Wildcraft", to: "1.8.0" }],
    removed: [{ name: "SmithingPlus", from: "2.4.1" }],
  },
};

function renderSection() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <LastKnownGoodSection instanceId="instance-1" />
    </QueryClientProvider>,
  );
}

describe("last known good section", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("hides entirely when no marker was recorded yet", async () => {
    // The real backend returns the zero status with an empty recordedAt; the
    // section must not render a fake "changes" state for it.
    api.get.mockResolvedValue({
      recordedAt: "",
      gameVersion: "",
      modCount: 0,
      snapshotExists: false,
      matchesCurrent: false,
      changeCount: 0,
      changes: { updated: [], added: [], removed: [] },
    });
    renderSection();
    await waitFor(() => expect(api.get).toHaveBeenCalledWith("instance-1"));
    expect(screen.queryByText("Last known good")).toBeNull();
  });

  it("handles a game version change without any mod changes", async () => {
    api.get.mockResolvedValue({
      ...changed,
      changeCount: 1,
      changes: {
        updated: [],
        added: [],
        removed: [],
        gameVersionFrom: "1.21.5",
        gameVersionTo: "1.21.6",
      },
    });
    renderSection();
    expect(await screen.findByText("1 change since the last successful launch.")).toBeTruthy();

    await userEvent.setup().click(screen.getByRole("button", { name: "View changes" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("1.21.5 → 1.21.6")).toBeTruthy();
  });

  it("reports that the current configuration matches", async () => {
    api.get.mockResolvedValue(matching);
    renderSection();
    expect(
      await screen.findByText("Current configuration matches the last successful launch."),
    ).toBeTruthy();
    expect(screen.getByText(/Vintage Story 1\.21\.6/)).toBeTruthy();
    expect(screen.getByText(/84 mods/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: "View changes" })).toBeNull();
  });

  it("counts the changes and opens the details dialog", async () => {
    api.get.mockResolvedValue(changed);
    renderSection();
    expect(await screen.findByText("3 changes since the last successful launch.")).toBeTruthy();

    await userEvent.setup().click(screen.getByRole("button", { name: "View changes" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Changes since last successful launch")).toBeTruthy();
    expect(within(dialog).getByText(/Last successful launch:/)).toBeTruthy();
    expect(within(dialog).getByText("Game version")).toBeTruthy();
    expect(within(dialog).getByText("1.21.5 → 1.21.6")).toBeTruthy();
    expect(within(dialog).getByText("Updated")).toBeTruthy();
    expect(within(dialog).getByText("BetterRuins")).toBeTruthy();
    expect(within(dialog).getByText("Added")).toBeTruthy();
    expect(within(dialog).getByText("Wildcraft")).toBeTruthy();
    expect(within(dialog).getByText("Removed")).toBeTruthy();
    expect(within(dialog).getByText("SmithingPlus")).toBeTruthy();

    const closeButton = within(dialog)
      .getAllByRole("button", { name: "Close" })
      .find((button) => button.className.includes("ghost"));
    expect(closeButton).toBeTruthy();
    await userEvent.setup().click(closeButton!);
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
