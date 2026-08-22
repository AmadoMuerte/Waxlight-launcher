package launching

import (
	"context"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/mutations"
	"github.com/waxlight/waxlight-launcher/internal/sessions"
)

type lkgTestProcess struct{}

func (lkgTestProcess) PID() int           { return 1 }
func (lkgTestProcess) Wait() (int, error) { return 1, nil }
func (lkgTestProcess) Stop() error        { return nil }
func (lkgTestProcess) Kill() error        { return nil }

type lkgTestInstances struct{}

func (lkgTestInstances) GetInstance(context.Context, string) (instances.Instance, error) {
	return instances.Instance{}, nil
}
func (lkgTestInstances) ListInstances(context.Context) ([]instances.Instance, error) { return nil, nil }
func (lkgTestInstances) SaveInstance(context.Context, instances.Instance) error      { return nil }

type lkgTestSessions struct{}

func (lkgTestSessions) Create(context.Context, sessions.PlaySession) error { return nil }
func (lkgTestSessions) Finish(context.Context, string, int, bool, int64) error {
	return nil
}

type lkgTestRecovery struct{ records atomic.Int32 }

func (recovery *lkgTestRecovery) RecordLastKnownGood(context.Context, instances.Instance) {
	recovery.records.Add(1)
}
func (*lkgTestRecovery) HandleFailedLaunch(instances.Instance) {}

func TestExitedLaunchIsNotRecordedWhileCleanupRuns(t *testing.T) {
	registry := NewRegistry(mutations.NewSlot())
	recovery := &lkgTestRecovery{}
	coordinator := &Coordinator{
		registry:  registry,
		instances: lkgTestInstances{},
		sessions:  lkgTestSessions{},
		recovery:  recovery,
		now:       time.Now,
	}
	instance := instances.Instance{ID: "instance", Name: "Test"}
	registry.Start(instance.ID, runningGame{sessionID: "session"})

	markerDone := make(chan struct{})
	go func() {
		coordinator.markLaunchEstablished(context.Background(), instance, "session", 20*time.Millisecond)
		close(markerDone)
	}()

	cleanupStarted := make(chan struct{})
	releaseCleanup := make(chan struct{})
	waitDone := make(chan struct{})
	go func() {
		coordinator.waitForGame(instance, lkgTestProcess{}, "session", time.Now(), io.NopCloser(nil), func() error { return nil }, func() {
			close(cleanupStarted)
			<-releaseCleanup
		}, time.Second, func() {})
		close(waitDone)
	}()

	<-cleanupStarted
	<-markerDone
	if got := recovery.records.Load(); got != 0 {
		t.Fatalf("RecordLastKnownGood calls = %d, want 0", got)
	}
	close(releaseCleanup)
	<-waitDone
}

func TestRunningLaunchIsRecordedAfterStartupWindow(t *testing.T) {
	registry := NewRegistry(mutations.NewSlot())
	recovery := &lkgTestRecovery{}
	coordinator := &Coordinator{registry: registry, recovery: recovery}
	instance := instances.Instance{ID: "instance", Name: "Test"}
	registry.Start(instance.ID, runningGame{sessionID: "session"})

	coordinator.markLaunchEstablished(context.Background(), instance, "session", 0)

	if got := recovery.records.Load(); got != 1 {
		t.Fatalf("RecordLastKnownGood calls = %d, want 1", got)
	}
}
