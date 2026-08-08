// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useToastStore } from "../../app/stores/toast";
import type { InstanceSnapshot } from "../../entities/snapshot/model";
import { BackupsTab } from "./BackupsTab";

const api = vi.hoisted(() => ({
  create: vi.fn(),
  list: vi.fn(),
  restore: vi.fn(),
  remove: vi.fn(),
}));

vi.mock("../../shared/api/snapshots", () => ({ snapshotsApi: api }));

const snapshot: InstanceSnapshot = {
  id: "snap-1",
  instanceId: "instance-1",
  instanceName: "Survival",
  type: "manual",
  gameVersion: "1.20",
  createdAt: "2026-08-08T13:24:00Z",
  sizeBytes: 2576980378,
  modCount: 84,
  worldCount: 1,
};

function renderTab(onRestored = vi.fn(), listed: InstanceSnapshot[] = [], onCreated = vi.fn()) {
  const notify = vi.fn();
  useToastStore.setState({ notify });
  api.list.mockResolvedValue(listed);
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <BackupsTab instanceId="instance-1" onCreated={onCreated} onRestored={onRestored} />
    </QueryClientProvider>,
  );
  return { notify, onRestored, onCreated };
}

describe("backups tab", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    api.list.mockResolvedValue([]);
    api.create.mockResolvedValue({});
    api.restore.mockResolvedValue(undefined);
    api.remove.mockResolvedValue(undefined);
  });

  it("shows the empty state when there are no snapshots", async () => {
    renderTab();
    expect(await screen.findByText("No backups yet")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Create snapshot" })).toBeTruthy();
  });

  it("creates a snapshot and shows it in the list", async () => {
    const { notify, onCreated } = renderTab(vi.fn(), [snapshot]);

    await userEvent.setup().click(screen.getByRole("button", { name: "＋ Create snapshot" }));

    await waitFor(() => expect(api.create).toHaveBeenCalledWith("instance-1"));
    // The host navigates to the Operations page immediately so the running
    // snapshot operation can be watched there.
    expect(onCreated).toHaveBeenCalledTimes(1);
    expect(await screen.findByText("Manual snapshot")).toBeTruthy();
    expect(screen.getByText(/Vintage Story 1\.20/)).toBeTruthy();
    expect(screen.getByText(/84 mods/)).toBeTruthy();
    expect(screen.getByText(/2\.4 GB/)).toBeTruthy();
    expect(notify).toHaveBeenCalledWith("Snapshot created");
  });

  it("lists existing snapshots on load", async () => {
    renderTab(vi.fn(), [snapshot]);
    expect(await screen.findByText("Manual snapshot")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Restore" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Delete" })).toBeTruthy();
  });

  it("requires confirmation before restoring a snapshot", async () => {
    const { onRestored } = renderTab(vi.fn(), [snapshot]);
    await userEvent.setup().click(await screen.findByRole("button", { name: "Restore" }));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Restore this snapshot?")).toBeTruthy();
    expect(api.restore).not.toHaveBeenCalled();

    await userEvent.setup().click(within(dialog).getByRole("button", { name: "Restore" }));
    await waitFor(() => expect(api.restore).toHaveBeenCalledWith("instance-1", "snap-1"));
    await waitFor(() => expect(onRestored).toHaveBeenCalled());
  });

  it("requires confirmation before deleting a snapshot", async () => {
    const { notify } = renderTab(vi.fn(), [snapshot]);
    await userEvent.setup().click(await screen.findByRole("button", { name: "Delete" }));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Delete snapshot?")).toBeTruthy();
    expect(api.remove).not.toHaveBeenCalled();

    await userEvent.setup().click(within(dialog).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(api.remove).toHaveBeenCalledWith("instance-1", "snap-1"));
    await waitFor(() => expect(notify).toHaveBeenCalledWith("Snapshot deleted"));
  });

  it("shows a busy state while creating and blocks duplicate creates", async () => {
    let resolveCreate: (value: unknown) => void;
    api.create.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveCreate = resolve;
        }),
    );
    renderTab();

    await userEvent.setup().click(screen.getByRole("button", { name: "＋ Create snapshot" }));

    expect(await screen.findByText("Creating snapshot…")).toBeTruthy();
    expect(api.create).toHaveBeenCalledTimes(1);

    resolveCreate!({});
    await waitFor(() => expect(api.create).toHaveBeenCalledTimes(1));
  });
});
