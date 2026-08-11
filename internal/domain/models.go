package domain

import "time"

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
