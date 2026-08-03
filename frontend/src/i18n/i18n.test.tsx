// @vitest-environment jsdom

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import i18n, { changeAppLanguage } from ".";
import { normalizeLanguage } from "./languages";
import { SettingsPage } from "../pages/SettingsPage";
import type { Settings } from "../shared/api";

const api = vi.hoisted(() => ({ update: vi.fn() }));
vi.mock("../shared/api", () => ({ settingsApi: api }));

const settings: Settings = {
  theme: "dark",
  language: "en",
  downloadsParallel: 3,
  confirmDeletion: true,
  minSessionDurationSec: 10,
  globalLaunchArguments: [],
};

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("i18n", () => {
  it("normalizes languages and renders English, Russian, interpolation, and plurals", async () => {
    expect(normalizeLanguage(" RU-ru ")).toBe("ru");
    expect(normalizeLanguage("fr")).toBe("en");
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

  it("offers both languages and persists successful switches", async () => {
    api.update.mockImplementation(async (value: Settings) => value);
    render(<SettingsPage settings={settings} notify={vi.fn()} onSaved={vi.fn()} />);
    const user = userEvent.setup();
    const language = screen.getAllByRole("combobox")[0];
    await user.click(language);
    expect(screen.getByRole("option", { name: "English" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "Русский" })).toBeTruthy();
    await user.click(screen.getByRole("option", { name: "Русский" }));
    await user.click(screen.getByRole("button", { name: "Save settings" }));
    expect(api.update).toHaveBeenCalledWith(expect.objectContaining({ language: "ru" }));
    await waitFor(() => expect(i18n.resolvedLanguage).toBe("ru"));
  });

  it("saves English", async () => {
    api.update.mockImplementation(async (value: Settings) => value);
    render(<SettingsPage settings={settings} notify={vi.fn()} onSaved={vi.fn()} />);
    await userEvent.setup().click(screen.getByRole("button", { name: "Save settings" }));
    expect(api.update).toHaveBeenCalledWith(expect.objectContaining({ language: "en" }));
  });

  it("keeps the active language after a failed save", async () => {
    await changeAppLanguage("en");
    api.update.mockRejectedValue(new Error("save failed"));
    render(<SettingsPage settings={settings} notify={vi.fn()} onSaved={vi.fn()} />);
    const user = userEvent.setup();
    await user.click(screen.getAllByRole("combobox")[0]);
    await user.click(screen.getByRole("option", { name: "Русский" }));
    await user.click(screen.getByRole("button", { name: "Save settings" }));
    expect(api.update).toHaveBeenCalledWith(expect.objectContaining({ language: "ru" }));
    expect(i18n.resolvedLanguage).toBe("en");
  });
});
