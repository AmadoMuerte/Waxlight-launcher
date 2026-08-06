package domain

import "fmt"

type AppError struct {
	Code      string
	Message   string
	Retryable bool
	Cause     error
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Cause }

func NewError(code, message string) error {
	return &AppError{Code: code, Message: message}
}

const (
	ErrValidation          = "VALIDATION_ERROR"
	ErrAccountNotFound     = "ACCOUNT_NOT_FOUND"
	ErrInstanceNotFound    = "INSTANCE_NOT_FOUND"
	ErrInstanceRunning     = "INSTANCE_ALREADY_RUNNING"
	ErrDirectoryConflict   = "INSTANCE_DIRECTORY_CONFLICT"
	ErrVersionNotFound     = "GAME_VERSION_NOT_FOUND"
	ErrVersionNotInstalled = "GAME_VERSION_NOT_INSTALLED"
	ErrVersionExists       = "GAME_VERSION_ALREADY_INSTALLED"
	ErrFilePermission      = "FILE_PERMISSION_DENIED"
	ErrArchiveInvalid      = "ARCHIVE_INVALID"
	ErrChecksumMismatch    = "CHECKSUM_MISMATCH"
	ErrDownloadFailed      = "DOWNLOAD_FAILED"
	ErrVersionCatalog      = "VERSION_CATALOG_UNAVAILABLE"
	ErrOperationNotFound   = "OPERATION_NOT_FOUND"
	ErrInsufficientSpace   = "INSUFFICIENT_DISK_SPACE"
	ErrModNotFound         = "MOD_NOT_FOUND"
	ErrModVersionNotFound  = "MOD_VERSION_NOT_FOUND"
	ErrModCatalog          = "MOD_CATALOG_UNAVAILABLE"
	ErrModIncompatible     = "MOD_INCOMPATIBLE"
	ErrModAlreadyActive    = "MOD_DOWNLOAD_ALREADY_ACTIVE"
	ErrInvalidModFile      = "INVALID_MOD_FILE"
	ErrProcessStart        = "PROCESS_START_FAILED"
	ErrProcessStop         = "PROCESS_STOP_FAILED"
	ErrSessionExpired      = "SESSION_EXPIRED"
	ErrAuthNetwork         = "AUTH_NETWORK_ERROR"
	ErrAuthServer          = "AUTH_SERVER_ERROR"
	ErrAuthInvalidResponse = "AUTH_INVALID_RESPONSE"
	ErrAuthFlowExpired     = "AUTH_FLOW_EXPIRED"
	ErrSecretStorage       = "SECRET_STORAGE_ERROR"
	ErrClientSettings      = "CLIENT_SETTINGS_ERROR"
	ErrUpdateUnavailable   = "UPDATE_UNAVAILABLE"
	ErrUpdateInProgress    = "UPDATE_ALREADY_IN_PROGRESS"
	ErrUpdateFailed        = "UPDATE_FAILED"
	ErrDataFolderBusy      = "DATA_FOLDER_BUSY"

	ErrUpdateDownloadFailed     = "UPDATE_DOWNLOAD_FAILED"
	ErrUpdateChecksumMismatch   = "UPDATE_CHECKSUM_MISMATCH"
	ErrUpdateSignatureMissing   = "UPDATE_SIGNATURE_MISSING"
	ErrUpdateSignatureInvalid   = "UPDATE_SIGNATURE_INVALID"
	ErrUpdatePublisherMismatch  = "UPDATE_PUBLISHER_MISMATCH"
	ErrUpdateInstallerBlocked   = "UPDATE_INSTALLER_BLOCKED"
	ErrUpdateInstallerStartFail = "UPDATE_INSTALLER_START_FAILED"
	ErrUpdateInstallerExited    = "UPDATE_INSTALLER_EXITED_EARLY"
	ErrUpdateUnsupported        = "UPDATE_UNSUPPORTED_INSTALLATION"
	ErrUpdateRestartFailed      = "UPDATE_RESTART_FAILED"
)
