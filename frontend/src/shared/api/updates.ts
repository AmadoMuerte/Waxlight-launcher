import { call } from "./bridge";
import type { LauncherUpdate, Settings } from "./types";

export const updatesApi = {
  currentVersion: () => call<string>("LauncherUpdateController", "CurrentVersion"),
  check: (channel: Settings["updateChannel"]) =>
    call<LauncherUpdate>("LauncherUpdateController", "CheckUpdates", channel),
  install: (channel: Settings["updateChannel"]) =>
    call<void>("LauncherUpdateController", "InstallUpdate", channel),
  openReleasePage: (channel: Settings["updateChannel"]) =>
    call<void>("LauncherUpdateController", "OpenReleasePage", channel),
};
