package sessions

import (
	"context"
	"errors"
	"testing"
	"time"
)

type recordingRepository struct {
	playSessions []PlaySession
	finishID     string
	finishTime   time.Time
	finishExit   int
	finishCrash  bool
	finishSecs   int64
	recoveredAt  time.Time
	err          error
}

func (repository *recordingRepository) SaveSession(_ context.Context, session PlaySession) error {
	repository.playSessions = append(repository.playSessions, session)
	return repository.err
}

func (repository *recordingRepository) FinishSession(
	_ context.Context,
	id string,
	endedAt time.Time,
	exitCode int,
	crashed bool,
	durationSeconds int64,
) error {
	repository.finishID = id
	repository.finishTime = endedAt
	repository.finishExit = exitCode
	repository.finishCrash = crashed
	repository.finishSecs = durationSeconds
	return repository.err
}

func (repository *recordingRepository) ListSessions(
	_ context.Context,
	instanceID string,
	limit int,
) ([]PlaySession, error) {
	if repository.err != nil {
		return nil, repository.err
	}
	result := make([]PlaySession, 0, len(repository.playSessions))
	for _, playSession := range repository.playSessions {
		if instanceID == "" || playSession.InstanceID == instanceID {
			result = append(result, playSession)
		}
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

func (repository *recordingRepository) SessionStatistics(_ context.Context) (StatisticsTotals, error) {
	if repository.err != nil {
		return StatisticsTotals{}, repository.err
	}
	totals := StatisticsTotals{LaunchCount: len(repository.playSessions)}
	byInstance := make(map[string]int64)
	for _, playSession := range repository.playSessions {
		totals.TotalPlaytimeSeconds += playSession.DurationSec
		byInstance[playSession.InstanceID] += playSession.DurationSec
	}
	var mostPlayed string
	var mostPlayedSeconds int64
	for instanceID, seconds := range byInstance {
		if seconds > mostPlayedSeconds {
			mostPlayed = instanceID
			mostPlayedSeconds = seconds
		}
	}
	if mostPlayed != "" {
		totals.MostPlayedInstanceID = &mostPlayed
	}
	return totals, nil
}

func (repository *recordingRepository) InstancePlaytime(_ context.Context, instanceID string) (int64, error) {
	if repository.err != nil {
		return 0, repository.err
	}
	var total int64
	for _, playSession := range repository.playSessions {
		if playSession.InstanceID == instanceID {
			total += playSession.DurationSec
		}
	}
	return total, nil
}

func (repository *recordingRepository) RecoverOpenSessions(_ context.Context, now time.Time) error {
	repository.recoveredAt = now
	return repository.err
}

func TestServiceOwnsFinishAndRecoveryTime(t *testing.T) {
	now := time.Date(2026, time.August, 11, 12, 30, 0, 0, time.FixedZone("test", 3600))
	repository := &recordingRepository{}
	service := NewService(repository, func() time.Time { return now })

	if err := service.Finish(context.Background(), "session-id", 3, true, 120); err != nil {
		t.Fatal(err)
	}
	if repository.finishID != "session-id" || !repository.finishTime.Equal(now.UTC()) || repository.finishExit != 3 || !repository.finishCrash || repository.finishSecs != 120 {
		t.Fatalf("unexpected finish call: %+v", repository)
	}
	if err := service.RecoverOpen(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !repository.recoveredAt.Equal(now.UTC()) {
		t.Fatalf("unexpected recovery time: %s", repository.recoveredAt)
	}
}

func TestStatisticsAndInstancePlaytime(t *testing.T) {
	repository := &recordingRepository{}
	for index := 0; index < 12; index++ {
		instanceID := "secondary"
		if index < 9 {
			instanceID = "most-played"
		}
		repository.playSessions = append(repository.playSessions, PlaySession{
			ID:          time.Unix(int64(index), 0).String(),
			InstanceID:  instanceID,
			DurationSec: int64(index + 1),
		})
	}
	service := NewService(repository, time.Now)

	statistics, err := service.GetStatistics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if statistics.TotalPlaytimeSeconds != 78 || statistics.LaunchCount != 12 || statistics.AverageSessionSeconds != 6 {
		t.Fatalf("unexpected statistics: %+v", statistics)
	}
	if statistics.MostPlayedInstanceID == nil || *statistics.MostPlayedInstanceID != "most-played" {
		t.Fatalf("unexpected most-played instance: %+v", statistics.MostPlayedInstanceID)
	}
	if len(statistics.RecentSessions) != 10 {
		t.Fatalf("unexpected recent session count: %d", len(statistics.RecentSessions))
	}
	if statistics.RecentSessions[9].DurationSec != 10 {
		t.Fatalf("statistics did not preserve repository ordering: %+v", statistics.RecentSessions)
	}
	playtime, err := service.GetInstancePlaytime(context.Background(), "most-played")
	if err != nil || playtime != 45 {
		t.Fatalf("unexpected instance playtime: %d, %v", playtime, err)
	}
}

func TestQueriesPropagateRepositoryErrors(t *testing.T) {
	want := errors.New("sessions unavailable")
	service := NewService(&recordingRepository{err: want}, time.Now)

	if _, err := service.GetStatistics(context.Background()); !errors.Is(err, want) {
		t.Fatalf("GetStatistics error = %v, want %v", err, want)
	}
	if _, err := service.GetInstancePlaytime(context.Background(), "instance"); !errors.Is(err, want) {
		t.Fatalf("GetInstancePlaytime error = %v, want %v", err, want)
	}
}

func TestStatisticsAreNotLimitedToRecentSessionHistory(t *testing.T) {
	repository := &recordingRepository{playSessions: make([]PlaySession, 5001)}
	for index := range repository.playSessions {
		repository.playSessions[index] = PlaySession{InstanceID: "instance", DurationSec: 1}
	}
	service := NewService(repository, time.Now)

	statistics, err := service.GetStatistics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if statistics.LaunchCount != 5001 || statistics.TotalPlaytimeSeconds != 5001 || len(statistics.RecentSessions) != 10 {
		t.Fatalf("unexpected large-history statistics: %+v", statistics)
	}
}
