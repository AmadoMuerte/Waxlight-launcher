package domain

import "time"

type ModSide string

const (
	ModSideClient  ModSide = "client"
	ModSideServer  ModSide = "server"
	ModSideBoth    ModSide = "both"
	ModSideUnknown ModSide = "unknown"
)

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
	Downloads       int64
	CreatedAt       *time.Time
	UpdatedAt       *time.Time
	Tags            []string
	IsDownloaded    bool
	IsInstalled     bool
	UpdateAvailable bool
}

type ModScreenshot struct {
	URL     string
	Caption string
}

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

type ModDependency struct {
	ModID       string
	Name        string
	Version     string
	Requirement string
}

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

type ModSearchResult struct {
	Items      []ModSummary
	Page       int
	PageSize   int
	TotalItems int
	TotalPages int
	HasNext    bool
}

type InstalledModInstance struct {
	InstanceID   string
	InstanceName string
	Version      string
	Enabled      bool
}

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
	FileName           string                 `json:"fileName"`
	FilePath           string                 `json:"filePath"`
	FileSize           int64                  `json:"fileSize"`
	Checksum           string                 `json:"checksum,omitempty"`
	DownloadURL        string                 `json:"downloadUrl"`
	DownloadedAt       time.Time              `json:"downloadedAt"`
	InstalledInstances []InstalledModInstance `json:"-"`
	LatestVersion      string                 `json:"-"`
	UpdateAvailable    bool                   `json:"-"`
}

type DownloadModRequest struct {
	ModID             string
	VersionID         string
	InstanceIDs       []string
	DownloadOnly      bool
	AllowIncompatible bool
}

type ModInstallResult struct {
	TaskID        string
	Downloaded    DownloadedMod
	Installations []ModInstallationResult
}

type ModInstallationResult struct {
	InstanceID   string
	InstanceName string
	Installed    bool
	Message      string
}
