package settings

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/mutations"
)

type memoryRepository struct {
	value Settings
	saves int
}

func (repository *memoryRepository) GetSettings(context.Context) (Settings, error) {
	return repository.value, nil
}

func (repository *memoryRepository) SaveSettings(_ context.Context, value Settings) error {
	repository.value = value
	repository.saves++
	return nil
}

type consentRecorder struct{ calls int }

func (recorder *consentRecorder) SynchronizeConsent(change func() error) error {
	recorder.calls++
	return change()
}

type heartbeatRecorder struct{ calls int }

func (recorder *heartbeatRecorder) MaybeSendHeartbeat() { recorder.calls++ }

type limitRecorder struct{ value int }

func (recorder *limitRecorder) SetLimit(value int) { recorder.value = value }

func TestReaderNormalizesAndRepairsSettings(t *testing.T) {
	repository := &memoryRepository{value: Settings{
		Language: " RU_ru ", UpdateChannel: "invalid", DownloadsParallel: 3, LibrarySort: "invalid",
	}}
	value, err := NewReader(repository).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if value.Language != "ru" || value.UpdateChannel != "stable" || value.LibrarySort != LibrarySortLastPlayed {
		t.Fatalf("settings were not normalized: %+v", value)
	}
	if value.GlobalLaunchArguments == nil || repository.saves != 1 {
		t.Fatalf("repair was not persisted: %+v, saves=%d", value, repository.saves)
	}
}

func TestServiceSynchronizesConsentHeartbeatAndDownloadLimit(t *testing.T) {
	repository := &memoryRepository{value: Defaults()}
	consent := &consentRecorder{}
	heartbeat := &heartbeatRecorder{}
	limit := &limitRecorder{}
	service := NewService(repository, NewReader(repository), consent, heartbeat, limit)

	value := Defaults()
	value.Language = "BE_by"
	value.UpdateChannel = " PRERELEASE "
	value.SkippedUpdateVersion = " 1.2.3 "
	value.DownloadsParallel = 7
	value.TelemetryEnabled = true
	value.LibrarySort = LibrarySortGameVersion
	saved, err := service.Update(context.Background(), value)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Language != "be" || saved.UpdateChannel != "prerelease" || saved.SkippedUpdateVersion != "1.2.3" || saved.LibrarySort != LibrarySortGameVersion {
		t.Fatalf("unexpected normalized settings: %+v", saved)
	}
	if consent.calls != 1 || heartbeat.calls != 1 || limit.value != 7 {
		t.Fatalf("side effects: consent=%d heartbeat=%d limit=%d", consent.calls, heartbeat.calls, limit.value)
	}
	if _, err := service.Update(context.Background(), saved); err != nil {
		t.Fatal(err)
	}
	if heartbeat.calls != 1 {
		t.Fatalf("unchanged consent sent another heartbeat: %d", heartbeat.calls)
	}
}

func TestServiceUpdatesOnlyLibrarySort(t *testing.T) {
	repository := &memoryRepository{value: Defaults()}
	repository.value.Language = "de"
	service := NewService(repository, NewReader(repository), nil, nil, nil)

	saved, err := service.SetLibrarySort(context.Background(), LibrarySortName)
	if err != nil {
		t.Fatal(err)
	}
	if saved.LibrarySort != LibrarySortName || saved.Language != "de" {
		t.Fatalf("saved settings = %+v", saved)
	}
}

var errBusy = &testError{"busy"}

type testError struct{ message string }

func (err *testError) Error() string { return err.message }

type fakeChecker struct{ err error }

func (checker fakeChecker) CheckDataRootRelocation(context.Context) error { return checker.err }

type fakeRoot struct{ relocation Relocation }

func (fakeRoot) Current() (string, error)                          { return "/current", nil }
func (fakeRoot) Home() string                                      { return "/default" }
func (fakeRoot) ReadError() (string, error)                        { return "", nil }
func (root fakeRoot) PrepareRelocation(string) (Relocation, error) { return root.relocation, nil }

type fakeRelocation struct {
	run func(func(int64, int64)) error
}

