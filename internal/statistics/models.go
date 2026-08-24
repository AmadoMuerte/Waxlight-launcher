package statistics

import "github.com/AmadoMuerte/Waxlight-launcher/internal/sessions"

// Statistics aggregates persisted play sessions for the launcher statistics
// view.
type Statistics struct {
	TotalPlaytimeSeconds  int64
	LaunchCount           int
	AverageSessionSeconds int64
	MostPlayedInstanceID  *string
	RecentSessions        []sessions.PlaySession
}

// recentSessionLimit caps how many recent play sessions the overview carries.
const recentSessionLimit = 10
