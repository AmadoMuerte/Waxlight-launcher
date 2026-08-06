import type { presentation } from "../../wailsjs/go/models";
import {
  CancelLogin,
  CompleteTOTP,
  ListAccounts,
  Login,
  ReauthenticateAccount,
  RemoveAccount,
  SetDefaultAccount,
  ValidateAccount,
} from "../../wailsjs/go/presentation/AccountController";
import { call } from "./bridge";
import type {
  Account,
  AvailableGameVersion,
  DataFolder,
  GameVersion,
  ImportReport,
  InstanceModUpdateReport,
  InstalledMod,
  Instance,
  LaunchValidation,
  LoginResult,
  LauncherUpdate,
  Operation,
  DownloadedMod,
  ModDetails,
  ModInstallResult,
  ModSearchQuery,
  ModSearchResult,
  ModTag,
  PackageAuthor,
  PackageInspection,
  PackageManifest,
  Settings,
  Statistics,
} from "./types";

export const accountsApi = {
  list: async () => (await ListAccounts()) as Account[],
  login: async (email: string, password: string) => loginResult(await Login(email, password)),
  completeTOTP: async (flowId: string, code: string) =>
    loginResult(await CompleteTOTP(flowId, code)),
  cancelLogin: (flowId: string) => CancelLogin(flowId),
  reauthenticate: async (accountId: string, email: string, password: string) =>
    loginResult(await ReauthenticateAccount(accountId, email, password)),
  validate: async (id: string) => await ValidateAccount(id),
  setDefault: (id: string) => SetDefaultAccount(id),
  remove: (id: string) => RemoveAccount(id),
};

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

export const instancesApi = {
  list: () => call<Instance[]>("InstanceController", "ListInstances"),
  create: (request: {
    name: string;
    description: string;
    gameVersionId: string;
    defaultAccountId?: string;
    directory: string;
    launchArguments: string[];
  }) => call<Instance>("InstanceController", "CreateInstance", request),
  update: (request: {
    id: string;
    name: string;
    description: string;
    gameVersionId: string;
    defaultAccountId?: string;
    launchArguments: string[];
  }) => call<Instance>("InstanceController", "UpdateInstance", request),
  remove: (id: string, deleteFiles: boolean) =>
    call<void>("InstanceController", "DeleteInstance", id, deleteFiles),
  clone: (request: { sourceId: string; name: string }) =>
    call<Instance>("InstanceController", "CloneInstance", request),
};

export const modsApi = {
  list: (instanceId: string) =>
    call<InstalledMod[]>("ModManagerController", "ListInstalledMods", instanceId),
  checkInstanceUpdates: (instanceId: string) =>
    call<InstanceModUpdateReport>("ModManagerController", "CheckInstanceModUpdates", instanceId),
  install: (request: { instanceId: string; sourcePath: string; name: string; version: string }) =>
    call<Operation>("ModManagerController", "InstallModFile", request),
  toggle: (id: string, enabled: boolean) =>
    call<InstalledMod>("ModManagerController", "SetModEnabled", id, enabled),
  remove: (id: string) => call<void>("ModManagerController", "RemoveMod", id),
};

export const instancePackageApi = {
  export: (request: {
    instanceId: string;
    targetPath: string;
    name?: string;
    description?: string;
    author?: PackageAuthor;
  }) => call<PackageManifest>("InstancePackageController", "ExportInstance", request),
  inspect: (packagePath: string) =>
    call<PackageInspection>("InstancePackageController", "InspectPackage", packagePath),
  import: (request: {
    packagePath: string;
    name: string;
    description?: string;
    directory: string;
    gameVersionId: string;
    installVersion: boolean;
    allowIncompatible: boolean;
    skipUnavailable: boolean;
  }) => call<ImportReport>("InstancePackageController", "ImportPackage", request),
  selectExportPath: (suggestedName: string) =>
    call<string>("InstancePackageController", "SelectExportPath", suggestedName),
  selectPackageFile: () => call<string>("InstancePackageController", "SelectPackageFile"),
};

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
  installDownloaded: (request: {
    modId: string;
    versionId: string;
    instanceIds: string[];
    allowIncompatible: boolean;
  }) => call<ModInstallResult>("ModCatalogController", "InstallDownloadedMod", request),
  removeDownloaded: (modId: string, versionId: string) =>
    call<void>("ModCatalogController", "RemoveDownloadedMod", modId, versionId),
  cancelTask: (taskId: string) => call<void>("ModCatalogController", "CancelModTask", taskId),
  checkUpdates: (modId: string) =>
    call<DownloadedMod[]>("ModCatalogController", "CheckModUpdates", modId),
  tags: () => call<ModTag[]>("ModCatalogController", "ListModTags"),
};

