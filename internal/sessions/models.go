// Package sessions owns persisted game play sessions and playtime queries.
package sessions

import "time"

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

type StatisticsTotals struct {
	TotalPlaytimeSeconds int64
	LaunchCount          int
	MostPlayedInstanceID *string
}
