export interface Account {
  id: string;
  username: string;
  displayName: string;
  email: string;
  status: string;
  isDefault: boolean;
  lastValidatedAt?: string;
}

export type LoginStatus =
  | "success"
  | "totp_required"
  | "invalid_credentials"
  | "ip_changed"
  | "temporarily_blocked"
  | "network_error"
  | "server_error"
  | "invalid_response"
  | "unknown_error";

export interface LoginResult {
  status: LoginStatus;
  account?: Account;
  flowId?: string;
  message?: string;
}

export interface GameVersion {
  id: string;
  name: string;
  channel: string;
  platform: string;
  architecture: string;
  installationDir: string;
  executablePath: string;
  status: string;
  sizeBytes: number;
  installedAt: string;
}

export interface AvailableGameVersion {
  id: string;
  name: string;
  channel: "stable" | "unstable" | "unknown";
  platform: string;
  architecture: string;
  downloadSize: number;
  latest: boolean;
  installed: boolean;
  installStatus?: string;
}

export interface Instance {
  id: string;
  name: string;
  description: string;
  gameVersionId: string;
  defaultAccountId?: string;
  directory: string;
  status: string;
  launchArguments: string[];
  lastPlayedAt?: string;
  createdAt: string;
  enabledModCount: number;
  totalModCount: number;
  playtimeSeconds: number;
}

export interface InstalledMod {
  id: string;
  instanceId: string;
  name: string;
  version: string;
  fileName: string;
  filePath: string;
  enabled: boolean;
  managed: boolean;
  source: string;
  sizeBytes: number;
  installedAt: string;
}

export type ModUpdateStatus = "up_to_date" | "update_available" | "not_updatable" | "unknown";
export type ModNotUpdatableReason = "local_mod" | "not_in_catalog" | "catalog_error" | "";

export interface ModDependency {
  modId: string;
  name: string;
  requirement: string;
}

export interface ModUpdate {
  modId: string;
  name: string;
  installedVersion: string;
  targetVersionId: string;
  targetVersion: string;
  status: ModUpdateStatus;
  reason: ModNotUpdatableReason;
  changelog: string;
  compatible: boolean;
  prerelease: boolean;
  addedDeps: ModDependency[];
  removedDeps: ModDependency[];
}

export interface ModUpdateSummary {
  totalMods: number;
  upToDate: number;
  updatesAvailable: number;
  notUpdatableLocal: number;
  notUpdatableAbsent: number;
  notUpdatableCatalogError: number;
  incompatible: number;
}

export interface InstanceModUpdateReport {
  gameVersion: string;
  mods: ModUpdate[];
  summary: ModUpdateSummary;
}

export type ModSide = "client" | "server" | "both" | "unknown";
export type ModSort = "relevance" | "updated" | "newest" | "downloads" | "name_asc" | "name_desc";

export interface ModSummary {
  id: string;
  slug?: string;
  name: string;
  authorName: string;
  summary: string;
  imageUrl?: string;
  side: ModSide;
  latestVersion?: string;
  gameVersions: string[];
  downloads: number;
  createdAt?: string;
  updatedAt?: string;
  tags: string[];
  isDownloaded: boolean;
  isInstalled: boolean;
  updateAvailable: boolean;
}

export interface ModScreenshot {
  url: string;
  caption?: string;
}

export interface ModVersion {
  id: string;
  version: string;
  gameVersions: string[];
  releaseType: "stable" | "beta" | "alpha" | "unknown";
  fileName: string;
  fileSize: number;
  publishedAt?: string;
  changelog?: string;
}

export interface ModDetails extends ModSummary {
  description: string;
  screenshots: ModScreenshot[];
  versions: ModVersion[];
  websiteUrl?: string;
  sourceUrl?: string;
  license?: string;
}

export interface ModSearchQuery {
  text: string;
  gameVersion: string;
  side: "" | ModSide;
  updatedAfter?: string;
  tags: string[];
  compatibleOnly: boolean;
  instanceId: string;
  sort: ModSort;
  page: number;
  pageSize: number;
}

export interface ModSearchResult {
  items: ModSummary[];
  page: number;
  pageSize: number;
  totalItems: number;
  totalPages: number;
  hasNext: boolean;
}

export interface InstalledModInstance {
  instanceId: string;
  instanceName: string;
  version: string;
  enabled: boolean;
}

export interface DownloadedMod {
  modId: string;
  slug?: string;
  name: string;
  authorName: string;
  imageUrl?: string;
  side: ModSide;
  versionId: string;
  downloadedVersion: string;
  gameVersions: string[];
  fileName: string;
  fileSize: number;
  downloadedAt: string;
  installedInstances: InstalledModInstance[];
  latestVersion?: string;
  updateAvailable: boolean;
}

export interface ModInstallationResult {
  instanceId: string;
  instanceName: string;
  installed: boolean;
  message: string;
}

export interface ModInstallResult {
  taskId: string;
  downloaded: DownloadedMod;
  installations: ModInstallationResult[];
}

export interface ModTaskProgress {
  taskId: string;
  modId: string;
  phase: "preparing" | "downloading" | "verifying" | "installing" | "complete" | "failed";
  downloadedBytes: number;
  totalBytes: number;
  progress: number;
  message: string;
}

export type OperationStatus = "queued" | "running" | "completed" | "failed" | "cancelled";

export interface Operation {
  id: string;
  type: string;
  resourceId?: string;
  title: string;
  status: OperationStatus;
  progress: number;
  currentBytes: number;
  totalBytes: number;
  bytesPerSecond: number;
  errorCode?: string;
  errorMessage?: string;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
}

export interface PlaySession {
  id: string;
  instanceId: string;
  accountId?: string;
  versionId: string;
  startedAt: string;
  endedAt?: string;
  durationSeconds: number;
  exitCode?: number;
  crashed: boolean;
  recovered: boolean;
}

export interface Statistics {
  totalPlaytimeSeconds: number;
  launchCount: number;
  averageSessionSeconds: number;
  mostPlayedInstanceId?: string;
  recentSessions: PlaySession[];
}

export interface Settings {
  theme: string;
  language: string;
  downloadsParallel: number;
  confirmDeletion: boolean;
  minSessionDurationSec: number;
  globalLaunchArguments: string[];
  checkForUpdates: boolean;
  updateChannel: "stable" | "prerelease";
  skippedUpdateVersion: string;
}

export interface LauncherUpdate {
  installedVersion: string;
  version: string;
  available: boolean;
  downgrade: boolean;
  prerelease: boolean;
  releaseNotes: string;
  releasePageUrl: string;
  assetName: string;
  assetSize: number;
  installationMode: "installed" | "portable";
}

export interface LauncherUpdateProgress {
  phase: "checking" | "downloading" | "signature" | "installing" | "restarting";
  downloadedBytes: number;
  totalBytes: number;
  progress: number;
}

export interface LaunchValidation {
  valid: boolean;
  issues: string[] | null;
  warnings: string[] | null;
}
