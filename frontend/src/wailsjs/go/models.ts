export namespace presentation {
	
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
	export class CreateInstanceRequest {
	    name: string;
	    description: string;
	    gameVersionId: string;
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
	        this.defaultAccountId = source["defaultAccountId"];
	        this.directory = source["directory"];
	        this.launchArguments = source["launchArguments"];
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
	    defaultAccountId?: string;
	    directory: string;
	    status: string;
	    launchArguments: string[];
	    lastPlayedAt?: string;
	    createdAt: string;
	    enabledModCount: number;
	    totalModCount: number;
	    playtimeSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new InstanceDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.gameVersionId = source["gameVersionId"];
	        this.defaultAccountId = source["defaultAccountId"];
	        this.directory = source["directory"];
	        this.status = source["status"];
	        this.launchArguments = source["launchArguments"];
	        this.lastPlayedAt = source["lastPlayedAt"];
	        this.createdAt = source["createdAt"];
	        this.enabledModCount = source["enabledModCount"];
	        this.totalModCount = source["totalModCount"];
	        this.playtimeSeconds = source["playtimeSeconds"];
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
	export class OperationDTO {
	    id: string;
	    type: string;
	    resourceId?: string;
	    title: string;
	    status: string;
	    progress: number;
	    currentBytes: number;
	    totalBytes: number;
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
	        this.status = source["status"];
	        this.progress = source["progress"];
	        this.currentBytes = source["currentBytes"];
	        this.totalBytes = source["totalBytes"];
	        this.errorCode = source["errorCode"];
	        this.errorMessage = source["errorMessage"];
	        this.createdAt = source["createdAt"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
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
	export class SettingsDTO {
	    theme: string;
	    language: string;
	    downloadsParallel: number;
	    confirmDeletion: boolean;
	    minSessionDurationSec: number;
	    globalLaunchArguments: string[];
	
	    static createFrom(source: any = {}) {
	        return new SettingsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.language = source["language"];
	        this.downloadsParallel = source["downloadsParallel"];
	        this.confirmDeletion = source["confirmDeletion"];
	        this.minSessionDurationSec = source["minSessionDurationSec"];
	        this.globalLaunchArguments = source["globalLaunchArguments"];
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
	export class UpdateInstanceRequest {
	    id: string;
	    name: string;
	    description: string;
	    gameVersionId: string;
	    defaultAccountId?: string;
	    launchArguments: string[];
	
	    static createFrom(source: any = {}) {
	        return new UpdateInstanceRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.gameVersionId = source["gameVersionId"];
	        this.defaultAccountId = source["defaultAccountId"];
	        this.launchArguments = source["launchArguments"];
	    }
	}

}

