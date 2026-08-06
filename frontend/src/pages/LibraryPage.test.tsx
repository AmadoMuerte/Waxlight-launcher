// @vitest-environment jsdom

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { GameVersion } from "../shared/api";
import { LibraryPage } from "./LibraryPage";

const api = vi.hoisted(() => ({
  create: vi.fn(),
}));

vi.mock("../shared/api", () => ({
  instancesApi: { create: api.create },
  launcherApi: {},
  modsApi: {},
  settingsApi: {},
  instancePackageApi: {},
}));

const versions: GameVersion[] = [
  {
    id: "1.20",
    name: "1.20",
    channel: "stable",
    platform: "linux",
    architecture: "amd64",
    installationDir: "/game",
    executablePath: "/game/Vintagestory",
    status: "installed",
    sizeBytes: 1,
    installedAt: "2026-01-01T00:00:00Z",
  },
];

function renderPage() {
  return render(
    <MemoryRouter>
      <LibraryPage
        instances={[]}
        versions={versions}
        accounts={[]}
        loading={false}
        refresh={vi.fn().mockResolvedValue(undefined)}
        notify={vi.fn()}
      />
    </MemoryRouter>,
  );
}

describe("library instance creation", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    api.create.mockResolvedValue({});
  });

  it("submits an empty name so the backend generates a unique default", async () => {
    renderPage();

    await userEvent.setup().click(screen.getByRole("button", { name: "＋ New instance" }));
    await userEvent.setup().click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(api.create).toHaveBeenCalledWith(
        expect.objectContaining({ name: "", gameVersionId: "1.20" }),
      );
    });
  });

  it("submits the typed name when provided", async () => {
    renderPage();

    await userEvent.setup().click(screen.getByRole("button", { name: "＋ New instance" }));
    await userEvent.setup().type(screen.getByLabelText("Name"), "My cozy world");
    await userEvent.setup().click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(api.create).toHaveBeenCalledWith(expect.objectContaining({ name: "My cozy world" }));
    });
  });
});
