package mods

import "errors"

// Mod error codes. These are user-facing contract codes shared with the
// frontend and must not change.
const (
	ErrModNotFound        = "MOD_NOT_FOUND"
	ErrModVersionNotFound = "MOD_VERSION_NOT_FOUND"
	ErrModCatalog         = "MOD_CATALOG_UNAVAILABLE"
	ErrModIncompatible    = "MOD_INCOMPATIBLE"
	ErrModAlreadyActive   = "MOD_DOWNLOAD_ALREADY_ACTIVE"
	ErrInvalidModFile     = "INVALID_MOD_FILE"
)

// ErrModFileExists is returned when a mod file with the same name already
// exists in the target Mods directory.
var ErrModFileExists = errors.New("mod file already exists")

var (
	errLocalModNotMatched    = errors.New("local mod not matched to the catalog")
	errLocalModAlreadyExists = errors.New("local mod already in the library")
)

// Task progress phases reported through the mods:task-progress event.
const (
	phasePreparing    = "preparing"
	phaseDownloading  = "downloading"
	phaseResolving    = "resolving"
	phaseComplete     = "complete"
	phaseFailed       = "failed"
	messageDownload   = "Download complete"
	messageInstall    = "Installation complete"
	messageInstallErr = "Installation failed"
	messagePreparing  = "Preparing download"
)
