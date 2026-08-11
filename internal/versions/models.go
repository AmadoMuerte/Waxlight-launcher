// Package versions owns game version discovery, installation, and removal.
package versions

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
