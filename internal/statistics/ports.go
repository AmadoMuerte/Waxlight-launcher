// Package statistics owns launcher statistics and per-instance playtime
// queries over the sessions read capability.
package statistics

import (
	"context"

	"github.com/waxlight/waxlight-launcher/internal/sessions"
)

// SessionsReader exposes the narrow read capabilities of the sessions feature
// that statistics aggregation builds on. It is implemented by
// sessions.Service.
type SessionsReader interface {
	SessionStatistics(context.Context) (sessions.StatisticsTotals, error)
	ListSessions(context.Context, string, int) ([]sessions.PlaySession, error)
	InstancePlaytime(context.Context, string) (int64, error)
}
