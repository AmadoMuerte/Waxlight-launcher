import { call } from "./bridge";
import type { DataFolder, OptimumStatus, Settings } from "./types";

export const settingsApi = {
  get: () => call<Settings>("SettingsController", "GetSettings"),
  update: (settings: Settings) => call<Settings>("SettingsController", "UpdateSettings", settings),
  setLibrarySort: (value: Settings["librarySort"]) =>
    call<Settings>("SettingsController", "SetLibrarySort", value),
  selectGameArchive: () => call<string>("SettingsController", "SelectGameArchive"),
  selectGameDirectory: () => call<string>("SettingsController", "SelectGameDirectory"),
  getOptimumStatus: () => call<OptimumStatus>("SettingsController", "GetOptimumStatus"),
  detectOptimum: () => call<OptimumStatus>("SettingsController", "DetectOptimum"),
  inspectOptimum: (path: string) =>
    call<OptimumStatus>("SettingsController", "InspectOptimum", path),
  selectOptimumInstallation: () => call<string>("SettingsController", "SelectOptimumInstallation"),
  openOptimumInstallationGuide: () =>
    call<void>("SettingsController", "OpenOptimumInstallationGuide"),
  selectModFile: () => call<string>("SettingsController", "SelectModFile"),
  selectModFiles: () => call<string[]>("SettingsController", "SelectModFiles"),
  openDirectory: (path: string) => call<void>("SettingsController", "OpenDirectory", path),
  getDataFolder: () => call<DataFolder>("SettingsController", "GetDataFolder"),
  selectDataFolder: () => call<string>("SettingsController", "SelectDataFolder"),
  validateDataFolderTarget: (target: string) =>
    call<void>("SettingsController", "ValidateDataFolderTarget", target),
  moveDataFolder: (target: string) => call<void>("SettingsController", "MoveDataFolder", target),
};
