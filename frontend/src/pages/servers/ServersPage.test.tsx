// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "../../shared/ui/tooltip";
import { ServersPage } from "./ServersPage";

const serversApi = vi.hoisted(() => ({
  listPublic: vi.fn(),
  listFavorites: vi.fn(),
  saveFavorite: vi.fn(),
  removeFavorite: vi.fn(),
}));
const instancesApi = vi.hoisted(() => ({ list: vi.fn() }));
const launcherApi = vi.hoisted(() => ({ launch: vi.fn(), launchServer: vi.fn() }));

vi.mock("../../shared/api/servers", () => ({ serversApi }));
vi.mock("../../shared/api/instances", () => ({ instancesApi }));
vi.mock("../../shared/api/launcher", () => ({ launcherApi }));

function publicServer(name: string, requiresWhitelist: boolean) {
  return {
    name,
    address: `${name.toLocaleLowerCase()}.example.com`,
    description: `${name} description`,
    players: 1,
    modCount: 0,
    requiresWhitelist,
    accessRestricted: false,
    joinable: true,
  };
}

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <ServersPage />
      </TooltipProvider>
    </QueryClientProvider>,
  );
}

describe("server visibility filters", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    serversApi.listPublic.mockResolvedValue([
      publicServer("Public Server", false),
      publicServer("Whitelist Server", true),
    ]);
    serversApi.listFavorites.mockResolvedValue([]);
    instancesApi.list.mockResolvedValue([]);
    launcherApi.launch.mockResolvedValue(undefined);
    launcherApi.launchServer.mockResolvedValue(undefined);
  });

  it("shows whitelist servers only when enabled", async () => {
    const user = userEvent.setup();
    renderPage();

    expect(await screen.findByText("Public Server")).toBeTruthy();
    expect(screen.queryByText("Whitelist Server")).toBeNull();

    await user.click(screen.getByText("Show whitelist servers"));
    expect(await screen.findByText("Whitelist Server")).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "Reset" }));
    expect(screen.queryByText("Whitelist Server")).toBeNull();
  });

  it("prompts for a protected server password inside the game", async () => {
    const user = userEvent.setup();
    const protectedServer = publicServer("Protected Server", false);
    protectedServer.accessRestricted = true;
    serversApi.listPublic.mockResolvedValue([protectedServer]);
    instancesApi.list.mockResolvedValue([{ id: "instance-1", name: "Main Instance" }]);
    renderPage();

    await screen.findByText("Protected Server");
    await user.click(screen.getByRole("button", { name: "Play" }));

    expect(
      await screen.findByText(
        "After the game opens, select this server in the Vintage Story multiplayer list and enter its password.",
      ),
    ).toBeTruthy();
    expect(screen.queryByLabelText("Password")).toBeNull();
  });

  it("opens the game normally when a protected server address is hidden", async () => {
    const user = userEvent.setup();
    const protectedServer = publicServer("Hidden Protected Server", false);
    protectedServer.address = "";
    protectedServer.accessRestricted = true;
    protectedServer.joinable = false;
    serversApi.listPublic.mockResolvedValue([protectedServer]);
    instancesApi.list.mockResolvedValue([{ id: "instance-1", name: "Main Instance" }]);
    renderPage();

    await screen.findByText("Hidden Protected Server");
    const playButton = screen.getByRole("button", { name: "Play" });
    expect(playButton.hasAttribute("disabled")).toBe(false);
    await user.click(playButton);

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).queryByLabelText("Server address")).toBeNull();
    const modalPlayButton = within(dialog).getByRole("button", { name: "Play" });
    expect(modalPlayButton.hasAttribute("disabled")).toBe(false);
    await user.click(modalPlayButton);

    expect(launcherApi.launch).toHaveBeenCalledWith("instance-1");
    expect(launcherApi.launchServer).not.toHaveBeenCalled();
    expect(serversApi.saveFavorite).not.toHaveBeenCalled();
  });
});
