// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import i18n, { changeAppLanguage } from ".";
import { SettingsPage } from "../../pages/settings/SettingsPage";
import type { Settings } from "../api";
import { normalizeLanguage } from "./languages";

const api = vi.hoisted(() => ({
  get: vi.fn(),
  update: vi.fn(),
  getDataFolder: vi.fn().mockResolvedValue({ currentPath: "", defaultPath: "", lastError: "" }),
}));
vi.mock("../api", () => ({ settingsApi: api }));

const settings: Settings = {
  theme: "dark",
  language: "en",
  downloadsParallel: 3,
  confirmDeletion: true,
  minSessionDurationSec: 10,
  globalLaunchArguments: [],
  checkForUpdates: true,
  updateChannel: "stable",
  skippedUpdateVersion: "",
};

async function renderPage() {
  api.get.mockResolvedValue(settings);
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <SettingsPage />
    </QueryClientProvider>,
  );
  await waitFor(() => expect(screen.getAllByRole("combobox").length).toBeGreaterThan(0));
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("i18n", () => {
  it("normalizes languages and renders English, Russian, interpolation, and plurals", async () => {
    expect(normalizeLanguage(" RU-ru ")).toBe("ru");
    expect(normalizeLanguage("BE_by")).toBe("be");
    expect(normalizeLanguage("BY_by")).toBe("be");
    expect(normalizeLanguage("fr")).toBe("fr");
    expect(normalizeLanguage("it")).toBe("en");
    expect(i18n.t("library")).toBe("Library");
    await changeAppLanguage("ru");
    expect(document.documentElement.lang).toBe("ru");
    expect(i18n.t("library")).toBe("Библиотека");
    expect(i18n.t("started_instance", { name: "Дом" })).toContain("Дом");
    expect(i18n.t("instances_count", { count: 5 })).toBe("5 сборок");
  });

  it("falls back to English for a missing Russian value", async () => {
    const russian = i18n.getResource("ru", "translation", "settings");
    const bundle = i18n.getResourceBundle("ru", "translation") as Record<string, string>;
    delete bundle.settings;
    await changeAppLanguage("ru");
    expect(i18n.t("settings")).toBe("Settings");
    i18n.addResource("ru", "translation", "settings", russian);
  });

  it("offers all languages and persists successful switches", async () => {
    await changeAppLanguage("en");
    let languageWhileSaving = "";
    api.update.mockImplementation(async (value: Settings) => {
      languageWhileSaving = i18n.resolvedLanguage ?? "";
      return value;
    });
    await renderPage();
    const user = userEvent.setup();
    const language = screen.getAllByRole("combobox")[0];
    await user.click(language);
    expect(screen.getByRole("option", { name: "English" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "Русский" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "Беларускі" })).toBeTruthy();
    await user.click(screen.getByRole("option", { name: "Беларускі" }));
    await waitFor(() => expect(api.update).toHaveBeenCalledTimes(1));
    expect(api.update).toHaveBeenCalledWith(expect.objectContaining({ language: "be" }));
    expect(languageWhileSaving).toBe("be");
    await waitFor(() => expect(i18n.resolvedLanguage).toBe("be"));
  });

  it("autosaves every setting without a save button", async () => {
    await changeAppLanguage("en");
    api.update.mockImplementation(async (value: Settings) => value);
    await renderPage();
    const user = userEvent.setup();

    expect(screen.queryByRole("button", { name: "Save settings" })).toBeNull();
    await user.click(screen.getAllByRole("combobox")[1]);
    await user.click(screen.getByRole("option", { name: "System" }));

    const numberInputs = screen.getAllByRole("spinbutton");
    fireEvent.change(numberInputs[0], { target: { value: "7" } });
    fireEvent.change(numberInputs[1], { target: { value: "45" } });
    fireEvent.change(screen.getByPlaceholderText("--debug"), {
      target: { value: "--debug --safe" },
    });
    await user.click(screen.getAllByRole("checkbox")[0]);

    await waitFor(() => expect(api.update).toHaveBeenCalledTimes(1));
    expect(api.update).toHaveBeenLastCalledWith(
      expect.objectContaining({
        theme: "system",
        language: "en",
        downloadsParallel: 7,
        minSessionDurationSec: 45,
        globalLaunchArguments: ["--debug", "--safe"],
        confirmDeletion: false,
      }),
    );
  });

  it("keeps the active language after a failed save", async () => {
    await changeAppLanguage("en");
    api.update.mockRejectedValue(new Error("save failed"));
    await renderPage();
    const user = userEvent.setup();
    await user.click(screen.getAllByRole("combobox")[0]);
    await user.click(screen.getByRole("option", { name: "Русский" }));
    await waitFor(() =>
      expect(api.update).toHaveBeenCalledWith(expect.objectContaining({ language: "ru" })),
    );
    await waitFor(() => expect(i18n.resolvedLanguage).toBe("en"));
  });
});
