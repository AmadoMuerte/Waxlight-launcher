import { call } from "./bridge";
import type {
  DownloadedMod,
  ModBatchInstallResult,
  ModDetails,
  ModInstallResult,
  ModSearchQuery,
  ModSearchResult,
  ModTag,
  UploadModsResult,
} from "./types";

export const modCatalogApi = {
  search: (query: ModSearchQuery) =>
    call<ModSearchResult>("ModCatalogController", "SearchMods", query),
  get: (modId: string) => call<ModDetails>("ModCatalogController", "GetMod", modId),
  downloaded: () => call<DownloadedMod[]>("ModCatalogController", "ListDownloadedMods"),
  download: (request: {
    modId: string;
    versionId: string;
    instanceIds: string[];
    downloadOnly: boolean;
    allowIncompatible: boolean;
  }) => call<ModInstallResult>("ModCatalogController", "DownloadMod", request),
  downloadBatch: (request: {
    instanceId: string;
    targets: { modId: string; versionId: string }[];
  }) => call<ModBatchInstallResult[]>("ModCatalogController", "DownloadModsBatch", request),
  installDownloaded: (request: {
    modId: string;
    versionId: string;
    instanceIds: string[];
    allowIncompatible: boolean;
  }) => call<ModInstallResult>("ModCatalogController", "InstallDownloadedMod", request),
  removeDownloaded: (modId: string, versionId: string) =>
    call<void>("ModCatalogController", "RemoveDownloadedMod", modId, versionId),
  uploadMods: (sourcePaths: string[]) =>
    call<UploadModsResult>("ModCatalogController", "UploadMods", sourcePaths),
  cancelTask: (taskId: string) => call<void>("ModCatalogController", "CancelModTask", taskId),
  checkUpdates: (modId: string) =>
    call<DownloadedMod[]>("ModCatalogController", "CheckModUpdates", modId),
  tags: () => call<ModTag[]>("ModCatalogController", "ListModTags"),
};
