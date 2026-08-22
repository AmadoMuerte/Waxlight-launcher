// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { FavoriteServer, Instance, PublicServer } from "../../shared/api";
import i18n from "../../shared/i18n";
import { TooltipProvider } from "../../shared/ui/tooltip";
import { ServerCard } from "./ServerCard";

const server: PublicServer = {
  name: "The Lighthouse Community",
  address: "lighthouse.example.com:42420",
  description: "A relaxed community server focused on building.",
  players: 18,
  modCount: 12,
  requiresWhitelist: false,
  accessRestricted: false,
  joinable: true,
};

const favorite: FavoriteServer = {
  id: "favorite-1",
  name: server.name,
  address: server.address,
  instanceId: "instance-1",
};

const instance: Instance = {
  id: "instance-1",
  name: "Survival",
  description: "",
  gameVersionId: "1.20.4",
  gameClient: "vanilla",
  directory: "/mock/instances/survival",
  status: "ready",
  launchArguments: [],
  environmentVariables: {},
  isPinned: false,
  createdAt: "2026-01-01T00:00:00Z",
  enabledModCount: 0,
  totalModCount: 0,
  playtimeSeconds: 0,
};

function renderCard(
  overrides: Partial<Parameters<typeof ServerCard>[0]> = {},
  cardServer: PublicServer = server,
) {
  const handlers = {
    onJoin: vi.fn(),
    onToggleFavorite: vi.fn(),
    onDetails: vi.fn(),
    onCopyAddress: vi.fn(),
    onCopyLink: vi.fn(),
  };
  render(
    <TooltipProvider>
      <ServerCard server={cardServer} {...handlers} {...overrides} />
    </TooltipProvider>,
  );
  return handlers;
}

beforeAll(() => i18n.changeLanguage("en"));
afterEach(cleanup);

describe("ServerCard", () => {
  it("presents identity, address, population, and opens details from the card", async () => {
    const handlers = renderCard();
    const user = userEvent.setup();

    expect(screen.getByRole("heading", { name: "The Lighthouse Community" })).toBeTruthy();
    expect(screen.getByText("lighthouse.example.com:42420")).toBeTruthy();
    expect(screen.getByText("18 players")).toBeTruthy();
    expect(screen.getByText("12 mods")).toBeTruthy();
    expect(screen.getByText("A relaxed community server focused on building.")).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "Open server details" }));
    expect(handlers.onDetails).toHaveBeenCalledWith(server);
  });

  it("joins from the primary action", async () => {
    const handlers = renderCard();
    await userEvent.setup().click(screen.getByRole("button", { name: "Play" }));
    expect(handlers.onJoin).toHaveBeenCalledWith(server, undefined);
    expect(handlers.onDetails).not.toHaveBeenCalled();
  });

  it("keeps the primary action above the full-card details overlay", () => {
    renderCard();
    const play = screen.getByRole("button", { name: "Play" });
    const overlay = screen.getByRole("button", { name: "Open server details" });
    const playLayer = (play.parentElement as HTMLElement).className;
    const overlayLayer = overlay.className;
    expect(playLayer).toMatch(/\bz-\[2\]/);
    expect(overlayLayer).toMatch(/\bz-\[1\]/);
    expect(overlayLayer).toMatch(/\babsolute\b/);
  });

  it("toggles favorite from the heart and reflects the saved state", async () => {
    const user = userEvent.setup();

    const notSaved = renderCard();
    const addButton = screen.getByRole("button", { name: "Add favorite" });
    expect(addButton.getAttribute("aria-pressed")).toBe("false");
    await user.click(addButton);
    expect(notSaved.onToggleFavorite).toHaveBeenCalledWith(server, undefined);

    const saved = renderCard({ favorite });
    const removeButton = screen.getByRole("button", { name: "Remove from favorites" });
    expect(removeButton.getAttribute("aria-pressed")).toBe("true");
    await user.click(removeButton);
    expect(saved.onToggleFavorite).toHaveBeenCalledWith(server, favorite);
  });

  it("exposes copy, details, and remove actions through the overflow menu", async () => {
    const user = userEvent.setup();
    const handlers = renderCard({ favorite });

    await user.click(
      screen.getByRole("button", { name: "The Lighthouse Community server actions" }),
    );
    await user.click(screen.getByRole("menuitem", { name: "View details" }));
    expect(handlers.onDetails).toHaveBeenCalledWith(server);

    await user.click(
      screen.getByRole("button", { name: "The Lighthouse Community server actions" }),
    );
    await user.click(screen.getByRole("menuitem", { name: "Copy server address" }));
    expect(handlers.onCopyAddress).toHaveBeenCalledWith(server.address);

    await user.click(
      screen.getByRole("button", { name: "The Lighthouse Community server actions" }),
    );
    await user.click(screen.getByRole("menuitem", { name: "Copy Waxlight link" }));
    expect(handlers.onCopyLink).toHaveBeenCalledWith(server.address);

    await user.click(
      screen.getByRole("button", { name: "The Lighthouse Community server actions" }),
    );
    await user.click(screen.getByRole("menuitem", { name: "Remove from favorites" }));
    expect(handlers.onToggleFavorite).toHaveBeenCalledWith(server, favorite);
  });

  it("shows the preferred instance for a favorite", () => {
    renderCard({ favorite, preferredInstance: instance });
    expect(screen.getByText("Using: Survival")).toBeTruthy();
  });

  it("warns when the preferred instance is missing", () => {
    renderCard({ favorite });
    expect(screen.getByText("The linked instance is no longer available.")).toBeTruthy();
  });

  it("keeps the primary action busy and disabled while connecting", () => {
    renderCard({ busy: true });
    const play = screen.getByRole("button", { name: "Connecting…" }) as HTMLButtonElement;
    expect(play.disabled).toBe(true);
    expect(play.getAttribute("aria-busy")).toBe("true");
    const detailsButton = screen.getByRole("button", {
      name: "Open server details",
    }) as HTMLButtonElement;
    expect(detailsButton.disabled).toBe(true);
  });

  it("disables joining when the server cannot be launched", () => {
    renderCard({}, { ...server, joinable: false });
    const play = screen.getByRole("button", { name: "Play" }) as HTMLButtonElement;
    expect(play.disabled).toBe(true);
  });

  it("hides the player count when the catalog provides none", () => {
    renderCard({}, { ...server, players: 0, modCount: 0 });
    expect(screen.queryByText(/players?$/)).toBeNull();
    expect(screen.queryByText(/mods?$/)).toBeNull();
  });
});
