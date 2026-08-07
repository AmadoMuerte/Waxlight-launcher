import { call } from "./bridge";
import type { DataFolder, Settings } from "./types";

export const settingsApi = {
  get: () => call<Settings>("SettingsController", "GetSettings"),
  update: (settings: Settings) => call<Settings>("SettingsController", "UpdateSettings", settings),
  selectGameArchive: () => call<string>("SettingsController", "SelectGameArchive"),
  selectGameDirectory: () => call<string>("SettingsController", "SelectGameDirectory"),
  selectModFile: () => call<string>("SettingsController", "SelectModFile"),
  selectModFiles: () => call<string[]>("SettingsController", "SelectModFiles"),
  openDirectory: (path: string) => call<void>("SettingsController", "OpenDirectory", path),
  getDataFolder: () => call<DataFolder>("SettingsController", "GetDataFolder"),
  selectDataFolder: () => call<string>("SettingsController", "SelectDataFolder"),
  moveDataFolder: (target: string) => call<void>("SettingsController", "MoveDataFolder", target),
};
