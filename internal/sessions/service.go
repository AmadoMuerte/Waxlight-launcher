package sessions

import (
	"context"
	"time"
)

// Service owns play-session persistence and recovery. Read capabilities used
// by the statistics feature are exposed through the Reader interface; the
// service implements it directly.
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

func (service *Service) SessionStatistics(ctx context.Context) (StatisticsTotals, error) {
	return service.repository.SessionStatistics(ctx)
}

func (service *Service) ListSessions(ctx context.Context, instanceID string, limit int) ([]PlaySession, error) {
	return service.repository.ListSessions(ctx, instanceID, limit)
}

func (service *Service) InstancePlaytime(ctx context.Context, instanceID string) (int64, error) {
	return service.repository.InstancePlaytime(ctx, instanceID)
}
