package sessions

import (
	"context"
	"time"
)

type Repository interface {
	SaveSession(context.Context, PlaySession) error
	FinishSession(context.Context, string, time.Time, int, bool, int64) error
	ListSessions(context.Context, string, int) ([]PlaySession, error)
	SessionStatistics(context.Context) (StatisticsTotals, error)
	InstancePlaytime(context.Context, string) (int64, error)
	RecoverOpenSessions(context.Context, time.Time) error
}
