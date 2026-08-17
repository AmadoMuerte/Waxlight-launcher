export namespace deeplink {
	
	export class Target {
	    type: string;
	    modId?: string;
	    address?: string;
	
	    static createFrom(source: any = {}) {
	        return new Target(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.modId = source["modId"];
	        this.address = source["address"];
	    }
	}

}

export namespace wails {
	
	export class AccountDTO {
	    id: string;
	    username: string;
	    displayName: string;
	    email: string;
	    status: string;
	    isDefault: boolean;
	    lastValidatedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.username = source["username"];
	        this.displayName = source["displayName"];
	        this.email = source["email"];
	        this.status = source["status"];
	        this.isDefault = source["isDefault"];
	        this.lastValidatedAt = source["lastValidatedAt"];
	    }
	}
	export class AvailableGameVersionDTO {
	    id: string;
	    name: string;
	    channel: string;
	    platform: string;
	    architecture: string;
	    downloadSize: number;
	    latest: boolean;
	    installed: boolean;
	    installStatus?: string;
	
	    static createFrom(source: any = {}) {
	        return new AvailableGameVersionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.channel = source["channel"];
	        this.platform = source["platform"];
	        this.architecture = source["architecture"];
	        this.downloadSize = source["downloadSize"];
	        this.latest = source["latest"];
	        this.installed = source["installed"];
	        this.installStatus = source["installStatus"];
	    }
	}
	export class ModInstallationResultDTO {
	    instanceId: string;
	    instanceName: string;
	    installed: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ModInstallationResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instanceId = source["instanceId"];
	        this.instanceName = source["instanceName"];
	        this.installed = source["installed"];
	        this.message = source["message"];
	    }
	}
	export class InstalledModInstanceDTO {
	    instanceId: string;
	    instanceName: string;
	    version: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InstalledModInstanceDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instanceId = source["instanceId"];
	        this.instanceName = source["instanceName"];
	        this.version = source["version"];
	        this.enabled = source["enabled"];
	    }
	}
	export class DownloadedModDTO {
	    modId: string;
	    slug?: string;
	    name: string;
	    authorName: string;
	    imageUrl?: string;
	    side: string;
	    versionId: string;
	    downloadedVersion: string;
	    gameVersions: string[];
	    tags: string[];
	    fileName: string;
	    fileSize: number;
	    downloadedAt: string;
	    installedInstances: InstalledModInstanceDTO[];
	    latestVersion?: string;
	    updateAvailable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DownloadedModDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modId = source["modId"];
	        this.slug = source["slug"];
	        this.name = source["name"];
	        this.authorName = source["authorName"];
	        this.imageUrl = source["imageUrl"];
	        this.side = source["side"];
	        this.versionId = source["versionId"];
	        this.downloadedVersion = source["downloadedVersion"];
	        this.gameVersions = source["gameVersions"];
	        this.tags = source["tags"];
	        this.fileName = source["fileName"];
	        this.fileSize = source["fileSize"];
	        this.downloadedAt = source["downloadedAt"];
	        this.installedInstances = this.convertValues(source["installedInstances"], InstalledModInstanceDTO);
	        this.latestVersion = source["latestVersion"];
	        this.updateAvailable = source["updateAvailable"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ModInstallResultDTO {
	    taskId: string;
	    downloaded: DownloadedModDTO;
	    installations: ModInstallationResultDTO[];
	
	    static createFrom(source: any = {}) {
	        return new ModInstallResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.taskId = source["taskId"];
	        this.downloaded = this.convertValues(source["downloaded"], DownloadedModDTO);
	        this.installations = this.convertValues(source["installations"], ModInstallationResultDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BatchModInstallResultDTO {
	    modId: string;
	    versionId: string;
	    result: ModInstallResultDTO;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new BatchModInstallResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modId = source["modId"];
	        this.versionId = source["versionId"];
	        this.result = this.convertValues(source["result"], ModInstallResultDTO);
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CloneInstanceRequest {
	    sourceId: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new CloneInstanceRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceId = source["sourceId"];
	        this.name = source["name"];
	    }
	}
	export class ModChangeDTO {
	    name: string;
	    from?: string;
	    to?: string;
	
	    static createFrom(source: any = {}) {
	        return new ModChangeDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.from = source["from"];
	        this.to = source["to"];
	    }
	}
	export class ConfigurationChangesDTO {
	    gameVersionFrom?: string;
	    gameVersionTo?: string;
	    updated: ModChangeDTO[];
	    added: ModChangeDTO[];
	    removed: ModChangeDTO[];
	
	    static createFrom(source: any = {}) {
	        return new ConfigurationChangesDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameVersionFrom = source["gameVersionFrom"];
	        this.gameVersionTo = source["gameVersionTo"];
	        this.updated = this.convertValues(source["updated"], ModChangeDTO);
	        this.added = this.convertValues(source["added"], ModChangeDTO);
	        this.removed = this.convertValues(source["removed"], ModChangeDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CreateInstanceRequest {
	    name: string;
	    description: string;
	    gameVersionId: string;
	    gameClient: string;
	    defaultAccountId?: string;
	    directory: string;
	    launchArguments: string[];
	
	    static createFrom(source: any = {}) {
	        return new CreateInstanceRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.gameVersionId = source["gameVersionId"];
	        this.gameClient = source["gameClient"];
	        this.defaultAccountId = source["defaultAccountId"];
	        this.directory = source["directory"];
	        this.launchArguments = source["launchArguments"];
	    }
	}
	export class DataFolderDTO {
	    currentPath: string;
	    defaultPath: string;
	    lastError: string;
	
	    static createFrom(source: any = {}) {
	        return new DataFolderDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentPath = source["currentPath"];
	        this.defaultPath = source["defaultPath"];
	        this.lastError = source["lastError"];
	    }
	}
	export class DownloadCatalogModRequest {
	    modId: string;
	    versionId: string;
	    instanceIds: string[];
	    downloadOnly: boolean;
	    allowIncompatible: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DownloadCatalogModRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modId = source["modId"];
	        this.versionId = source["versionId"];
	        this.instanceIds = source["instanceIds"];
	        this.downloadOnly = source["downloadOnly"];
	        this.allowIncompatible = source["allowIncompatible"];
	    }
	}
	export class DownloadModTargetRequest {
	    modId: string;
	    versionId: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadModTargetRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modId = source["modId"];
	        this.versionId = source["versionId"];
	    }
	}
	export class DownloadModsBatchRequest {
	    instanceId: string;
	    targets: DownloadModTargetRequest[];
	
	    static createFrom(source: any = {}) {
	        return new DownloadModsBatchRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instanceId = source["instanceId"];
	        this.targets = this.convertValues(source["targets"], DownloadModTargetRequest);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DownloadedModCleanupResultDTO {
	    removedCount: number;
	    freedBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new DownloadedModCleanupResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.removedCount = source["removedCount"];
	        this.freedBytes = source["freedBytes"];
	    }
	}
	
	export class ExportInstanceRequest {
	    instanceId: string;
	    targetPath: string;
	    name: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new ExportInstanceRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instanceId = source["instanceId"];
	        this.targetPath = source["targetPath"];
	        this.name = source["name"];
	        this.description = source["description"];
	    }
	}
	export class FavoriteServerDTO {
	    id: string;
	    name: string;
	    address: string;
	    instanceId?: string;
	
	    static createFrom(source: any = {}) {
	        return new FavoriteServerDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.address = source["address"];
	        this.instanceId = source["instanceId"];
	    }
	}
	export class GameVersionDTO {
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
	
	    static createFrom(source: any = {}) {
	        return new GameVersionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.channel = source["channel"];
	        this.platform = source["platform"];
	        this.architecture = source["architecture"];
	        this.installationDir = source["installationDir"];
	        this.executablePath = source["executablePath"];
	        this.status = source["status"];
	        this.sizeBytes = source["sizeBytes"];
	        this.installedAt = source["installedAt"];
	    }
	}
	export class ImportInstanceRequest {
	    packagePath: string;
	    name: string;
	    description: string;
	    directory: string;
	    gameVersionId: string;
	    installVersion: boolean;
	    allowIncompatible: boolean;
	    skipUnavailable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ImportInstanceRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packagePath = source["packagePath"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.directory = source["directory"];
	        this.gameVersionId = source["gameVersionId"];
	        this.installVersion = source["installVersion"];
	        this.allowIncompatible = source["allowIncompatible"];
	        this.skipUnavailable = source["skipUnavailable"];
	    }
	}
	export class InstallDownloadedModRequest {
	    modId: string;
	    versionId: string;
	    instanceIds: string[];
	    allowIncompatible: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InstallDownloadedModRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modId = source["modId"];
	        this.versionId = source["versionId"];
	        this.instanceIds = source["instanceIds"];
	        this.allowIncompatible = source["allowIncompatible"];
	    }
	}
	export class InstallModFileRequest {
	    instanceId: string;
	    sourcePath: string;
	    name: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new InstallModFileRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instanceId = source["instanceId"];
	        this.sourcePath = source["sourcePath"];
	        this.name = source["name"];
	        this.version = source["version"];
	    }
	}
	export class InstallModFilesRequest {
	    instanceId: string;
	    sourcePaths: string[];
	
	    static createFrom(source: any = {}) {
	        return new InstallModFilesRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instanceId = source["instanceId"];
	        this.sourcePaths = source["sourcePaths"];
	    }
	}
	export class ModFileFailureDTO {
	    path: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ModFileFailureDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.error = source["error"];
	    }
	}
	export class InstallModFilesResultDTO {
	    installed: string[];
	    skipped: string[];
	    failed: ModFileFailureDTO[];
	
	    static createFrom(source: any = {}) {
	        return new InstallModFilesResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installed = source["installed"];
	        this.skipped = source["skipped"];
	        this.failed = this.convertValues(source["failed"], ModFileFailureDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InstallVersionRequest {
	    id: string;
	    name: string;
	    sourcePath: string;
	    executableRelativePath: string;
	    expectedSha256: string;
	
	    static createFrom(source: any = {}) {
	        return new InstallVersionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.sourcePath = source["sourcePath"];
	        this.executableRelativePath = source["executableRelativePath"];
	        this.expectedSha256 = source["expectedSha256"];
	    }
	}
	export class InstalledModDTO {
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
	
	    static createFrom(source: any = {}) {
	        return new InstalledModDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.instanceId = source["instanceId"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.fileName = source["fileName"];
	        this.filePath = source["filePath"];
	        this.enabled = source["enabled"];
	        this.managed = source["managed"];
	        this.source = source["source"];
	        this.sizeBytes = source["sizeBytes"];
	        this.installedAt = source["installedAt"];
	    }
	}
	
	export class InstanceDTO {
	    id: string;
	    name: string;
	    description: string;
	    gameVersionId: string;
	    gameClient: string;
	    defaultAccountId?: string;
	    directory: string;
	    status: string;
	    launchArguments: string[];
	    lastPlayedAt?: string;
	    createdAt: string;
	    enabledModCount: number;
	    totalModCount: number;
	    playtimeSeconds: number;
	    coverUrl?: string;
	
	    static createFrom(source: any = {}) {
	        return new InstanceDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.gameVersionId = source["gameVersionId"];
	        this.gameClient = source["gameClient"];
	        this.defaultAccountId = source["defaultAccountId"];
	        this.directory = source["directory"];
	        this.status = source["status"];
	        this.launchArguments = source["launchArguments"];
	        this.lastPlayedAt = source["lastPlayedAt"];
	        this.createdAt = source["createdAt"];
	        this.enabledModCount = source["enabledModCount"];
	        this.totalModCount = source["totalModCount"];
	        this.playtimeSeconds = source["playtimeSeconds"];
	        this.coverUrl = source["coverUrl"];
	    }
	}
	export class ModUpdateSummaryDTO {
	    totalMods: number;
	    upToDate: number;
	    updatesAvailable: number;
	    notUpdatableLocal: number;
	    notUpdatableAbsent: number;
	    notUpdatableCatalogError: number;
	    incompatible: number;
	
	    static createFrom(source: any = {}) {
	        return new ModUpdateSummaryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalMods = source["totalMods"];
	        this.upToDate = source["upToDate"];
	        this.updatesAvailable = source["updatesAvailable"];
	        this.notUpdatableLocal = source["notUpdatableLocal"];
	        this.notUpdatableAbsent = source["notUpdatableAbsent"];
	        this.notUpdatableCatalogError = source["notUpdatableCatalogError"];
	        this.incompatible = source["incompatible"];
	    }
	}
	export class ModDependencyDTO {
	    modId: string;
	    name: string;
	    requirement: string;
	
	    static createFrom(source: any = {}) {
	        return new ModDependencyDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modId = source["modId"];
	        this.name = source["name"];
	        this.requirement = source["requirement"];
	    }
	}
	export class ModUpdateDTO {
	    modId: string;
	    name: string;
	    installedVersion: string;
	    targetVersionId: string;
	    targetVersion: string;
	    status: string;
	    reason: string;
	    changelog: string;
	    compatible: boolean;
	    prerelease: boolean;
	    addedDeps: ModDependencyDTO[];
	    removedDeps: ModDependencyDTO[];
	
	    static createFrom(source: any = {}) {
	        return new ModUpdateDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modId = source["modId"];
	        this.name = source["name"];
	        this.installedVersion = source["installedVersion"];
	        this.targetVersionId = source["targetVersionId"];
	        this.targetVersion = source["targetVersion"];
	        this.status = source["status"];
	        this.reason = source["reason"];
	        this.changelog = source["changelog"];
	        this.compatible = source["compatible"];
	        this.prerelease = source["prerelease"];
	        this.addedDeps = this.convertValues(source["addedDeps"], ModDependencyDTO);
	        this.removedDeps = this.convertValues(source["removedDeps"], ModDependencyDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InstanceModUpdateReportDTO {
	    gameVersion: string;
	    mods: ModUpdateDTO[];
	    summary: ModUpdateSummaryDTO;
	
	    static createFrom(source: any = {}) {
	        return new InstanceModUpdateReportDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameVersion = source["gameVersion"];
	        this.mods = this.convertValues(source["mods"], ModUpdateDTO);
	        this.summary = this.convertValues(source["summary"], ModUpdateSummaryDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InstanceSnapshotDTO {
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
	
	    static createFrom(source: any = {}) {
	        return new InstanceSnapshotDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.instanceId = source["instanceId"];
	        this.instanceName = source["instanceName"];
	        this.type = source["type"];
	        this.reason = source["reason"];
	        this.context = source["context"];
	        this.gameVersion = source["gameVersion"];
	        this.createdAt = source["createdAt"];
	        this.sizeBytes = source["sizeBytes"];
	        this.modCount = source["modCount"];
	        this.worldCount = source["worldCount"];
	    }
	}
	export class LastKnownGoodDTO {
	    recordedAt: string;
	    gameVersion: string;
	    modCount: number;
	    snapshotId?: string;
	    snapshotExists: boolean;
	    matchesCurrent: boolean;
	    changeCount: number;
	    changes: ConfigurationChangesDTO;
	
	    static createFrom(source: any = {}) {
	        return new LastKnownGoodDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.recordedAt = source["recordedAt"];
	        this.gameVersion = source["gameVersion"];
	        this.modCount = source["modCount"];
	        this.snapshotId = source["snapshotId"];
	        this.snapshotExists = source["snapshotExists"];
	        this.matchesCurrent = source["matchesCurrent"];
	        this.changeCount = source["changeCount"];
	        this.changes = this.convertValues(source["changes"], ConfigurationChangesDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LaunchRequest {
	    instanceId: string;
	    accountId?: string;
	
	    static createFrom(source: any = {}) {
	        return new LaunchRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instanceId = source["instanceId"];
	        this.accountId = source["accountId"];
	    }
	}
	export class LaunchValidationDTO {
	    valid: boolean;
	    issues: string[];
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new LaunchValidationDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.valid = source["valid"];
	        this.issues = source["issues"];
	        this.warnings = source["warnings"];
	    }
	}
	export class LauncherUpdateDTO {
	    installedVersion: string;
	    version: string;
	    available: boolean;
	    downgrade: boolean;
	    prerelease: boolean;
	    releaseNotes: string;
	    releasePageUrl: string;
	    assetName: string;
	    assetSize: number;
	    installationMode: string;
	
	    static createFrom(source: any = {}) {
	        return new LauncherUpdateDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installedVersion = source["installedVersion"];
	        this.version = source["version"];
	        this.available = source["available"];
	        this.downgrade = source["downgrade"];
	        this.prerelease = source["prerelease"];
	        this.releaseNotes = source["releaseNotes"];
	        this.releasePageUrl = source["releasePageUrl"];
	        this.assetName = source["assetName"];
	        this.assetSize = source["assetSize"];
	        this.installationMode = source["installationMode"];
	    }
	}
	export class LocalModLinkDTO {
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
	
	    static createFrom(source: any = {}) {
	        return new LocalModLinkDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.fileName = source["fileName"];
	        this.modId = source["modId"];
	        this.versionId = source["versionId"];
	        this.slug = source["slug"];
	        this.latestVersion = source["latestVersion"];
	        this.updateAvailable = source["updateAvailable"];
	        this.reason = source["reason"];
	    }
	}
	export class LinkLocalModsResultDTO {
	    linked: LocalModLinkDTO[];
	    notMatched: LocalModLinkDTO[];
	    failed: LocalModLinkDTO[];
	
	    static createFrom(source: any = {}) {
	        return new LinkLocalModsResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.linked = this.convertValues(source["linked"], LocalModLinkDTO);
	        this.notMatched = this.convertValues(source["notMatched"], LocalModLinkDTO);
	        this.failed = this.convertValues(source["failed"], LocalModLinkDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class LoginResultDTO {
	    status: string;
	    account?: AccountDTO;
	    flowId?: string;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new LoginResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.account = this.convertValues(source["account"], AccountDTO);
	        this.flowId = source["flowId"];
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ModDeletePreviewDTO {
	    modId: string;
	    modName: string;
	    dependencies: InstalledModDTO[];
	
	    static createFrom(source: any = {}) {
	        return new ModDeletePreviewDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modId = source["modId"];
	        this.modName = source["modName"];
	        this.dependencies = this.convertValues(source["dependencies"], InstalledModDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ModVersionDTO {
	    id: string;
	    version: string;
	    gameVersions: string[];
	    releaseType: string;
	    fileName: string;
	    fileSize: number;
	    publishedAt?: string;
	    changelog?: string;
	
	    static createFrom(source: any = {}) {
	        return new ModVersionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.version = source["version"];
	        this.gameVersions = source["gameVersions"];
	        this.releaseType = source["releaseType"];
	        this.fileName = source["fileName"];
	        this.fileSize = source["fileSize"];
	        this.publishedAt = source["publishedAt"];
	        this.changelog = source["changelog"];
	    }
	}
	export class ModScreenshotDTO {
	    url: string;
	    caption?: string;
	
	    static createFrom(source: any = {}) {
	        return new ModScreenshotDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.caption = source["caption"];
	    }
	}
	export class ModDetailsDTO {
	    id: string;
	    slug?: string;
	    name: string;
	    authorName: string;
	    summary: string;
	    imageUrl?: string;
	    side: string;
	    latestVersion?: string;
	    gameVersions: string[];
	    downloads: number;
	    createdAt?: string;
	    updatedAt?: string;
	    tags: string[];
	    isDownloaded: boolean;
	    isInstalled: boolean;
	    updateAvailable: boolean;
	    description: string;
	    screenshots: ModScreenshotDTO[];
	    versions: ModVersionDTO[];
	    websiteUrl?: string;
	    sourceUrl?: string;
	    license?: string;
	
	    static createFrom(source: any = {}) {
	        return new ModDetailsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.slug = source["slug"];
	        this.name = source["name"];
	        this.authorName = source["authorName"];
	        this.summary = source["summary"];
	        this.imageUrl = source["imageUrl"];
	        this.side = source["side"];
	        this.latestVersion = source["latestVersion"];
	        this.gameVersions = source["gameVersions"];
	        this.downloads = source["downloads"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.tags = source["tags"];
	        this.isDownloaded = source["isDownloaded"];
	        this.isInstalled = source["isInstalled"];
	        this.updateAvailable = source["updateAvailable"];
	        this.description = source["description"];
	        this.screenshots = this.convertValues(source["screenshots"], ModScreenshotDTO);
	        this.versions = this.convertValues(source["versions"], ModVersionDTO);
	        this.websiteUrl = source["websiteUrl"];
	        this.sourceUrl = source["sourceUrl"];
	        this.license = source["license"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	export class ModSearchRequest {
	    text: string;
	    gameVersion: string;
	    side: string;
	    updatedAfter?: string;
	    tags: string[];
	    compatibleOnly: boolean;
	    instanceId: string;
	    sort: string;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new ModSearchRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.gameVersion = source["gameVersion"];
	        this.side = source["side"];
	        this.updatedAfter = source["updatedAfter"];
	        this.tags = source["tags"];
	        this.compatibleOnly = source["compatibleOnly"];
	        this.instanceId = source["instanceId"];
	        this.sort = source["sort"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	    }
	}
	export class ModSummaryDTO {
	    id: string;
	    slug?: string;
	    name: string;
	    authorName: string;
	    summary: string;
	    imageUrl?: string;
	    side: string;
	    latestVersion?: string;
	    gameVersions: string[];
	    downloads: number;
	    createdAt?: string;
	    updatedAt?: string;
	    tags: string[];
	    isDownloaded: boolean;
	    isInstalled: boolean;
	    updateAvailable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ModSummaryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.slug = source["slug"];
	        this.name = source["name"];
	        this.authorName = source["authorName"];
	        this.summary = source["summary"];
	        this.imageUrl = source["imageUrl"];
	        this.side = source["side"];
	        this.latestVersion = source["latestVersion"];
	        this.gameVersions = source["gameVersions"];
	        this.downloads = source["downloads"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.tags = source["tags"];
	        this.isDownloaded = source["isDownloaded"];
	        this.isInstalled = source["isInstalled"];
	        this.updateAvailable = source["updateAvailable"];
	    }
	}
	export class ModSearchResultDTO {
	    items: ModSummaryDTO[];
	    page: number;
	    pageSize: number;
	    totalItems: number;
	    totalPages: number;
	    hasNext: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ModSearchResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], ModSummaryDTO);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalItems = source["totalItems"];
	        this.totalPages = source["totalPages"];
	        this.hasNext = source["hasNext"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ModTagDTO {
	    name: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new ModTagDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.count = source["count"];
	    }
	}
	
	export class ModUpdateResultDTO {
	    updated: number;
	
	    static createFrom(source: any = {}) {
	        return new ModUpdateResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.updated = source["updated"];
	    }
	}
	
	export class ModUpdateTargetDTO {
	    modId: string;
	    versionId: string;
	
	    static createFrom(source: any = {}) {
	        return new ModUpdateTargetDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modId = source["modId"];
	        this.versionId = source["versionId"];
	    }
	}
	
	export class OperationDTO {
	    id: string;
	    type: string;
	    resourceId?: string;
	    title: string;
	    titleKey?: string;
	    titleParams?: Record<string, string>;
	    status: string;
	    progress: number;
	    currentBytes: number;
	    totalBytes: number;
	    bytesPerSecond: number;
	    errorCode?: string;
	    errorMessage?: string;
	    createdAt: string;
	    startedAt?: string;
	    finishedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new OperationDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.resourceId = source["resourceId"];
	        this.title = source["title"];
	        this.titleKey = source["titleKey"];
	        this.titleParams = source["titleParams"];
	        this.status = source["status"];
	        this.progress = source["progress"];
	        this.currentBytes = source["currentBytes"];
	        this.totalBytes = source["totalBytes"];
	        this.bytesPerSecond = source["bytesPerSecond"];
	        this.errorCode = source["errorCode"];
	        this.errorMessage = source["errorMessage"];
	        this.createdAt = source["createdAt"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	    }
	}
	export class OptimumStatusDTO {
	    path: string;
	    executable: string;
	    gameVersion: string;
	    ready: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new OptimumStatusDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.executable = source["executable"];
	        this.gameVersion = source["gameVersion"];
	        this.ready = source["ready"];
	        this.message = source["message"];
	    }
	}
	export class PackageAuthorDTO {
	    name?: string;
	    homepage?: string;
	    source?: string;
	
	    static createFrom(source: any = {}) {
	        return new PackageAuthorDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.homepage = source["homepage"];
	        this.source = source["source"];
	    }
	}
	export class PackageGameVersionDTO {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new PackageGameVersionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class PackageModCheckDTO {
	    modId?: string;
	    versionId?: string;
	    name: string;
	    version: string;
	    source: string;
	    enabled: boolean;
	    status: string;
	    message?: string;
	    hasEmbedded?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PackageModCheckDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modId = source["modId"];
	        this.versionId = source["versionId"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.source = source["source"];
	        this.enabled = source["enabled"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.hasEmbedded = source["hasEmbedded"];
	    }
	}
	export class PackageInspectionDTO {
	    path: string;
	    schemaVersion: number;
	    name: string;
	    description?: string;
	    author?: PackageAuthorDTO;
	    gameVersion: PackageGameVersionDTO;
	    versionStatus: string;
	    launchArguments: string[];
	    mods: PackageModCheckDTO[];
	    configFiles: string[];
	    hasIcon: boolean;
	    totalSize: number;
	    unverifiedFiles: number;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new PackageInspectionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.schemaVersion = source["schemaVersion"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.author = this.convertValues(source["author"], PackageAuthorDTO);
	        this.gameVersion = this.convertValues(source["gameVersion"], PackageGameVersionDTO);
	        this.versionStatus = source["versionStatus"];
	        this.launchArguments = source["launchArguments"];
	        this.mods = this.convertValues(source["mods"], PackageModCheckDTO);
	        this.configFiles = source["configFiles"];
	        this.hasIcon = source["hasIcon"];
	        this.totalSize = source["totalSize"];
	        this.unverifiedFiles = source["unverifiedFiles"];
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PackageModDTO {
	    modId?: string;
	    versionId?: string;
	    name: string;
	    version?: string;
	    fileName: string;
	    source: string;
	    checksum?: string;
	    downloadUrl?: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PackageModDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modId = source["modId"];
	        this.versionId = source["versionId"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.fileName = source["fileName"];
	        this.source = source["source"];
	        this.checksum = source["checksum"];
	        this.downloadUrl = source["downloadUrl"];
	        this.enabled = source["enabled"];
	    }
	}
	export class PackageManifestDTO {
	    schemaVersion: number;
	    name: string;
	    description?: string;
	    author?: PackageAuthorDTO;
	    gameVersion: PackageGameVersionDTO;
	    launchArguments: string[];
	    mods: PackageModDTO[];
	    configFiles: string[];
	    hasIcon: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PackageManifestDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.author = this.convertValues(source["author"], PackageAuthorDTO);
	        this.gameVersion = this.convertValues(source["gameVersion"], PackageGameVersionDTO);
	        this.launchArguments = source["launchArguments"];
	        this.mods = this.convertValues(source["mods"], PackageModDTO);
	        this.configFiles = source["configFiles"];
	        this.hasIcon = source["hasIcon"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class PlaySessionDTO {
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
	
	    static createFrom(source: any = {}) {
	        return new PlaySessionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.instanceId = source["instanceId"];
	        this.accountId = source["accountId"];
	        this.versionId = source["versionId"];
	        this.startedAt = source["startedAt"];
	        this.endedAt = source["endedAt"];
	        this.durationSeconds = source["durationSeconds"];
	        this.exitCode = source["exitCode"];
	        this.crashed = source["crashed"];
	        this.recovered = source["recovered"];
	    }
	}
	export class PublicServerDTO {
	    name: string;
	    address: string;
	    description: string;
	    players: number;
	    modCount: number;
	    requiresWhitelist: boolean;
	    accessRestricted: boolean;
	    joinable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PublicServerDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.address = source["address"];
	        this.description = source["description"];
	        this.players = source["players"];
	        this.modCount = source["modCount"];
	        this.requiresWhitelist = source["requiresWhitelist"];
	        this.accessRestricted = source["accessRestricted"];
	        this.joinable = source["joinable"];
	    }
	}
	export class SaveFavoriteServerRequest {
	    id: string;
	    name: string;
	    address: string;
	    instanceId?: string;
	
	    static createFrom(source: any = {}) {
	        return new SaveFavoriteServerRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.address = source["address"];
	        this.instanceId = source["instanceId"];
	    }
	}
	export class ServerLaunchRequest {
	    instanceId: string;
	    accountId?: string;
	    address: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerLaunchRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instanceId = source["instanceId"];
	        this.accountId = source["accountId"];
	        this.address = source["address"];
	    }
	}
	export class SettingsDTO {
	    language: string;
	    downloadsParallel: number;
	    confirmDeletion: boolean;
	    globalLaunchArguments: string[];
	    optimumPath: string;
	    checkForUpdates: boolean;
	    updateChannel: string;
	    skippedUpdateVersion: string;
	    telemetryEnabled: boolean;
	    automaticSafetySnapshots: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SettingsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.downloadsParallel = source["downloadsParallel"];
	        this.confirmDeletion = source["confirmDeletion"];
	        this.globalLaunchArguments = source["globalLaunchArguments"];
	        this.optimumPath = source["optimumPath"];
	        this.checkForUpdates = source["checkForUpdates"];
	        this.updateChannel = source["updateChannel"];
	        this.skippedUpdateVersion = source["skippedUpdateVersion"];
	        this.telemetryEnabled = source["telemetryEnabled"];
	        this.automaticSafetySnapshots = source["automaticSafetySnapshots"];
	    }
	}
	export class StatisticsDTO {
	    totalPlaytimeSeconds: number;
	    launchCount: number;
	    averageSessionSeconds: number;
	    mostPlayedInstanceId?: string;
	    recentSessions: PlaySessionDTO[];
	
	    static createFrom(source: any = {}) {
	        return new StatisticsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalPlaytimeSeconds = source["totalPlaytimeSeconds"];
	        this.launchCount = source["launchCount"];
	        this.averageSessionSeconds = source["averageSessionSeconds"];
	        this.mostPlayedInstanceId = source["mostPlayedInstanceId"];
	        this.recentSessions = this.convertValues(source["recentSessions"], PlaySessionDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UpdateInstanceModsRequest {
	    instanceId: string;
	    mods: ModUpdateTargetDTO[];
	    allowIncompatible: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInstanceModsRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instanceId = source["instanceId"];
	        this.mods = this.convertValues(source["mods"], ModUpdateTargetDTO);
	        this.allowIncompatible = source["allowIncompatible"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UpdateInstanceRequest {
	    id: string;
	    name: string;
	    description: string;
	    gameVersionId: string;
	    gameClient?: string;
	    defaultAccountId?: string;
	    launchArguments: string[];
	    coverSourcePath?: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInstanceRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.gameVersionId = source["gameVersionId"];
	        this.gameClient = source["gameClient"];
	        this.defaultAccountId = source["defaultAccountId"];
	        this.launchArguments = source["launchArguments"];
	        this.coverSourcePath = source["coverSourcePath"];
	    }
	}
	export class UploadModsResultDTO {
	    linked: LocalModLinkDTO[];
	    notMatched: LocalModLinkDTO[];
	    skipped: string[];
	    failed: LocalModLinkDTO[];
	
	    static createFrom(source: any = {}) {
	        return new UploadModsResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.linked = this.convertValues(source["linked"], LocalModLinkDTO);
	        this.notMatched = this.convertValues(source["notMatched"], LocalModLinkDTO);
	        this.skipped = source["skipped"];
	        this.failed = this.convertValues(source["failed"], LocalModLinkDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

