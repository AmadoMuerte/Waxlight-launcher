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
  gameClient: "vanilla" | "optimum";
  defaultAccountId?: string;
  directory: string;
  status: string;
  launchArguments: string[];
  environmentVariables: Record<string, string>;
  isPinned: boolean;
  lastPlayedAt?: string;
  createdAt: string;
  enabledModCount: number;
  totalModCount: number;
  playtimeSeconds: number;
  coverUrl?: string;
}

export interface MigrationCandidate {
  path: string;
  worldCount: number;
  modCount: number;
  totalBytes: number;
  totalFiles: number;
  hasClientSettings: boolean;
  hasModConfig: boolean;
  detectedGameVersion: string;
  versionConfidence: string;
  warnings: string[];
}

export interface ExistingDataImportRequest {
  sourcePath: string;
  name: string;
  description: string;
  gameVersionId: string;
}

export type LibrarySort = "lastPlayed" | "name" | "playtime" | "gameVersion" | "createdAt";

export interface FavoriteServer {
  id: string;
  name: string;
  address: string;
  instanceId?: string;
}

export interface PublicServer {
  name: string;
  address: string;
  description: string;
  players: number;
  modCount: number;
  requiresWhitelist: boolean;
  accessRestricted: boolean;
  joinable: boolean;
}

export interface InstanceSnapshot {
  id: string;
  instanceId: string;
  instanceName: string;
  type: string;
  reason?: string;
  context?: Record<string, string>;
  gameVersion: string;
  createdAt: string;
  sizeBytes: number;
  modCount: number;
  worldCount: number;
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
  updatePolicy: ModUpdatePolicy;
  sizeBytes: number;
  installedAt: string;
}

export type ModUpdatePolicy = "automatic" | "compatible_only" | "pinned";

export interface ModDeletePreview {
  modId: string;
  modName: string;
  dependencies: InstalledMod[];
}

export interface ModFileFailure {
  path: string;
  error: string;
}

export interface InstallModFilesResult {
  installed: string[];
  skipped: string[];
  failed: ModFileFailure[];
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

export interface ModTag {
  name: string;
  count: number;
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
  tags?: string[];
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

export interface ModBatchInstallResult {
  modId: string;
  versionId: string;
  result: ModInstallResult;
  error?: string;
}

export interface DownloadedModCleanupResult {
  removedCount: number;
  freedBytes: number;
}

export interface ModUpdateTarget {
  modId: string;
  versionId: string;
}

export interface ModUpdateResult {
  updated: number;
  skippedByPolicy: number;
}

export interface LocalModLink {
  path?: string;
  name: string;
  version: string;
  fileName: string;
  modId?: string;
  versionId?: string;
  slug?: string;
  latestVersion?: string;
  updateAvailable: boolean;
  reason?: string;
}

export interface LinkLocalModsResult {
  linked: LocalModLink[];
  notMatched: LocalModLink[];
  failed: LocalModLink[];
}

export interface UploadModsResult {
  linked: LocalModLink[];
  notMatched: LocalModLink[];
  skipped: string[];
  failed: LocalModLink[];
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
  titleKey?: string;
  titleParams?: Record<string, string>;
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

export interface ModChange {
  name: string;
  from?: string;
  to?: string;
}

export interface ConfigurationChanges {
  gameVersionFrom?: string;
  gameVersionTo?: string;
  updated: ModChange[];
  added: ModChange[];
  removed: ModChange[];
}

export interface LastKnownGood {
  recordedAt: string;
  gameVersion: string;
  modCount: number;
  snapshotId?: string;
  snapshotExists: boolean;
  matchesCurrent: boolean;
  changeCount: number;
  changes: ConfigurationChanges;
}

// RecoverySuggestion is the backend event payload published after a failed
// startup. The frontend only renders it; it never decides whether a launch
// failed or which snapshot to restore.
export interface RecoverySuggestion {
  instanceId: string;
  recordedAt: string;
  snapshotId?: string;
  snapshotExists: boolean;
  changes: ConfigurationChanges;
  stateSignature: string;
}

export interface Statistics {
  totalPlaytimeSeconds: number;
  launchCount: number;
  averageSessionSeconds: number;
  mostPlayedInstanceId?: string;
  recentSessions: PlaySession[];
}

export interface Settings {
  language: string;
  downloadsParallel: number;
  confirmDeletion: boolean;
  globalLaunchArguments: string[];
  optimumPath: string;
  checkForUpdates: boolean;
  updateChannel: "stable" | "prerelease";
  skippedUpdateVersion: string;
  telemetryEnabled: boolean;
  automaticSafetySnapshots: boolean;
  automaticSnapshotRetention: number;
  librarySort: LibrarySort;
}

export interface DataFolder {
  currentPath: string;
  defaultPath: string;
  lastError: string;
}

export interface OptimumStatus {
  path: string;
  executable: string;
  gameVersion: string;
  ready: boolean;
  message: string;
}

export interface DataFolderProgress {
  copiedBytes: number;
  totalBytes: number;
  progress: number;
  phase: string;
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

export interface PackageAuthor {
  name?: string;
  homepage?: string;
  source?: string;
}

export interface PackageGameVersion {
  id: string;
  name: string;
}

export type PackageModSource = "moddb" | "embedded";

export interface PackageManifest {
  schemaVersion: number;
  name: string;
  description?: string;
  author?: PackageAuthor;
  gameVersion: PackageGameVersion;
  launchArguments: string[];
  mods: PackageMod[];
  configFiles: string[];
  hasIcon: boolean;
}

export interface PackageMod {
  modId?: string;
  versionId?: string;
  name: string;
  version?: string;
  fileName: string;
  source: PackageModSource;
  checksum?: string;
  downloadUrl?: string;
  enabled: boolean;
}

export type PackageVersionStatus = "installed" | "available" | "missing";
export type PackageModStatus = "available" | "incompatible" | "missing" | "embedded";

export interface PackageModCheck {
  modId?: string;
  versionId?: string;
  name: string;
  version: string;
  source: PackageModSource;
  enabled: boolean;
  status: PackageModStatus;
  message?: string;
  hasEmbedded?: boolean;
}

export interface PackageInspection {
  path: string;
  schemaVersion: number;
  name: string;
  description?: string;
  author?: PackageAuthor;
  gameVersion: PackageGameVersion;
  versionStatus: PackageVersionStatus;
  launchArguments: string[];
  mods: PackageModCheck[];
  configFiles: string[];
  hasIcon: boolean;
  totalSize: number;
  unverifiedFiles: number;
  warnings: string[];
}

export interface ImportedModResult {
  name: string;
  version: string;
  status: "installed" | "skipped" | "failed";
  message?: string;
}

export interface ImportReport {
  instanceId: string;
  instanceName: string;
  gameVersionId: string;
  mods: ImportedModResult[];
  warnings: string[];
}

export type NewsCategory = "news" | "release" | "development";

export interface NewsItem {
  id: string;
  title: string;
  url: string;
  summary: string;
  imageUrl?: string;
  publishedAt: string;
  category: NewsCategory;
}

export interface NewsFeed {
  items: NewsItem[];
  newItems: NewsItem[];
  fetchedAt: string;
  unreadCount: number;
  refreshFailed: boolean;
}
