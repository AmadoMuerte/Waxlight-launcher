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

export type OperationStatus =
  | "queued"
  | "running"
  | "completed"
  | "failed"
  | "cancelled";

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
}

export interface LaunchValidation {
  valid: boolean;
  issues: string[] | null;
  warnings: string[] | null;
}
