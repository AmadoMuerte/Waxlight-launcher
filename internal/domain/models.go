package domain

import "time"

type AccountStatus string

const (
	AccountStatusValid       AccountStatus = "valid"
	AccountStatusExpired     AccountStatus = "expired"
	AccountStatusUnknown     AccountStatus = "unknown"
	AccountStatusNeedsReauth AccountStatus = "needs_reauth"
)

type Account struct {
	ID               string        `json:"id"`
	Username         string        `json:"username"`
	DisplayName      string        `json:"displayName"`
	Email            string        `json:"email"`
	UID              string        `json:"uid"`
	SessionKey       string        `json:"-"`
	SessionSignature string        `json:"-"`
	Status           AccountStatus `json:"status"`
	IsDefault        bool          `json:"isDefault"`
	CreatedAt        time.Time     `json:"createdAt"`
	UpdatedAt        time.Time     `json:"updatedAt"`
	LastValidatedAt  *time.Time    `json:"lastValidatedAt,omitempty"`
}

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
	ID             string
	Type           string
	ResourceID     *string
	Title          string
	Status         string
	Progress       float64
	CurrentBytes   int64
	TotalBytes     int64
	BytesPerSecond int64
	ErrorCode      *string
	ErrorMessage   *string
	CreatedAt      time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
}

type Settings struct {
	Theme                 string
	Language              string
	DownloadsParallel     int
	ConfirmDeletion       bool
	MinSessionDurationSec int64
	GlobalLaunchArguments []string
}
