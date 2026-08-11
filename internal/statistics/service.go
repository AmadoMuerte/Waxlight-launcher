package statistics

import (
	"context"
)

// Service aggregates launcher statistics and per-instance playtime over the
// narrow sessions read capability.
type Service struct {
	sessions SessionsReader
}

func NewService(sessions SessionsReader) *Service {
	return &Service{sessions: sessions}
}

// Overview returns the aggregated launcher statistics.
func (service *Service) Overview(ctx context.Context) (Statistics, error) {
	totals, err := service.sessions.SessionStatistics(ctx)
	if err != nil {
		return Statistics{}, err
	}
	playSessions, err := service.sessions.ListSessions(ctx, "", recentSessionLimit)
	if err != nil {
		return Statistics{}, err
	}
	statistics := Statistics{
		TotalPlaytimeSeconds: totals.TotalPlaytimeSeconds,
		LaunchCount:          totals.LaunchCount,
		MostPlayedInstanceID: totals.MostPlayedInstanceID,
		RecentSessions:       playSessions,
	}
	if totals.LaunchCount > 0 {
		statistics.AverageSessionSeconds = totals.TotalPlaytimeSeconds / int64(totals.LaunchCount)
	}
	return statistics, nil
}

// InstancePlaytime returns the total playtime recorded for one instance.
func (service *Service) InstancePlaytime(ctx context.Context, instanceID string) (int64, error) {
	return service.sessions.InstancePlaytime(ctx, instanceID)
}
