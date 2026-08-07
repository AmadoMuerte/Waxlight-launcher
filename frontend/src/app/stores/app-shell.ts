import { create } from "zustand";

import {
  settingsApi,
  updatesApi,
  type LauncherUpdate,
  type LauncherUpdateProgress,
  type Settings,
} from "../../shared/api";
import { errorMessage } from "../../shared/api/bridge";
import { useToastStore } from "./toast";

let updateCheckSequence = 0;

interface AppShellState {
  platform: string;
  launcherVersion: string;
  launcherUpdate?: LauncherUpdate;
  updateProgress?: LauncherUpdateProgress;
  installingUpdate: boolean;
  fatalError: string;
  setPlatform: (platform: string) => void;
  setLauncherVersion: (version: string) => void;
  setUpdateProgress: (progress?: LauncherUpdateProgress) => void;
  setFatalError: (message: string) => void;
  checkForUpdate: (channel: Settings["updateChannel"], skippedVersion: string) => Promise<void>;
  installUpdate: (channel: Settings["updateChannel"]) => Promise<void>;
  skipUpdate: (settings: Settings, version: string) => Promise<void>;
  openRelease: (channel: Settings["updateChannel"]) => Promise<void>;
  dismissUpdate: () => void;
}

export const useAppShellStore = create<AppShellState>((set) => ({
  platform: "",
  launcherVersion: "",
  installingUpdate: false,
  fatalError: "",
  setPlatform: (platform) => set({ platform }),
  setLauncherVersion: (launcherVersion) => set({ launcherVersion }),
  setUpdateProgress: (updateProgress) => set({ updateProgress }),
  setFatalError: (fatalError) => set({ fatalError }),
  checkForUpdate: async (channel, skippedVersion) => {
    const sequence = ++updateCheckSequence;

    try {
      const update = await updatesApi.check(channel);

      if (sequence !== updateCheckSequence) {
        return;
      }

      set({ launcherVersion: update.installedVersion });

      if (update.available && update.version !== skippedVersion) {
        set({ launcherUpdate: update });
      } else {
        set({ launcherUpdate: undefined });
      }
    } catch {
      if (sequence === updateCheckSequence) {
        set({ launcherUpdate: undefined });
      }
    }
  },
  installUpdate: async (channel) => {
    set({ installingUpdate: true });
    set({
      updateProgress: {
        phase: "downloading",
        downloadedBytes: 0,
        totalBytes: useAppShellStore.getState().launcherUpdate?.assetSize ?? 0,
        progress: 0,
      },
    });

    try {
      await updatesApi.install(channel);
    } catch (error) {
      set({ installingUpdate: false, updateProgress: undefined });
      useToastStore.getState().notify(errorMessage(error), "error");
    }
  },
  skipUpdate: async (settings, version) => {
    try {
      await settingsApi.update({ ...settings, skippedUpdateVersion: version });
      set({ launcherUpdate: undefined });
    } catch (error) {
      useToastStore.getState().notify(errorMessage(error), "error");
    }
  },
  openRelease: async (channel) => {
    try {
      await updatesApi.openReleasePage(channel);
    } catch (error) {
      useToastStore.getState().notify(errorMessage(error), "error");
    }
  },
  dismissUpdate: () => set({ launcherUpdate: undefined }),
}));
