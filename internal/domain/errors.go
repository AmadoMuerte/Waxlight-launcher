package domain

import "fmt"

type AppError struct {
	Code      string
	Message   string
	Retryable bool
	Cause     error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
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
	ErrProcessStart        = "PROCESS_START_FAILED"
	ErrProcessStop         = "PROCESS_STOP_FAILED"
	ErrSessionExpired      = "SESSION_EXPIRED"
	ErrAuthNetwork         = "AUTH_NETWORK_ERROR"
	ErrAuthServer          = "AUTH_SERVER_ERROR"
	ErrAuthInvalidResponse = "AUTH_INVALID_RESPONSE"
	ErrAuthFlowExpired     = "AUTH_FLOW_EXPIRED"
	ErrSecretStorage       = "SECRET_STORAGE_ERROR"
	ErrClientSettings      = "CLIENT_SETTINGS_ERROR"
)
