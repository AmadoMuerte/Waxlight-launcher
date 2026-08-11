// Package instances owns launcher instance models and persistence boundaries.
package instances

import "time"

const (
	StatusReady   = "ready"
	StatusRunning = "running"
)

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

type CreateInput struct {
	Name             string
	Description      string
	GameVersionID    string
	Directory        string
	DefaultAccountID *string
	LaunchArguments  []string
}
