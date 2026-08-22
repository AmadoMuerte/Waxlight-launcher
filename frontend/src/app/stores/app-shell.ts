import { create } from "zustand";

import { errorMessage } from "../../shared/api/bridge";
import type { LauncherUpdate, LauncherUpdateProgress, Settings } from "../../shared/api/types";
import { updatesApi } from "../../shared/api/updates";
import { useToastStore } from "./toast";

let updateCheckSequence = 0;

interface AppShellState {
  platform: string;
  launcherVersion: string;
  launcherUpdate?: LauncherUpdate;
  updateDialogOpen: boolean;
  updateNotificationEnabled: boolean;
  updateProgress?: LauncherUpdateProgress;
  installingUpdate: boolean;
  fatalError: string;
  setPlatform: (platform: string) => void;
  setLauncherVersion: (version: string) => void;
  setUpdateProgress: (progress?: LauncherUpdateProgress) => void;
  setFatalError: (message: string) => void;
  checkForUpdate: (channel: Settings["updateChannel"], openDialog?: boolean) => Promise<void>;
  installUpdate: (channel: Settings["updateChannel"]) => Promise<void>;
  openRelease: (channel: Settings["updateChannel"]) => Promise<void>;
  showUpdate: () => void;
  dismissUpdate: () => void;
}

export const useAppShellStore = create<AppShellState>((set) => ({
  platform: "",
  launcherVersion: "",
  updateDialogOpen: false,
  updateNotificationEnabled: false,
  installingUpdate: false,
  fatalError: "",
  setPlatform: (platform) => set({ platform }),
  setLauncherVersion: (launcherVersion) => set({ launcherVersion }),
  setUpdateProgress: (updateProgress) => set({ updateProgress }),
  setFatalError: (fatalError) => set({ fatalError }),
  checkForUpdate: async (channel, openDialog = false) => {
    const sequence = ++updateCheckSequence;

    try {
      const update = await updatesApi.check(channel);

      if (sequence !== updateCheckSequence) {
        return;
      }

      set({ launcherVersion: update.installedVersion });

      if (update.available) {
        set({
          launcherUpdate: update,
          updateDialogOpen: openDialog,
          updateNotificationEnabled: !openDialog,
        });
      } else {
        set({
          launcherUpdate: undefined,
          updateDialogOpen: false,
          updateNotificationEnabled: false,
        });
      }
    } catch {
      return;
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
  openRelease: async (channel) => {
    try {
      await updatesApi.openReleasePage(channel);
    } catch (error) {
      useToastStore.getState().notify(errorMessage(error), "error");
    }
  },
  showUpdate: () => {
    if (useAppShellStore.getState().launcherUpdate) {
      set({ updateDialogOpen: true });
    }
  },
  dismissUpdate: () => set({ updateDialogOpen: false }),
}));
