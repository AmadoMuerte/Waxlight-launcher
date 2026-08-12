package mods

// Allowlisted telemetry constants owned by the mods feature. The telemetry
// package references them so the allowlist can never drift from the emitted
// events.
const (
	EventModDownloaded = "mod_downloaded"
	EventModRemoved    = "mod_removed"

	ErrorModDownloadHTTP404 = "MOD_DOWNLOAD_HTTP_404"
	ErrorModDownloadFailed  = "MOD_DOWNLOAD_FAILED"

	ComponentModDownloader = "mod_downloader"

	OperationDownloadMod = "download_mod"
)
