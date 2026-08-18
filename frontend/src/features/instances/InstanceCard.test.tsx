// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { GameVersion } from "../../entities/game-version/model";
import type { Instance } from "../../entities/instance/model";
import i18n from "../../shared/i18n";
import { InstanceCard } from "./InstanceCard";

const instance: Instance = {
  id: "instance-1",
  name: "Warm home",
  description: "A cozy base",
  gameVersionId: "1.20",
  gameClient: "vanilla",
  directory: "/instances/warm-home",
  status: "ready",
  launchArguments: [],
  environmentVariables: {},
  createdAt: "2026-01-01T00:00:00Z",
  enabledModCount: 12,
  totalModCount: 14,
  playtimeSeconds: 52320,
  coverUrl: "invalid-cover.png",
};

const version: GameVersion = {
  id: "1.20",
  name: "Vintage Story 1.20",
  channel: "stable",
  platform: "linux",
  architecture: "amd64",
  installationDir: "/game",
  executablePath: "/game/Vintagestory",
  status: "installed",
  sizeBytes: 1,
  installedAt: "2026-01-01T00:00:00Z",
};

function renderCard(overrides: Partial<Parameters<typeof InstanceCard>[0]> = {}) {
  const handlers = {
    onOpen: vi.fn(),
    onEdit: vi.fn(),
    onOpenDirectory: vi.fn(),
    onClone: vi.fn(),
    onExport: vi.fn(),
    onDelete: vi.fn(),
    onLaunch: vi.fn(),
    onStop: vi.fn().mockResolvedValue(undefined),
  };
  render(<InstanceCard instance={instance} version={version} {...handlers} {...overrides} />);
  return handlers;
}

beforeAll(() => i18n.changeLanguage("en"));
afterEach(cleanup);

describe("InstanceCard", () => {
  it("presents instance identity, metadata, cover fallback, and the primary Play action", async () => {
    const handlers = renderCard();

    expect(screen.getByRole("heading", { name: "Warm home" })).toBeTruthy();
    expect(screen.getByText("Vintage Story 1.20")).toBeTruthy();
    expect(screen.getByText("12 mods")).toBeTruthy();

    const cover = document.querySelector("img") as HTMLImageElement;
    fireEvent.error(cover);
    expect(cover.hasAttribute("hidden")).toBe(true);

    await userEvent.setup().click(screen.getByRole("button", { name: "Play" }));
    expect(handlers.onLaunch).toHaveBeenCalledWith(instance);
  });

  it("exposes secondary and destructive actions through an accessible overflow menu", async () => {
    const handlers = renderCard();
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "Actions for Warm home" }));
    expect(screen.getByRole("menu")).toBeTruthy();
    expect(screen.getByRole("menuitem", { name: "Open directory" })).toBeTruthy();
    expect(screen.getByRole("menuitem", { name: "Delete instance" })).toBeTruthy();

    await user.click(screen.getByRole("menuitem", { name: "Clone" }));
    expect(handlers.onClone).toHaveBeenCalledWith(instance);

    await user.click(screen.getByRole("button", { name: "Actions for Warm home" }));
    await user.click(screen.getByRole("menuitem", { name: "Delete instance" }));
    expect(handlers.onDelete).toHaveBeenCalledWith(instance);
  });

  it("disables the primary action and announces progress while launching", () => {
    renderCard({ busy: true });

    const button = screen.getByRole("button", { name: "Starting…" });
    expect((button as HTMLButtonElement).disabled).toBe(true);
    expect(button.getAttribute("aria-busy")).toBe("true");
  });

  it("keeps a long translated name available without changing the action hierarchy", () => {
    const name = "A very long translated instance name that should remain available to the user";
    renderCard({ instance: { ...instance, name } });

    expect(screen.getByRole("heading", { name }).getAttribute("title")).toBe(name);
    expect(screen.getByRole("button", { name: "Play" })).toBeTruthy();
  });
});
