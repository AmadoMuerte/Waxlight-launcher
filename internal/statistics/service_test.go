package statistics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/sessions"
)

// recordingReader implements SessionsReader with in-memory state.
type recordingReader struct {
	playSessions []sessions.PlaySession
	err          error
}

func (reader *recordingReader) SessionStatistics(_ context.Context) (sessions.StatisticsTotals, error) {
	if reader.err != nil {
		return sessions.StatisticsTotals{}, reader.err
	}
	totals := sessions.StatisticsTotals{LaunchCount: len(reader.playSessions)}
	byInstance := make(map[string]int64)
	for _, playSession := range reader.playSessions {
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

func (reader *recordingReader) ListSessions(
	_ context.Context,
	instanceID string,
	limit int,
) ([]sessions.PlaySession, error) {
	if reader.err != nil {
		return nil, reader.err
	}
	result := make([]sessions.PlaySession, 0, len(reader.playSessions))
	for _, playSession := range reader.playSessions {
		if instanceID == "" || playSession.InstanceID == instanceID {
			result = append(result, playSession)
		}
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

func (reader *recordingReader) InstancePlaytime(_ context.Context, instanceID string) (int64, error) {
	if reader.err != nil {
		return 0, reader.err
	}
	var total int64
	for _, playSession := range reader.playSessions {
		if playSession.InstanceID == instanceID {
			total += playSession.DurationSec
		}
	}
	return total, nil
}

func TestOverviewAggregatesSessions(t *testing.T) {
	reader := &recordingReader{}
	for index := 0; index < 12; index++ {
		instanceID := "secondary"
		if index < 9 {
			instanceID = "most-played"
		}
		reader.playSessions = append(reader.playSessions, sessions.PlaySession{
			ID:          time.Unix(int64(index), 0).String(),
			InstanceID:  instanceID,
			DurationSec: int64(index + 1),
		})
	}
	service := NewService(reader)

	overview, err := service.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if overview.TotalPlaytimeSeconds != 78 || overview.LaunchCount != 12 || overview.AverageSessionSeconds != 6 {
		t.Fatalf("unexpected overview: %+v", overview)
	}
	if overview.MostPlayedInstanceID == nil || *overview.MostPlayedInstanceID != "most-played" {
		t.Fatalf("unexpected most-played instance: %+v", overview.MostPlayedInstanceID)
	}
	if len(overview.RecentSessions) != 10 {
		t.Fatalf("unexpected recent session count: %d", len(overview.RecentSessions))
	}
	if overview.RecentSessions[9].DurationSec != 10 {
		t.Fatalf("overview did not preserve reader ordering: %+v", overview.RecentSessions)
	}
	playtime, err := service.InstancePlaytime(context.Background(), "most-played")
	if err != nil || playtime != 45 {
		t.Fatalf("unexpected instance playtime: %d, %v", playtime, err)
	}
}

func TestOverviewPropagatesReaderErrors(t *testing.T) {
	want := errors.New("sessions unavailable")
	service := NewService(&recordingReader{err: want})

	if _, err := service.Overview(context.Background()); !errors.Is(err, want) {
		t.Fatalf("Overview error = %v, want %v", err, want)
	}
	if _, err := service.InstancePlaytime(context.Background(), "instance"); !errors.Is(err, want) {
		t.Fatalf("InstancePlaytime error = %v, want %v", err, want)
	}
}

func TestOverviewIsNotLimitedToRecentSessionHistory(t *testing.T) {
	reader := &recordingReader{playSessions: make([]sessions.PlaySession, 5001)}
	for index := range reader.playSessions {
		reader.playSessions[index] = sessions.PlaySession{InstanceID: "instance", DurationSec: 1}
	}
	service := NewService(reader)

	overview, err := service.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if overview.LaunchCount != 5001 || overview.TotalPlaytimeSeconds != 5001 || len(overview.RecentSessions) != 10 {
		t.Fatalf("unexpected large-history overview: %+v", overview)
	}
}
