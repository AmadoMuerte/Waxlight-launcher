import { call } from "./bridge";
import type { AvailableGameVersion, GameVersion, Operation } from "./types";

export const versionsApi = {
  list: () => call<GameVersion[]>("GameVersionController", "ListInstalledVersions"),
  install: (request: {
    id: string;
    name: string;
    sourcePath: string;
    executableRelativePath: string;
    expectedSha256: string;
  }) => call<Operation>("GameVersionController", "InstallLocalVersion", request),
  available: () => call<AvailableGameVersion[]>("GameVersionController", "ListAvailableVersions"),
  installAvailable: (versionId: string) =>
    call<Operation>("GameVersionController", "InstallVersion", versionId),
  remove: (id: string, deleteFiles: boolean) =>
    call<void>("GameVersionController", "RemoveVersion", id, deleteFiles),
};
