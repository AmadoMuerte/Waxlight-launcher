package sessions

import (
	"context"
	"time"
)

const recentSessionLimit = 10

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository, now func() time.Time) *Service {
	return &Service{repository: repository, now: now}
}

func (service *Service) Create(ctx context.Context, session PlaySession) error {
	return service.repository.SaveSession(ctx, session)
}

func (service *Service) Finish(
	ctx context.Context,
	id string,
	exitCode int,
	crashed bool,
	durationSeconds int64,
) error {
	return service.repository.FinishSession(
		ctx,
		id,
		service.now().UTC(),
		exitCode,
		crashed,
		durationSeconds,
	)
}

func (service *Service) RecoverOpen(ctx context.Context) error {
	return service.repository.RecoverOpenSessions(ctx, service.now().UTC())
}

func (service *Service) GetStatistics(ctx context.Context) (Statistics, error) {
	totals, err := service.repository.SessionStatistics(ctx)
	if err != nil {
		return Statistics{}, err
	}
	playSessions, err := service.repository.ListSessions(ctx, "", recentSessionLimit)
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

func (service *Service) GetInstancePlaytime(ctx context.Context, instanceID string) (int64, error) {
	return service.repository.InstancePlaytime(ctx, instanceID)
}
