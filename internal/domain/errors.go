package domain

import (
	"errors"
	"fmt"
)

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

// ErrNotFound is returned when a stored record does not exist and its absence
// is a regular state (for example a Last Known Good marker that was never
// recorded), not an error.
var ErrNotFound = errors.New("not found")

const (
	ErrValidation           = "VALIDATION_ERROR"
	ErrAccountNotFound      = "ACCOUNT_NOT_FOUND"
	ErrVersionNotFound      = "GAME_VERSION_NOT_FOUND"
	ErrVersionNotInstalled  = "GAME_VERSION_NOT_INSTALLED"
	ErrVersionExists        = "GAME_VERSION_ALREADY_INSTALLED"
	ErrVersionInstall       = "GAME_VERSION_INSTALL_FAILED"
	ErrFilePermission       = "FILE_PERMISSION_DENIED"
	ErrArchiveInvalid       = "ARCHIVE_INVALID"
	ErrChecksumMismatch     = "CHECKSUM_MISMATCH"
	ErrDownloadFailed       = "DOWNLOAD_FAILED"
	ErrVersionCatalog       = "VERSION_CATALOG_UNAVAILABLE"
	ErrOperationNotFound    = "OPERATION_NOT_FOUND"
	ErrOperationInterrupted = "OPERATION_INTERRUPTED"
	ErrInsufficientSpace    = "INSUFFICIENT_DISK_SPACE"
	ErrModVersionNotFound   = "MOD_VERSION_NOT_FOUND"
	ErrModCatalog           = "MOD_CATALOG_UNAVAILABLE"
	ErrModIncompatible      = "MOD_INCOMPATIBLE"
	ErrModAlreadyActive     = "MOD_DOWNLOAD_ALREADY_ACTIVE"
	ErrInvalidModFile       = "INVALID_MOD_FILE"
	ErrPackageInvalid       = "PACKAGE_INVALID"
	ErrPackageUnsupported   = "PACKAGE_UNSUPPORTED_VERSION"
	ErrPackageSecurity      = "PACKAGE_SECURITY_VIOLATION"
	ErrProcessStart         = "PROCESS_START_FAILED"
	ErrProcessStop          = "PROCESS_STOP_FAILED"
	ErrSessionExpired       = "SESSION_EXPIRED"
	ErrAuthNetwork          = "AUTH_NETWORK_ERROR"
	ErrAuthServer           = "AUTH_SERVER_ERROR"
	ErrAuthInvalidResponse  = "AUTH_INVALID_RESPONSE"
	ErrAuthFlowExpired      = "AUTH_FLOW_EXPIRED"
	ErrSecretStorage        = "SECRET_STORAGE_ERROR"
	ErrClientSettings       = "CLIENT_SETTINGS_ERROR"
	ErrDataFolderBusy       = "DATA_FOLDER_BUSY"
	ErrInvalidURL           = "INVALID_URL"
	ErrServerNotFound       = "SERVER_NOT_FOUND"

	ErrUpdateChecksumMismatch  = "UPDATE_CHECKSUM_MISMATCH"
	ErrUpdateSignatureMissing  = "UPDATE_SIGNATURE_MISSING"
	ErrUpdatePublisherMismatch = "UPDATE_PUBLISHER_MISMATCH"
	ErrUpdateInstallerBlocked  = "UPDATE_INSTALLER_BLOCKED"
	ErrUpdateInstallerExited   = "UPDATE_INSTALLER_EXITED_EARLY"
	ErrUpdateRestartFailed     = "UPDATE_RESTART_FAILED"
)
