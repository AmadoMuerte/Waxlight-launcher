// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { DownloadedMod, ModSummary } from "../../shared/api";
import i18n from "../../shared/i18n";
import { ModCard } from "./ModCard";

const mod: ModSummary = {
  id: "51",
  name: "Player Corpse",
  authorName: "Ada",
  summary: "Creates a recoverable corpse after death.",
  side: "both",
  gameVersions: ["1.20.x"],
  downloads: 42_000,
  updatedAt: "2026-08-01T10:00:00Z",
  tags: [],
  isDownloaded: false,
  isInstalled: false,
  updateAvailable: false,
};

const downloaded: DownloadedMod = {
  modId: "51",
  name: "Player Corpse",
  authorName: "Ada",
  side: "both",
  versionId: "7",
  downloadedVersion: "2.0.0",
  gameVersions: ["1.20"],
  fileName: "playercorpse.zip",
  fileSize: 100,
  downloadedAt: "2026-08-02T10:00:00Z",
  installedInstances: [],
  updateAvailable: false,
};

function renderCard(
  overrides: Partial<Parameters<typeof ModCard>[0]> = {},
  cardMod: ModSummary = mod,
) {
  const handlers = {
    onOpen: vi.fn(),
    onInstall: vi.fn(),
    onSelectedChange: vi.fn(),
    onDelete: vi.fn(),
  };
  render(<ModCard mod={cardMod} layout="grid" {...handlers} {...overrides} />);
  return handlers;
}

beforeAll(() => i18n.changeLanguage("en"));
afterEach(cleanup);

describe("ModCard", () => {
  it("presents identity, author, summary, and opens details from the card", async () => {
    const handlers = renderCard();
    const user = userEvent.setup();

    expect(screen.getByRole("heading", { name: "Player Corpse" })).toBeTruthy();
    expect(screen.getByText("by Ada")).toBeTruthy();
    expect(screen.getByText("Creates a recoverable corpse after death.")).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "Open Player Corpse" }));
    expect(handlers.onOpen).toHaveBeenCalledWith("51");
  });

  it("opens details from the keyboard", () => {
    const handlers = renderCard();
    fireEvent.keyDown(screen.getByRole("button", { name: "Open Player Corpse" }), {
      key: "Enter",
    });
    expect(handlers.onOpen).toHaveBeenCalledWith("51");
  });

  it("offers Download when the mod is not downloaded", async () => {
    const handlers = renderCard();
    await userEvent.setup().click(screen.getByRole("button", { name: "Download" }));
    expect(handlers.onInstall).toHaveBeenCalledWith("51", undefined);
  });

  it("offers Install to instance for a downloaded mod and shows its status", () => {
    renderCard({ downloaded });
    expect(screen.getByRole("button", { name: "Install to instance" })).toBeTruthy();
    expect(screen.getByText("Downloaded · Not installed")).toBeTruthy();
  });

  it("offers Install to another when already installed somewhere", () => {
    renderCard({
      downloaded: {
        ...downloaded,
        installedInstances: [
          { instanceId: "inst-1", instanceName: "Survival", version: "2.0.0", enabled: true },
        ],
      },
    });
    expect(screen.getByRole("button", { name: "Install to another" })).toBeTruthy();
    expect(screen.getByText("Installed in 1 instance")).toBeTruthy();
  });

  it("shows Update with the version range when an update is available", () => {
    renderCard({
      downloaded: { ...downloaded, updateAvailable: true, latestVersion: "2.1.0" },
      mod: { ...mod, isDownloaded: true, updateAvailable: true },
    });
    expect(screen.getByRole("button", { name: "Update" })).toBeTruthy();
    expect(screen.getByText("2.0.0 → 2.1.0")).toBeTruthy();
  });

  it("keeps the primary action disabled and busy while installing", () => {
    renderCard({ installBusy: true });
    const button = screen.getByRole("button", { name: "Download" }) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    expect(button.getAttribute("aria-busy")).toBe("true");
  });

  it("exposes destructive removal through an overflow menu", async () => {
    const handlers = renderCard({ downloaded });
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "Player Corpse mod actions" }));
    await user.click(screen.getByRole("menuitem", { name: "Delete" }));
    expect(handlers.onDelete).toHaveBeenCalledWith(downloaded);
  });

  it("uses a neutral status for catalog cards that are already downloaded", () => {
    renderCard({ mod: { ...mod, isDownloaded: true } });
    expect(screen.getByText("✓ Downloaded")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Install to instance" })).toBeTruthy();
  });
});
