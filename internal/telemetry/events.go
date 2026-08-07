package telemetry

// Allowlisted lifecycle event names. Every event here maps to a completed
// authoritative operation; the backend accepts no other event names.
const (
	EventInstanceCreated     = "instance_created"
	EventInstanceDeleted     = "instance_deleted"
	EventModDownloaded       = "mod_downloaded"
	EventModRemoved          = "mod_removed"
	EventUpdateStarted       = "update_started"
	EventUpdateSucceeded     = "update_succeeded"
	EventUpdateFailed        = "update_failed"
	EventGameLaunchSucceeded = "game_launch_succeeded"
	EventGameLaunchFailed    = "game_launch_failed"
)

var allowedEvents = map[string]struct{}{
	EventInstanceCreated:     {},
	EventInstanceDeleted:     {},
	EventModDownloaded:       {},
	EventModRemoved:          {},
	EventUpdateStarted:       {},
	EventUpdateSucceeded:     {},
	EventUpdateFailed:        {},
	EventGameLaunchSucceeded: {},
	EventGameLaunchFailed:    {},
}

// Structured error codes sent to POST /v1/errors. The backend only accepts
// codes from this taxonomy; they describe failure categories, never raw
// error text.
const (
	ErrorModDownloadHTTP404     = "MOD_DOWNLOAD_HTTP_404"
	ErrorModDownloadFailed      = "MOD_DOWNLOAD_FAILED"
	ErrorUpdateDownloadFailed   = "UPDATE_DOWNLOAD_FAILED"
	ErrorUpdateInstallFailed    = "UPDATE_INSTALL_FAILED"
	ErrorUpdateSignatureInvalid = "UPDATE_SIGNATURE_INVALID"
	ErrorGameLaunchFailed       = "GAME_LAUNCH_FAILED"
	ErrorAuthServerUnavailable  = "AUTH_SERVER_UNAVAILABLE"
)

var allowedErrorCodes = map[string]struct{}{
	ErrorModDownloadHTTP404:     {},
	ErrorModDownloadFailed:      {},
	ErrorUpdateDownloadFailed:   {},
	ErrorUpdateInstallFailed:    {},
	ErrorUpdateSignatureInvalid: {},
	ErrorGameLaunchFailed:       {},
	ErrorAuthServerUnavailable:  {},
}

// Components identify the Waxlight subsystem that produced an error.
const (
	ComponentLauncher       = "launcher"
	ComponentInstances      = "instances"
	ComponentModDownloader  = "mod_downloader"
	ComponentUpdater        = "updater"
	ComponentGameLauncher   = "game_launcher"
	ComponentAuthentication = "authentication"
)

var allowedComponents = map[string]struct{}{
	ComponentLauncher:       {},
	ComponentInstances:      {},
	ComponentModDownloader:  {},
	ComponentUpdater:        {},
	ComponentGameLauncher:   {},
	ComponentAuthentication: {},
}

// Operations name the action that produced an error.
const (
	OperationCreateInstance = "create_instance"
	OperationDeleteInstance = "delete_instance"
	OperationDownloadMod    = "download_mod"
	OperationRemoveMod      = "remove_mod"
	OperationDownloadUpdate = "download_update"
	OperationInstallUpdate  = "install_update"
	OperationLaunchGame     = "launch_game"
	OperationAuthenticate   = "authenticate"
)

var allowedOperations = map[string]struct{}{
	OperationCreateInstance: {},
	OperationDeleteInstance: {},
	OperationDownloadMod:    {},
	OperationRemoveMod:      {},
	OperationDownloadUpdate: {},
	OperationInstallUpdate:  {},
	OperationLaunchGame:     {},
	OperationAuthenticate:   {},
}
