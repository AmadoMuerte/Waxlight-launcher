package domain

import "time"

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
