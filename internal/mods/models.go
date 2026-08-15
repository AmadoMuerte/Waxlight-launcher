// Package mods owns installed-mod orchestration, the downloaded-mod cache,
// ModDB browsing, and mod dependency handling for the launcher.
package mods

import "time"

// InstalledMod is a mod installed into a launcher instance.
type InstalledMod struct {
	ID          string
	InstanceID  string
	Name        string
	Version     string
	FileName    string
	FilePath    string
	Enabled     bool
	Managed     bool
	Source      string
	SizeBytes   int64
	InstalledAt time.Time
	UpdatedAt   time.Time
}

// DiscoveredMod is a mod file found on disk by a directory scan.
type DiscoveredMod struct {
	Name       string
	Version    string
	ModID      string
	FileName   string
	FilePath   string
	Enabled    bool
	SizeBytes  int64
	ModifiedAt time.Time
}

// InstanceRef is the minimal instance view used by mod orchestration. It keeps
// the mods feature independent of the instances feature model.
type InstanceRef struct {
	ID            string
	Name          string
	Directory     string
	GameVersionID string
}

// ModSide describes where a catalog mod runs in a Vintage Story game.
type ModSide string

const (
	ModSideClient  ModSide = "client"
	ModSideServer  ModSide = "server"
	ModSideBoth    ModSide = "both"
	ModSideUnknown ModSide = "unknown"
)

// ModSummary is the catalog listing of a mod.
type ModSummary struct {
	ID              string
	Slug            string
	Name            string
	AuthorName      string
	Summary         string
	ImageURL        string
	Side            ModSide
	LatestVersion   string
	GameVersions    []string
	ModIDStrings    []string
	Downloads       int64
	CreatedAt       *time.Time
	UpdatedAt       *time.Time
	Tags            []string
	IsDownloaded    bool
	IsInstalled     bool
	UpdateAvailable bool
}

// ModScreenshot is a catalog screenshot of a mod.
type ModScreenshot struct {
	URL     string
	Caption string
}

// ModVersion is a catalog release of a mod.
type ModVersion struct {
	ID           string
	Version      string
	GameVersions []string
	ReleaseType  string
	FileName     string
	FileSize     int64
	DownloadURL  string
	Checksum     string
	PublishedAt  *time.Time
	Changelog    string
}

// ModDependency is a declared catalog dependency of a mod.
type ModDependency struct {
	ModID       string
	Name        string
	Version     string
	Requirement string
}

// ModDetails is the full catalog record of a mod.
type ModDetails struct {
	ModSummary
	Description  string
	Screenshots  []ModScreenshot
	Versions     []ModVersion
	Dependencies []ModDependency
	WebsiteURL   string
	SourceURL    string
	License      string
}

// ModSearchQuery filters the catalog search.
type ModSearchQuery struct {
	Text           string
	GameVersion    string
	Side           ModSide
	UpdatedAfter   *time.Time
	Tags           []string
	CompatibleOnly bool
	InstanceID     string
	Sort           string
	Page           int
	PageSize       int
}

// ModTag is a catalog tag with the number of mods using it.
type ModTag struct {
	Name  string
	Count int
}

// ModSearchResult is one page of catalog search results.
type ModSearchResult struct {
	Items      []ModSummary
	Page       int
	PageSize   int
	TotalItems int
	TotalPages int
	HasNext    bool
}

// InstalledModInstance describes an instance where a downloaded mod is
// installed.
type InstalledModInstance struct {
	InstanceID   string
	InstanceName string
	Version      string
	Enabled      bool
}

// DownloadedMod is a mod cached in the launcher library.
type DownloadedMod struct {
	SchemaVersion      int                    `json:"schemaVersion"`
	ModID              string                 `json:"modId"`
	Slug               string                 `json:"slug,omitempty"`
	Name               string                 `json:"name"`
	AuthorName         string                 `json:"authorName"`
	ImageURL           string                 `json:"imageUrl,omitempty"`
	Side               ModSide                `json:"side"`
	VersionID          string                 `json:"versionId"`
	DownloadedVersion  string                 `json:"downloadedVersion"`
	GameVersions       []string               `json:"gameVersions"`
	Tags               []string               `json:"tags"`
	FileName           string                 `json:"fileName"`
	FilePath           string                 `json:"filePath"`
	FileSize           int64                  `json:"fileSize"`
	Checksum           string                 `json:"checksum,omitempty"`
	DownloadURL        string                 `json:"downloadUrl"`
	DownloadedAt       time.Time              `json:"downloadedAt"`
	InstalledInstances []InstalledModInstance `json:"-"`
	LatestVersion      string                 `json:"latestVersion,omitempty"`
	UpdateAvailable    bool                   `json:"updateAvailable"`
}

// DownloadModRequest asks for a catalog mod download and optional installs.
type DownloadModRequest struct {
	ModID             string
	VersionID         string
	InstanceIDs       []string
	DownloadOnly      bool
	AllowIncompatible bool
}

// DownloadModTarget identifies one catalog mod release to download.
type DownloadModTarget struct {
	ModID     string
	VersionID string
}

// BatchDownloadModsRequest downloads several catalog mods for one instance.
type BatchDownloadModsRequest struct {
	InstanceID string
	Targets    []DownloadModTarget
}

// ModInstallResult is the outcome of a catalog mod download.
type ModInstallResult struct {
	TaskID        string
	Downloaded    DownloadedMod
	DownloadedNow []DownloadedMod
	Installations []ModInstallationResult
}

// BatchModInstallResult is the per-target outcome of a batch download.
type BatchModInstallResult struct {
	ModID     string
	VersionID string
	Result    ModInstallResult
	Error     string
}

// DownloadedModCleanupResult summarizes unused cache cleanup.
type DownloadedModCleanupResult struct {
	RemovedCount int
	FreedBytes   int64
}

// ModInstallationResult is the per-instance outcome of an install.
type ModInstallationResult struct {
	InstanceID   string
	InstanceName string
	Installed    bool
	Message      string
}

// LocalModLink describes the outcome of binding a local mod file to its
// catalog entry.
type LocalModLink struct {
	Path            string
	Name            string
	Version         string
	FileName        string
	ModID           string
	VersionID       string
	Slug            string
	LatestVersion   string
	UpdateAvailable bool
	Reason          string
}

// LinkLocalModsResult aggregates local-mod binding outcomes for an instance.
type LinkLocalModsResult struct {
	Linked     []LocalModLink
	NotMatched []LocalModLink
	Failed     []LocalModLink
}

// UploadModsResult aggregates library upload outcomes.
type UploadModsResult struct {
	Linked     []LocalModLink
	NotMatched []LocalModLink
	Skipped    []string
	Failed     []LocalModLink
}

// InstallModFilesResult aggregates a batch local-file install.
type InstallModFilesResult struct {
	Installed []string
	Skipped   []string
	Failed    []ModFileFailure
}

// ModFileFailure is one failed file of a batch install.
type ModFileFailure struct {
	Path  string
	Error string
}

// ModDeletePreview reports which dependencies a mod deletion would remove.
type ModDeletePreview struct {
	ModID        string
	ModName      string
	Dependencies []InstalledMod
}

// ModUpdateTarget identifies the exact catalog release an installed mod
// should be updated to.
type ModUpdateTarget struct {
	ModID     string
	VersionID string
}

// ModUpdateResult summarizes a bulk mod update of one instance.
type ModUpdateResult struct {
	Updated int
}
