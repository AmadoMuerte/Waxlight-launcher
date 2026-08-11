package updates

// Update error codes. The values are part of the frontend contract and must
// not change.
const (
	ErrUpdateUnavailable        = "UPDATE_UNAVAILABLE"
	ErrUpdateInProgress         = "UPDATE_ALREADY_IN_PROGRESS"
	ErrUpdateFailed             = "UPDATE_FAILED"
	ErrUpdateUnsupported        = "UPDATE_UNSUPPORTED_INSTALLATION"
	ErrUpdateDownloadFailed     = "UPDATE_DOWNLOAD_FAILED"
	ErrUpdateSignatureInvalid   = "UPDATE_SIGNATURE_INVALID"
	ErrUpdateInstallerStartFail = "UPDATE_INSTALLER_START_FAILED"
)
