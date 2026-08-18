// Package instances owns launcher instance models and persistence boundaries.
package instances

import "time"

const (
	StatusReady   = "ready"
	StatusRunning = "running"
)

type GameClient string

const (
	GameClientVanilla GameClient = "vanilla"
	GameClientOptimum GameClient = "optimum"
)

func NormalizeGameClient(client GameClient) (GameClient, bool) {
	switch client {
	case "", GameClientVanilla:
		return GameClientVanilla, true
	case GameClientOptimum:
		return GameClientOptimum, true
	default:
		return "", false
	}
}

type Instance struct {
	ID                   string
	Name                 string
	Description          string
	GameVersionID        string
	GameClient           GameClient
	DefaultAccountID     *string
	Directory            string
	CoverPath            *string
	Status               string
	LaunchArguments      []string
	EnvironmentVariables map[string]string
	LastPlayedAt         *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type CreateInput struct {
	Name                 string
	Description          string
	GameVersionID        string
	GameClient           GameClient
	Directory            string
	DefaultAccountID     *string
	LaunchArguments      []string
	EnvironmentVariables map[string]string
}
