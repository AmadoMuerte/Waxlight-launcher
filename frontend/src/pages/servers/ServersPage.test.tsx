// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
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
const versionsApi = vi.hoisted(() => ({ list: vi.fn() }));
const launcherApi = vi.hoisted(() => ({ launch: vi.fn(), launchServer: vi.fn() }));

vi.mock("../../shared/api/servers", () => ({ serversApi }));
vi.mock("../../shared/api/instances", () => ({ instancesApi }));
vi.mock("../../shared/api/game-versions", () => ({ versionsApi }));
vi.mock("../../shared/api/launcher", () => ({ launcherApi }));

function publicServer(name: string, requiresWhitelist: boolean) {
  return {
    name,
    address: `${name.toLocaleLowerCase().replace(/\s+/g, "-")}.example.com`,
    description: `${name} description`,
    players: 1,
    modCount: 0,
    requiresWhitelist,
    accessRestricted: false,
    joinable: true,
  };
}

function instance(id: string, name: string) {
  return {
    id,
    name,
    description: "",
    gameVersionId: "1.20.4",
    gameClient: "vanilla",
    directory: `/mock/instances/${id}`,
    status: "ready",
    launchArguments: [],
    createdAt: "2026-01-01T00:00:00Z",
    enabledModCount: 0,
    totalModCount: 0,
    playtimeSeconds: 0,
  };
}

function renderPage(deepLinkAddress?: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[{ pathname: "/servers", state: { deepLinkAddress } }]}>
        <TooltipProvider>
          <ServersPage />
        </TooltipProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

async function selectJoinInstance(user: ReturnType<typeof userEvent.setup>, name: string) {
  await user.click(screen.getByRole("combobox"));
  await user.click(await screen.findByRole("option", { name: new RegExp(name) }));
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
    versionsApi.list.mockResolvedValue([]);
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

    await user.click(screen.getByText("Show whitelist servers"));
    expect(screen.queryByText("Whitelist Server")).toBeNull();
  });

  it("prompts for a protected server password inside the game", async () => {
    const user = userEvent.setup();
    const protectedServer = publicServer("Protected Server", false);
    protectedServer.accessRestricted = true;
    serversApi.listPublic.mockResolvedValue([protectedServer]);
    instancesApi.list.mockResolvedValue([instance("instance-1", "Main Instance")]);
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
    instancesApi.list.mockResolvedValue([instance("instance-1", "Main Instance")]);
    renderPage();

    await screen.findByText("Hidden Protected Server");
    const playButton = screen.getByRole("button", { name: "Play" });
    expect(playButton.hasAttribute("disabled")).toBe(false);
    await user.click(playButton);

    const dialog = await screen.findByRole("dialog");
    await selectJoinInstance(user, "Main Instance");
    const modalPlayButton = within(dialog).getByRole("button", { name: "Play" });
    expect(modalPlayButton.hasAttribute("disabled")).toBe(false);
    await user.click(modalPlayButton);

    expect(launcherApi.launch).toHaveBeenCalledWith("instance-1");
    expect(launcherApi.launchServer).not.toHaveBeenCalled();
    expect(serversApi.saveFavorite).not.toHaveBeenCalled();
  });

  it("opens catalog server details from a server deep link", async () => {
    const user = userEvent.setup();
    const server = publicServer("Catalog Server", false);
    server.address = "catalog.example.com:42420";
    serversApi.listPublic.mockResolvedValue([server]);
    renderPage("catalog.example.com:42420");

    const dialog = await screen.findByRole("dialog", { name: "Catalog Server" });
    await user.click(within(dialog).getAllByRole("button", { name: "Close" })[0]);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("opens favorite server details from a server deep link", async () => {
    serversApi.listFavorites.mockResolvedValue([
      { id: "favorite-1", name: "Favorite Server", address: "favorite.example.com:42420" },
    ]);
    renderPage("favorite.example.com:42420");

    expect(await screen.findByRole("dialog", { name: "Favorite Server" })).toBeTruthy();
  });

  it("shows a playable fallback when a deep-linked server is not listed", async () => {
    renderPage("missing.example.com:42420");

    const dialog = await screen.findByRole("dialog", { name: "Server" });
    expect(within(dialog).getByText("missing.example.com:42420")).toBeTruthy();
    expect(
      within(dialog).getByText("This server is not currently listed in the public server catalog."),
    ).toBeTruthy();
    expect(within(dialog).getByRole("button", { name: "Play" }).hasAttribute("disabled")).toBe(
      false,
    );
  });
});