func (relocation fakeRelocation) Run(_ context.Context, progress func(int64, int64)) error {
	return relocation.run(progress)
}

type testWorkers struct{ workers sync.WaitGroup }

func (owner *testWorkers) Go(worker func(context.Context)) bool {
	owner.workers.Add(1)
	go func() {
		defer owner.workers.Done()
		worker(context.Background())
	}()
	return true
}

type eventRecord struct {
	name    string
	payload any
}

type eventRecorder struct {
	mu     sync.Mutex
	events []eventRecord
}

func (recorder *eventRecorder) Publish(name string, payload any) {
	recorder.mu.Lock()
	recorder.events = append(recorder.events, eventRecord{name: name, payload: payload})
	recorder.mu.Unlock()
}

func (recorder *eventRecorder) snapshot() []eventRecord {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]eventRecord(nil), recorder.events...)
}

type fakeQuitter struct{ called chan struct{} }

func (quitter fakeQuitter) Quit(context.Context) {
	if quitter.called != nil {
		close(quitter.called)
	}
}

func TestDataRootMoveIsAtomicAndProgressCallbacksAreRaceSafe(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	relocation := fakeRelocation{run: func(progress func(int64, int64)) error {
		close(started)
		var callbacks sync.WaitGroup
		for index := int64(1); index <= 32; index++ {
			callbacks.Add(1)
			go func(copied int64) {
				defer callbacks.Done()
				progress(copied, 32)
			}(index)
		}
		callbacks.Wait()
		<-release
		return errBusy
	}}
	gate := &mutations.Gate{}
	workers := &testWorkers{}
	events := &eventRecorder{}
	service := NewDataRootService(fakeRoot{relocation: relocation}, gate, fakeChecker{}, workers, events, fakeQuitter{})
	if err := service.Move(context.Background(), "/target"); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := service.Move(context.Background(), "/other"); err == nil {
		t.Fatal("concurrent relocation acquired the gate")
	}
	close(release)
	workers.workers.Wait()
	if gate.Busy() {
		t.Fatal("failed relocation did not release the gate")
	}
	recorded := events.snapshot()
	if len(recorded) < 2 || recorded[0] != (eventRecord{name: "data-folder:progress", payload: RelocationProgress{Phase: "preparing"}}) {
		t.Fatalf("unexpected first event: %#v", recorded)
	}
	foundMoving := false
	for _, event := range recorded {
		progress, ok := event.payload.(RelocationProgress)
		if event.name == "data-folder:progress" && ok && progress.Phase == "moving" {
			foundMoving = progress.TotalBytes == 32 && progress.CopiedBytes > 0 && progress.Progress > 0
		}
	}
	if !foundMoving {
		t.Fatalf("moving payload was not published: %#v", recorded)
	}
	last := recorded[len(recorded)-1]
	if last.name != "data-folder:error" {
		t.Fatalf("unexpected final event: %#v", last)
	}
	payload, ok := last.payload.(map[string]string)
	if !ok || payload["message"] != "busy" {
		t.Fatalf("unexpected error payload: %#v", last.payload)
	}
}

func TestSuccessfulDataRootMoveKeepsGateThroughQuit(t *testing.T) {
	gate := &mutations.Gate{}
	workers := &testWorkers{}
	quit := make(chan struct{})
	events := &eventRecorder{}
	service := NewDataRootService(
		fakeRoot{relocation: fakeRelocation{run: func(func(int64, int64)) error { return nil }}},
		gate,
		fakeChecker{},
		workers,
		events,
		fakeQuitter{called: quit},
	)
	if err := service.Move(context.Background(), "/target"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-quit:
	case <-time.After(2 * time.Second):
		t.Fatal("quit was not requested")
	}
	workers.workers.Wait()
	if !gate.Busy() {
		t.Fatal("successful relocation released the gate before process exit")
	}
	recorded := events.snapshot()
	last := recorded[len(recorded)-1]
	if last != (eventRecord{name: "data-folder:progress", payload: RelocationProgress{Progress: 1, Phase: "relaunching"}}) {
		t.Fatalf("unexpected relaunch event: %#v", last)
	}
}
