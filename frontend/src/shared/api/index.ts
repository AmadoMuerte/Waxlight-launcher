import { call } from "./bridge";
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
import type {
  Account,
  AvailableGameVersion,
  GameVersion,
  InstalledMod,
  Instance,
  LaunchValidation,
  LoginResult,
  Operation,
	DownloadedMod,
	ModDetails,
	ModInstallResult,
	ModSearchQuery,
	ModSearchResult,
  Settings,
  Statistics,
} from "./types";

export const accountsApi = {
  list: async () => (await ListAccounts()) as Account[],
  login: (email: string, password: string) =>
    Login(email, password) as Promise<LoginResult>,
  completeTOTP: (flowId: string, code: string) =>
    CompleteTOTP(flowId, code) as Promise<LoginResult>,
  cancelLogin: (flowId: string) => CancelLogin(flowId),
  reauthenticate: (accountId: string, email: string, password: string) =>
    ReauthenticateAccount(accountId, email, password) as Promise<LoginResult>,
  validate: async (id: string) => (await ValidateAccount(id)) as Account,
  setDefault: (id: string) => SetDefaultAccount(id),
  remove: (id: string) => RemoveAccount(id),
};

export const versionsApi = {
  list: () =>
    call<GameVersion[]>("GameVersionController", "ListInstalledVersions"),
  install: (request: {
    id: string;
    name: string;
    sourcePath: string;
    executableRelativePath: string;
    expectedSha256: string;
  }) =>
    call<Operation>(
      "GameVersionController",
      "InstallLocalVersion",
      request,
    ),
  available: () =>
    call<AvailableGameVersion[]>(
      "GameVersionController",
      "ListAvailableVersions",
    ),
  installAvailable: (versionId: string) =>
    call<Operation>(
      "GameVersionController",
      "InstallVersion",
      versionId,
    ),
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
};

export const modsApi = {
  list: (instanceId: string) =>
    call<InstalledMod[]>(
      "ModManagerController",
      "ListInstalledMods",
      instanceId,
    ),
  install: (request: {
    instanceId: string;
    sourcePath: string;
    name: string;
    version: string;
  }) => call<Operation>("ModManagerController", "InstallModFile", request),
  toggle: (id: string, enabled: boolean) =>
    call<InstalledMod>(
      "ModManagerController",
      "SetModEnabled",
      id,
      enabled,
    ),
  remove: (id: string) =>
    call<void>("ModManagerController", "RemoveMod", id),
};

export const modCatalogApi = {
  search: (query: ModSearchQuery) =>
    call<ModSearchResult>("ModCatalogController", "SearchMods", query),
  get: (modId: string) =>
    call<ModDetails>("ModCatalogController", "GetMod", modId),
  downloaded: () =>
    call<DownloadedMod[]>("ModCatalogController", "ListDownloadedMods"),
  download: (request: {
    modId: string;
    versionId: string;
    instanceIds: string[];
    downloadOnly: boolean;
    allowIncompatible: boolean;
  }) =>
    call<ModInstallResult>("ModCatalogController", "DownloadMod", request),
  installDownloaded: (request: {
    modId: string;
    versionId: string;
    instanceIds: string[];
    allowIncompatible: boolean;
  }) =>
    call<ModInstallResult>(
      "ModCatalogController",
      "InstallDownloadedMod",
      request,
    ),
  removeDownloaded: (modId: string, versionId: string) =>
    call<void>(
      "ModCatalogController",
      "RemoveDownloadedMod",
      modId,
      versionId,
    ),
  cancelTask: (taskId: string) =>
    call<void>("ModCatalogController", "CancelModTask", taskId),
  checkUpdates: (modId: string) =>
    call<DownloadedMod[]>("ModCatalogController", "CheckModUpdates", modId),
};

export const launcherApi = {
  validate: (instanceId: string, accountId?: string) =>
    call<LaunchValidation>("LaunchController", "ValidateLaunch", {
      instanceId,
      accountId,
    }),
  launch: (instanceId: string, accountId?: string) =>
    call("LaunchController", "LaunchInstance", { instanceId, accountId }),
  stop: (instanceId: string) =>
    call<void>("LaunchController", "StopInstance", instanceId),
  running: () =>
    call<string[]>("LaunchController", "GetRunningInstances"),
};

export const operationsApi = {
  list: () => call<Operation[]>("OperationController", "ListOperations"),
  cancel: (id: string) =>
    call<void>("OperationController", "CancelOperation", id),
  remove: (id: string) =>
    call<void>("OperationController", "DeleteOperation", id),
  clearHistory: () =>
    call<number>("OperationController", "ClearOperationHistory"),
};

export const statisticsApi = {
  overview: () =>
    call<Statistics>("StatisticsController", "GetOverviewStatistics"),
};

export const settingsApi = {
  get: () => call<Settings>("SettingsController", "GetSettings"),
  update: (settings: Settings) =>
    call<Settings>("SettingsController", "UpdateSettings", settings),
  selectGameArchive: () =>
    call<string>("SettingsController", "SelectGameArchive"),
  selectGameDirectory: () =>
    call<string>("SettingsController", "SelectGameDirectory"),
  selectModFile: () =>
    call<string>("SettingsController", "SelectModFile"),
  openDirectory: (path: string) =>
    call<void>("SettingsController", "OpenDirectory", path),
};

export * from "./types";