describe("server joining", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    serversApi.listPublic.mockResolvedValue([publicServer("Public Server", false)]);
    serversApi.listFavorites.mockResolvedValue([]);
    instancesApi.list.mockResolvedValue([instance("instance-1", "Main Instance")]);
    versionsApi.list.mockResolvedValue([]);
    launcherApi.launch.mockResolvedValue(undefined);
    launcherApi.launchServer.mockResolvedValue(undefined);
    serversApi.saveFavorite.mockResolvedValue({
      id: "favorite-1",
      name: "Public Server",
      address: "public-server.example.com",
      instanceId: "instance-1",
    });
  });

  it("remembers the favorite instance and joins after a single confirm", async () => {
    serversApi.listFavorites.mockResolvedValue([
      {
        id: "favorite-1",
        name: "Public Server",
        address: "public-server.example.com",
        instanceId: "instance-1",
      },
    ]);
    const user = userEvent.setup();
    renderPage();

    await screen.findByText("Using: Main Instance");
    await user.click(screen.getByRole("button", { name: "Play" }));

    const dialog = await screen.findByRole("dialog");
    const combobox = within(dialog).getByRole("combobox");
    expect(combobox.textContent).toContain("Main Instance");
    const modalPlayButton = within(dialog).getByRole("button", { name: "Play" });
    expect(modalPlayButton.hasAttribute("disabled")).toBe(false);

    await user.click(modalPlayButton);

    expect(launcherApi.launchServer).toHaveBeenCalledWith(
      "instance-1",
      "public-server.example.com",
    );
    expect(serversApi.saveFavorite).toHaveBeenCalledWith({
      id: "favorite-1",
      name: "Public Server",
      address: "public-server.example.com",
      instanceId: "instance-1",
    });
  });

  it("requires explicit instance selection before joining", async () => {
    const user = userEvent.setup();
    renderPage();

    await screen.findByText("Public Server");
    await user.click(screen.getByRole("button", { name: "Play" }));

    const dialog = await screen.findByRole("dialog");
    const modalPlayButton = within(dialog).getByRole("button", { name: "Play" });
    expect(modalPlayButton.hasAttribute("disabled")).toBe(true);

    await selectJoinInstance(user, "Main Instance");
    await user.click(within(dialog).getByRole("button", { name: "Play" }));

    expect(launcherApi.launchServer).toHaveBeenCalledWith(
      "instance-1",
      "public-server.example.com",
    );
  });

  it("opens the selector when the remembered instance is missing", async () => {
    serversApi.listFavorites.mockResolvedValue([
      {
        id: "favorite-1",
        name: "Public Server",
        address: "public-server.example.com",
        instanceId: "deleted",
      },
    ]);
    const user = userEvent.setup();
    renderPage();

    await screen.findByText("Public Server");
    await user.click(screen.getByRole("button", { name: "Play" }));

    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(launcherApi.launchServer).not.toHaveBeenCalled();
  });
});

describe("server favorites and empty states", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    serversApi.listPublic.mockResolvedValue([publicServer("Public Server", false)]);
    serversApi.listFavorites.mockResolvedValue([]);
    instancesApi.list.mockResolvedValue([]);
    versionsApi.list.mockResolvedValue([]);
    launcherApi.launch.mockResolvedValue(undefined);
    launcherApi.launchServer.mockResolvedValue(undefined);
  });

  it("saves a public server as a favorite from the heart", async () => {
    const user = userEvent.setup();
    renderPage();

    await screen.findByText("Public Server");
    await user.click(screen.getByRole("button", { name: "Add favorite" }));

    expect(serversApi.saveFavorite).toHaveBeenCalledWith({
      id: "",
      name: "Public Server",
      address: "public-server.example.com",
    });
  });

  it("does not duplicate public servers after visiting favorites", async () => {
    const anonymousServers = Array.from({ length: 3 }, () => ({
      ...publicServer("Vintage Story Server", false),
      address: "",
    }));
    const favoriteServer = publicServer("Official Server", false);
    serversApi.listPublic.mockResolvedValue([...anonymousServers, favoriteServer]);
    serversApi.listFavorites
      .mockResolvedValueOnce([])
      .mockResolvedValue([
        { id: "favorite-1", name: favoriteServer.name, address: favoriteServer.address },
      ]);
    const user = userEvent.setup();
    renderPage();

    await screen.findByText("Official Server");
    const favoriteButtons = screen.getAllByRole("button", { name: "Add favorite" });
    await user.click(favoriteButtons.at(-1)!);
    await screen.findByRole("button", { name: "Remove from favorites" });
    await user.click(screen.getByRole("tab", { name: /Favorites/ }));

    expect(screen.getAllByRole("article")).toHaveLength(1);
    await user.click(screen.getByRole("tab", { name: /Public servers/ }));

    expect(screen.getAllByRole("article")).toHaveLength(4);
  });

  it("offers an empty Favorites state that returns to the public browser", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("tab", { name: /Favorites/ }));
    expect(await screen.findByText("No favorite servers yet")).toBeTruthy();
    expect(
      screen.getByText("Save servers you play on frequently for quicker access."),
    ).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "Public servers" }));
    expect(await screen.findByText("Public Server")).toBeTruthy();
  });

  it("distinguishes empty search results with a reset action", async () => {
    const user = userEvent.setup();
    renderPage();

    await screen.findByText("Public Server");
    await user.type(screen.getByLabelText("Search servers"), "zzz-no-match");

    expect(await screen.findByText("No servers found")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Reset" }));
    expect(await screen.findByText("Public Server")).toBeTruthy();
  });

  it("starts the join flow from the Server Details primary action", async () => {
    instancesApi.list.mockResolvedValue([instance("instance-1", "Main Instance")]);
    const user = userEvent.setup();
    renderPage();

    await screen.findByText("Public Server");
    await user.click(screen.getByRole("button", { name: "Open server details" }));

    const dialog = await screen.findByRole("dialog", { name: "Public Server" });
    expect(within(dialog).getByText("1 player")).toBeTruthy();
    await user.click(within(dialog).getByRole("button", { name: "Play" }));

    const joinDialog = await screen.findByRole("dialog", { name: "Public Server" });
    expect(within(joinDialog).getByRole("combobox")).toBeTruthy();
  });
});
