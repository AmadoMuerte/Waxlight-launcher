package domain

import "time"

type GameVersion struct {
	ID              string
	Name            string
	Channel         string
	Platform        string
	Architecture    string
	InstallationDir string
	ExecutablePath  string
	Status          string
	InstalledAt     time.Time
	VerifiedAt      *time.Time
	SizeBytes       int64
}

// AvailableGameVersion describes a client distribution published by the
// upstream game version catalog. It is deliberately separate from
// GameVersion, which represents an installation owned by Waxlight.
type AvailableGameVersion struct {
	ID                string
	Name              string
	Channel           string
	Platform          string
	Architecture      string
	Filename          string
	DownloadURL       string
	DownloadSize      int64
	Checksum          string
	ChecksumAlgorithm string
	Latest            bool
	Installed         bool
	InstallStatus     *string
}

type Instance struct {
	ID               string
	Name             string
	Description      string
	GameVersionID    string
	DefaultAccountID *string
	Directory        string
	CoverPath        *string
	Status           string
	LaunchArguments  []string
	LastPlayedAt     *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// FavoriteServer is a locally saved server address. Credentials are never
// stored here; Vintage Story owns its own authenticated connection flow.
type FavoriteServer struct {
	ID         string
	Name       string
	Address    string
	InstanceID *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// PublicServer is a listing published by Vintage Story's public server catalog.
// It contains no account or connection credentials.
type PublicServer struct {
	Name              string
	Address           string
	Description       string
	Players           int
	ModCount          int
	RequiresWhitelist bool
	PasswordProtected bool
	Joinable          bool
}

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

type PlaySession struct {
	ID          string
	InstanceID  string
	AccountID   *string
	VersionID   string
	ProcessID   *int
	StartedAt   time.Time
	EndedAt     *time.Time
	DurationSec int64
	ExitCode    *int
	Crashed     bool
	Recovered   bool
}

type Operation struct {
	ID             string            `json:"id"`
	Type           string            `json:"type"`
	ResourceID     *string           `json:"resourceId,omitempty"`
	Title          string            `json:"title"`
	TitleKey       string            `json:"titleKey,omitempty"`
	TitleParams    map[string]string `json:"titleParams,omitempty"`
	Status         string            `json:"status"`
	Progress       float64           `json:"progress"`
	CurrentBytes   int64             `json:"currentBytes"`
	TotalBytes     int64             `json:"totalBytes"`
	BytesPerSecond int64             `json:"bytesPerSecond"`
	ErrorCode      *string           `json:"errorCode,omitempty"`
	ErrorMessage   *string           `json:"errorMessage,omitempty"`
	CreatedAt      time.Time         `json:"createdAt"`
	StartedAt      *time.Time        `json:"startedAt,omitempty"`
	FinishedAt     *time.Time        `json:"finishedAt,omitempty"`
}

type Settings struct {
	Language                 string
	DownloadsParallel        int
	ConfirmDeletion          bool
	GlobalLaunchArguments    []string
	CheckForUpdates          bool
	UpdateChannel            string
	SkippedUpdateVersion     string
	TelemetryEnabled         bool
	AutomaticSafetySnapshots bool
}
