// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useToastStore } from "../../app/stores/toast";
import { VersionsPage } from "./VersionsPage";

const api = vi.hoisted(() => ({
  list: vi.fn(),
  available: vi.fn(),
  installAvailable: vi.fn(),
  remove: vi.fn(),
}));

const settingsQuery = vi.hoisted(() => ({ useSettingsQuery: vi.fn() }));

vi.mock("../../shared/api/game-versions", () => ({ versionsApi: api }));
vi.mock("../../entities/settings/queries", () => settingsQuery);

const installedVersion = {
  id: "1.20",
  name: "1.20",
  channel: "stable",
  platform: "linux",
  architecture: "amd64",
  installationDir: "/game",
  executablePath: "/game/Vintagestory",
  status: "installed",
  sizeBytes: 100,
  installedAt: "2026-01-01T00:00:00Z",
};

function renderPage() {
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
      <VersionsPage />
    </QueryClientProvider>,
  );
  return { notify };
}

describe("confirmDeletion gate", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    settingsQuery.useSettingsQuery.mockReturnValue({ data: undefined });
    api.list.mockResolvedValue([installedVersion]);
    api.available.mockResolvedValue([]);
    api.remove.mockResolvedValue(undefined);
  });

  it("removes a version directly when confirmDeletion is false", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: { confirmDeletion: false } });
    const user = userEvent.setup();
    const { notify } = renderPage();
    await screen.findByRole("button", { name: "Remove" });

    await user.click(screen.getByRole("button", { name: "Remove" }));

    await waitFor(() => expect(api.remove).toHaveBeenCalledWith("1.20", true));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(notify).toHaveBeenCalledWith("Game version removed");
  });

  it("shows a confirm dialog before removing when confirmDeletion is true", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: { confirmDeletion: true } });
    const user = userEvent.setup();
    const { notify } = renderPage();
    await screen.findByRole("button", { name: "Remove" });

    await user.click(screen.getByRole("button", { name: "Remove" }));
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(api.remove).not.toHaveBeenCalled();

    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(api.remove).toHaveBeenCalledWith("1.20", true));
    expect(notify).toHaveBeenCalledWith("Game version removed");
  });

  it("shows a confirm dialog when settings are still loading", async () => {
    settingsQuery.useSettingsQuery.mockReturnValue({ data: undefined });
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("button", { name: "Remove" });

    await user.click(screen.getByRole("button", { name: "Remove" }));
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(api.remove).not.toHaveBeenCalled();
  });
});
