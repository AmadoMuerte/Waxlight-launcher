package domain

import "time"

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
