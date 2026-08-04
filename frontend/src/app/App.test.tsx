// @vitest-environment jsdom

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, it, vi } from "vitest";

import i18n, { changeAppLanguage } from "../i18n";
import { App } from "./App";

const api = vi.hoisted(() => ({
  list: vi.fn().mockResolvedValue([]),
  overview: vi.fn().mockResolvedValue({
    totalPlaytimeSeconds: 0,
    launchCount: 0,
    averageSessionSeconds: 0,
    recentSessions: [],
  }),
  get: vi.fn().mockResolvedValue({
    theme: "dark",
    language: "ru",
    downloadsParallel: 3,
    confirmDeletion: true,
    minSessionDurationSec: 10,
    globalLaunchArguments: [],
  }),
}));

vi.mock("../shared/api", () => ({
  instancesApi: { list: api.list },
  versionsApi: { list: api.list },
  accountsApi: { list: api.list },
  operationsApi: { list: api.list },
  statisticsApi: { overview: api.overview },
  settingsApi: { get: api.get },
  launcherApi: {},
  modsApi: {},
  modCatalogApi: {},
}));

afterEach(() => cleanup());

it("applies persisted language before rendering and navigation reacts to changes", async () => {
  render(
    <MemoryRouter>
      <App />
    </MemoryRouter>,
  );
  expect(await screen.findByRole("link", { name: /Библиотека/ })).toBeTruthy();
  expect(document.documentElement.lang).toBe("ru");
  await changeAppLanguage("en");
  await waitFor(() => expect(screen.getByRole("link", { name: /Library/ })).toBeTruthy());
  expect(i18n.resolvedLanguage).toBe("en");
});