export const launcherApi = {
  validate: (instanceId: string, accountId?: string) =>
    call<LaunchValidation>("LaunchController", "ValidateLaunch", {
      instanceId,
      accountId,
    }),
  launch: (instanceId: string, accountId?: string) =>
    call("LaunchController", "LaunchInstance", { instanceId, accountId }),
  stop: (instanceId: string) => call<void>("LaunchController", "StopInstance", instanceId),
  running: () => call<string[]>("LaunchController", "GetRunningInstances"),
};

export const operationsApi = {
  list: () => call<Operation[]>("OperationController", "ListOperations"),
  cancel: (id: string) => call<void>("OperationController", "CancelOperation", id),
  remove: (id: string) => call<void>("OperationController", "DeleteOperation", id),
  clearHistory: () => call<number>("OperationController", "ClearOperationHistory"),
};

export const logsApi = {
  list: (limit?: number) => call<string[]>("LogController", "ListLogs", limit ?? 0),
  exportLogs: () => call<string>("LogController", "ExportLogs"),
  openDirectory: () => call<void>("LogController", "OpenLogsDirectory"),
};

export const statisticsApi = {
  overview: () => call<Statistics>("StatisticsController", "GetOverviewStatistics"),
};

export const settingsApi = {
  get: () => call<Settings>("SettingsController", "GetSettings"),
  update: (settings: Settings) => call<Settings>("SettingsController", "UpdateSettings", settings),
  selectGameArchive: () => call<string>("SettingsController", "SelectGameArchive"),
  selectGameDirectory: () => call<string>("SettingsController", "SelectGameDirectory"),
  selectModFile: () => call<string>("SettingsController", "SelectModFile"),
  openDirectory: (path: string) => call<void>("SettingsController", "OpenDirectory", path),
  getDataFolder: () => call<DataFolder>("SettingsController", "GetDataFolder"),
  selectDataFolder: () => call<string>("SettingsController", "SelectDataFolder"),
  moveDataFolder: (target: string) => call<void>("SettingsController", "MoveDataFolder", target),
};

export const updatesApi = {
  currentVersion: () => call<string>("LauncherUpdateController", "CurrentVersion"),
  check: (channel: Settings["updateChannel"]) =>
    call<LauncherUpdate>("LauncherUpdateController", "CheckUpdates", channel),
  install: (channel: Settings["updateChannel"]) =>
    call<void>("LauncherUpdateController", "InstallUpdate", channel),
  openReleasePage: (channel: Settings["updateChannel"]) =>
    call<void>("LauncherUpdateController", "OpenReleasePage", channel),
};

export * from "./types";

function loginResult(result: presentation.LoginResultDTO): LoginResult {
  return {
    status: loginStatus(result.status),
    account: result.account,
    flowId: result.flowId,
    message: result.message,
  };
}

function loginStatus(status: string): LoginResult["status"] {
  switch (status) {
    case "success":
    case "totp_required":
    case "invalid_credentials":
    case "ip_changed":
    case "temporarily_blocked":
    case "network_error":
    case "server_error":
    case "invalid_response":
    case "unknown_error":
      return status;
    default:
      return "unknown_error";
  }
}
