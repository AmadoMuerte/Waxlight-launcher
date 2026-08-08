import { call } from "./bridge";
import type {
  InstallModFilesResult,
  InstanceModUpdateReport,
  InstalledMod,
  LinkLocalModsResult,
  ModDeletePreview,
  ModUpdateResult,
  ModUpdateTarget,
  Operation,
} from "./types";

export const modsApi = {
  list: (instanceId: string) =>
    call<InstalledMod[]>("ModManagerController", "ListInstalledMods", instanceId),
  checkInstanceUpdates: (instanceId: string) =>
    call<InstanceModUpdateReport>("ModManagerController", "CheckInstanceModUpdates", instanceId),
  updateInstance: (request: {
    instanceId: string;
    mods: ModUpdateTarget[];
    allowIncompatible: boolean;
  }) => call<ModUpdateResult>("ModManagerController", "UpdateInstanceMods", request),
  install: (request: { instanceId: string; sourcePath: string; name: string; version: string }) =>
    call<Operation>("ModManagerController", "InstallModFile", request),
  installMany: (request: { instanceId: string; sourcePaths: string[] }) =>
    call<InstallModFilesResult>("ModManagerController", "InstallModFiles", request),
  linkLocal: (instanceId: string) =>
    call<LinkLocalModsResult>("ModManagerController", "LinkLocalMods", instanceId),
  toggle: (id: string, enabled: boolean) =>
    call<InstalledMod>("ModManagerController", "SetModEnabled", id, enabled),
  remove: (id: string, deleteDependencies: boolean) =>
    call<void>("ModManagerController", "RemoveMod", id, deleteDependencies),
  previewDelete: (id: string) =>
    call<ModDeletePreview>("ModManagerController", "GetModDeletePreview", id),
};
