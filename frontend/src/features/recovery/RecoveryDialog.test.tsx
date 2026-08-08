// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useRecoveryStore } from "../../app/stores/recovery";
import { useToastStore } from "../../app/stores/toast";
import type { RecoverySuggestion } from "../../entities/last-known-good/model";
import { RecoveryDialog } from "./RecoveryDialog";

const api = vi.hoisted(() => ({
  restore: vi.fn(),
}));

vi.mock("../../shared/api/snapshots", () => ({ snapshotsApi: api }));

const suggestion: RecoverySuggestion = {
  instanceId: "instance-1",
  recordedAt: "2026-08-08T15:42:00Z",
  snapshotId: "snap-1",
  snapshotExists: true,
  stateSignature: "signature-1",
  changes: {
    updated: [{ name: "BetterRuins", from: "0.9.7", to: "0.9.8" }],
    added: [{ name: "Wildcraft", to: "1.8.0" }],
    removed: [{ name: "SmithingPlus", from: "2.4.1" }],
  },
};

function renderDialog() {
  const notify = vi.fn();
  useToastStore.setState({ notify });
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <RecoveryDialog />
    </QueryClientProvider>,
  );
  return { notify };
}

function show(suggestionValue = suggestion) {
  useRecoveryStore.getState().show(suggestionValue);
}

describe("recovery dialog", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    useRecoveryStore.setState({
      suggestion: undefined,
      restoring: false,
      dismissedSignatures: {},
    });
    vi.clearAllMocks();
    api.restore.mockResolvedValue(undefined);
  });

  it("reports the failed start and the changes since the last successful launch", async () => {
    renderDialog();
    show();
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("The game failed to start after recent changes.")).toBeTruthy();
    expect(within(dialog).getByText("Changes since the last successful launch:")).toBeTruthy();
    expect(within(dialog).getByText("Updated")).toBeTruthy();
    expect(within(dialog).getByText("BetterRuins")).toBeTruthy();
    expect(within(dialog).getByText("0.9.7 → 0.9.8")).toBeTruthy();
    expect(within(dialog).getByText("Added")).toBeTruthy();
    expect(within(dialog).getByText("Wildcraft")).toBeTruthy();
    expect(within(dialog).getByText("Removed")).toBeTruthy();
    expect(within(dialog).getByText("SmithingPlus")).toBeTruthy();
    expect(within(dialog).getByText(/Last successful launch:/)).toBeTruthy();
  });

  it("shows the game version change when it differs", async () => {
    renderDialog();
    show({
      ...suggestion,
      changes: {
        ...suggestion.changes,
        gameVersionFrom: "1.21.5",
        gameVersionTo: "1.21.6",
      },
    });
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Game version")).toBeTruthy();
    expect(within(dialog).getByText("Vintage Story")).toBeTruthy();
    expect(within(dialog).getByText("1.21.5 → 1.21.6")).toBeTruthy();
  });

  it("renders a game version only change without mod lists", async () => {
    // An empty instance has no mod changes to report; the dialog must render
    // the version transition without crashing on missing arrays.
    renderDialog();
    show({
      ...suggestion,
      changes: {
        updated: [],
        added: [],
        removed: [],
        gameVersionFrom: "1.21.5",
        gameVersionTo: "1.21.6",
      },
    });
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("1.21.5 → 1.21.6")).toBeTruthy();
    expect(within(dialog).queryByText("Updated")).toBeNull();
    expect(within(dialog).getByRole("button", { name: "Rollback" })).toBeTruthy();
  });

  it("tolerates a stale payload without mod lists", async () => {
    renderDialog();
    show({
      ...suggestion,
      changes: {
        gameVersionFrom: "1.21.5",
        gameVersionTo: "1.21.6",
      } as RecoverySuggestion["changes"],
    });
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("1.21.5 → 1.21.6")).toBeTruthy();
  });

  it("offers restore only when a recovery snapshot exists", async () => {
    renderDialog();
    show();
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByRole("button", { name: "Rollback" })).toBeTruthy();

    useRecoveryStore.setState({ suggestion: undefined });
    show({ ...suggestion, snapshotExists: false, snapshotId: undefined });
    const withoutSnapshot = await screen.findByRole("dialog");
    expect(within(withoutSnapshot).queryByRole("button", { name: "Rollback" })).toBeNull();
    expect(within(withoutSnapshot).getByText("No recovery snapshot is available")).toBeTruthy();
    expect(
      within(withoutSnapshot).getByText("Review the changes and adjust the instance manually."),
    ).toBeTruthy();
  });

  it("restores through the existing snapshot restore api", async () => {
    const { notify } = renderDialog();
    show();
    fireEvent.click(await screen.findByRole("button", { name: "Rollback" }));
    await waitFor(() => expect(api.restore).toHaveBeenCalledWith("instance-1", "snap-1"));
    await waitFor(() => expect(notify).toHaveBeenCalledWith("Instance restored from snapshot"));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("never rolls back without asking; keep current state acknowledges", async () => {
    renderDialog();
    show();
    fireEvent.click(await screen.findByRole("button", { name: "Keep current state" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(api.restore).not.toHaveBeenCalled();

    // The exact same failed state must not prompt again.
    show();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("shows the prompt again when the configuration changed materially", async () => {
    renderDialog();
    show();
    fireEvent.click(await screen.findByRole("button", { name: "Keep current state" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    show({ ...suggestion, stateSignature: "signature-2" });
    expect(await screen.findByRole("dialog")).toBeTruthy();
  });
});
