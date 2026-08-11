package instances

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

type createRepository struct {
	items              []Instance
	used               bool
	usedResults        []bool
	usedCalls          int
	checkedDirectories []string
	saveErr            error
	saved              *Instance
	onSave             func()
}

func (repository *createRepository) ListInstances(context.Context) ([]Instance, error) {
	return repository.items, nil
}

func (repository *createRepository) SaveInstance(_ context.Context, instance Instance) error {
	if repository.saveErr != nil {
		return repository.saveErr
	}
	repository.saved = &instance
	if repository.onSave != nil {
		repository.onSave()
	}
	return nil
}

func (repository *createRepository) IsDirectoryUsed(_ context.Context, directory, _ string) (bool, error) {
	repository.checkedDirectories = append(repository.checkedDirectories, directory)
	repository.usedCalls++
	if len(repository.usedResults) >= repository.usedCalls {
		return repository.usedResults[repository.usedCalls-1], nil
	}
	return repository.used, nil
}

type versionReader struct{ err error }

func (reader versionReader) Get(context.Context, string) (versions.GameVersion, error) {
	return versions.GameVersion{}, reader.err
}

type accountReader struct{ err error }

func (reader accountReader) GetAccount(context.Context, string) (accounts.Account, error) {
	return accounts.Account{}, reader.err
}

type testGate struct {
	err   error
	ended bool
}

func (gate *testGate) Begin() error { return gate.err }
func (gate *testGate) End()         { gate.ended = true }

type testAllocation struct {
	directory     string
	committed     bool
	rolledBack    bool
	allocateCalls int
	allocateErr   error
}

func (allocation *testAllocation) Directory() string { return allocation.directory }
func (allocation *testAllocation) Commit()           { allocation.committed = true }
func (allocation *testAllocation) Rollback() error {
	allocation.rolledBack = true
	return nil
}

type testDirectoryStorage struct{ allocation *testAllocation }

func (storage testDirectoryStorage) Allocate(directory, _ string) (DirectoryAllocation, error) {
	storage.allocation.allocateCalls++
	if storage.allocation.allocateErr != nil {
		return nil, storage.allocation.allocateErr
	}
	storage.allocation.directory = filepath.Clean(directory)
	return storage.allocation, nil
}

type publishedEvent struct {
	name    string
	payload any
}

type eventRecorder struct {
	events    []publishedEvent
	onPublish func()
}

func (recorder *eventRecorder) Publish(name string, payload any) {
	recorder.events = append(recorder.events, publishedEvent{name: name, payload: payload})
	if recorder.onPublish != nil {
		recorder.onPublish()
	}
}

func newCreateServiceForTest(
	repository *createRepository,
	version VersionReader,
	account AccountReader,
	gate *testGate,
	allocation *testAllocation,
	language string,
	events Publisher,
	telemetry TelemetryFunc,
) *CreateService {
	return NewCreateService(
		repository,
		version,
		account,
		func(context.Context) (string, error) { return language, nil },
		gate,
		testDirectoryStorage{allocation: allocation},
		events,
		telemetry,
		"/data",
		func() time.Time { return time.Date(2026, 8, 11, 12, 0, 0, 0, time.FixedZone("test", 3600)) },
		func() string { return "instance-id" },
	)
}

func TestCreateServicePersistsThenEmits(t *testing.T) {
	repository := &createRepository{}
	gate := &testGate{}
	allocation := &testAllocation{}
	var calls []string
	repository.onSave = func() { calls = append(calls, "save") }
	events := &eventRecorder{onPublish: func() { calls = append(calls, "publish") }}
	var telemetryEvents []string
	service := newCreateServiceForTest(repository, versionReader{}, accountReader{}, gate, allocation, "en", events, func(_ context.Context, name string) {
		calls = append(calls, "telemetry")
		telemetryEvents = append(telemetryEvents, name)
	})

	accountID := "account-id"
	instance, err := service.Create(context.Background(), CreateInput{
		Name: "  Test  ", Description: "  Description  ", GameVersionID: "1.20",
		DefaultAccountID: &accountID, LaunchArguments: []string{"--foo"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.saved == nil || repository.saved.ID != "instance-id" || repository.saved.Name != "Test" {
		t.Fatalf("saved instance = %+v", repository.saved)
	}
	if instance.Directory != filepath.Clean("/data/instances/instance-id") || instance.Description != "Description" {
		t.Fatalf("instance = %+v", instance)
	}
	if !instance.CreatedAt.Equal(time.Date(2026, 8, 11, 11, 0, 0, 0, time.UTC)) || instance.Status != StatusReady {
		t.Fatalf("timestamps/status = %v, %q", instance.CreatedAt, instance.Status)
	}
	if !allocation.committed || allocation.rolledBack || !gate.ended {
		t.Fatalf("allocation committed=%v rolledBack=%v gateEnded=%v", allocation.committed, allocation.rolledBack, gate.ended)
	}
	if len(events.events) != 1 || events.events[0].name != "instance:created" {
		t.Fatalf("events = %+v", events.events)
	}
	eventInstance, ok := events.events[0].payload.(Instance)
	if !ok || !reflect.DeepEqual(eventInstance, instance) {
		t.Fatalf("event payload = %#v", events.events[0].payload)
	}
	if len(telemetryEvents) != 1 || telemetryEvents[0] != "instance_created" {
		t.Fatalf("telemetry events = %v", telemetryEvents)
	}
	if !reflect.DeepEqual(calls, []string{"save", "publish", "telemetry"}) {
		t.Fatalf("call order = %v", calls)
	}
}

func TestCreateServiceDefaultAndLocalizedNames(t *testing.T) {
	defaultService := newCreateServiceForTest(&createRepository{}, versionReader{}, nil, &testGate{}, &testAllocation{}, "en", nil, nil)
	defaultInstance, err := defaultService.Create(context.Background(), CreateInput{GameVersionID: "1.20"})
	if err != nil {
		t.Fatal(err)
	}
	if defaultInstance.Name != "Instance" {
		t.Fatalf("default name = %q", defaultInstance.Name)
	}

	repository := &createRepository{items: []Instance{{Name: " сборка "}, {Name: "СБОРКА-2"}}}
	service := newCreateServiceForTest(repository, versionReader{}, nil, &testGate{}, &testAllocation{}, "ru", nil, nil)

	instance, err := service.Create(context.Background(), CreateInput{GameVersionID: "1.20"})
	if err != nil {
		t.Fatal(err)
	}
	if instance.Name != "Сборка-3" {
		t.Fatalf("name = %q", instance.Name)
	}
}

func TestCreateServiceValidatesVersionAndAccount(t *testing.T) {
	versionErr := domain.NewError(domain.ErrVersionNotFound, "Game version not found")
	service := newCreateServiceForTest(&createRepository{}, versionReader{err: versionErr}, nil, &testGate{}, &testAllocation{}, "en", nil, nil)
	if _, err := service.Create(context.Background(), CreateInput{Name: "Test", GameVersionID: "missing"}); !errors.Is(err, versionErr) {
		t.Fatalf("version error = %v", err)
	}

	accountErr := domain.NewError(domain.ErrAccountNotFound, "Account not found")
	accountID := "missing"
	service = newCreateServiceForTest(&createRepository{}, versionReader{}, accountReader{err: accountErr}, &testGate{}, &testAllocation{}, "en", nil, nil)
	if _, err := service.Create(context.Background(), CreateInput{Name: "Test", GameVersionID: "1.20", DefaultAccountID: &accountID}); !errors.Is(err, accountErr) {
		t.Fatalf("account error = %v", err)
	}
}

func TestCreateServiceRejectsMutationGate(t *testing.T) {
	want := domain.NewError(domain.ErrDataFolderBusy, "The data folder is being moved; wait for the relocation to finish")
	gate := &testGate{err: want}
	service := newCreateServiceForTest(&createRepository{}, versionReader{}, nil, gate, &testAllocation{}, "en", nil, nil)

	if _, err := service.Create(context.Background(), CreateInput{Name: "Test", GameVersionID: "1.20"}); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
	if gate.ended {
		t.Fatal("rejected mutation ended an unacquired gate slot")
	}
}

func TestCreateServiceRejectsDatabaseConflictBeforeAllocation(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "nested", "..", "custom")
	if err := os.Mkdir(filepath.Clean(directory), 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(directory, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("user data"), 0o600); err != nil {
		t.Fatal(err)
	}
	repository := &createRepository{used: true}
	allocation := &testAllocation{}
	service := newCreateServiceForTest(repository, versionReader{}, nil, &testGate{}, allocation, "en", nil, nil)

	_, err := service.Create(context.Background(), CreateInput{Name: "Test", GameVersionID: "1.20", Directory: directory})
	var appError *domain.AppError
	if !errors.As(err, &appError) || appError.Code != ErrDirectoryConflict || appError.Message != "The directory is already used by another instance" {
		t.Fatalf("error = %v", err)
	}
	if allocation.allocateCalls != 0 {
		t.Fatalf("allocation calls = %d", allocation.allocateCalls)
	}
	wantDirectory, absErr := filepath.Abs(directory)
	if absErr != nil {
		t.Fatal(absErr)
	}
	if !reflect.DeepEqual(repository.checkedDirectories, []string{wantDirectory}) {
		t.Fatalf("checked directories = %v", repository.checkedDirectories)
	}
	content, readErr := os.ReadFile(sentinel)
	if readErr != nil || string(content) != "user data" {
		t.Fatalf("pre-existing content = %q, %v", content, readErr)
	}
}

func TestCreateServiceRollsBackOnPostAllocationConflictAndPersistenceFailure(t *testing.T) {
	for _, test := range []struct {
		name       string
		repository *createRepository
		wantCode   string
	}{
		{name: "conflict", repository: &createRepository{usedResults: []bool{false, true}}, wantCode: ErrDirectoryConflict},
		{name: "persistence", repository: &createRepository{saveErr: errors.New("save failed")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			allocation := &testAllocation{}
			events := &eventRecorder{}
			var telemetryCount int
			service := newCreateServiceForTest(test.repository, versionReader{}, nil, &testGate{}, allocation, "en", events, func(context.Context, string) { telemetryCount++ })
			_, err := service.Create(context.Background(), CreateInput{Name: "Test", GameVersionID: "1.20"})
			if err == nil {
				t.Fatal("expected failure")
			}
			if test.wantCode != "" {
				var appError *domain.AppError
				if !errors.As(err, &appError) || appError.Code != test.wantCode {
					t.Fatalf("error = %v", err)
				}
			}
			if !allocation.rolledBack || allocation.committed {
				t.Fatalf("allocation committed=%v rolledBack=%v", allocation.committed, allocation.rolledBack)
			}
			if len(events.events) != 0 || telemetryCount != 0 {
				t.Fatalf("failure emitted events=%d telemetry=%d", len(events.events), telemetryCount)
			}
		})
	}
}

func TestCreateServiceReturnsAllocationError(t *testing.T) {
	want := errors.New("allocation failed")
	allocation := &testAllocation{allocateErr: want}
	repository := &createRepository{}
	events := &eventRecorder{}
	var telemetryCount int
	service := newCreateServiceForTest(repository, versionReader{}, nil, &testGate{}, allocation, "en", events, func(context.Context, string) { telemetryCount++ })

	if _, err := service.Create(context.Background(), CreateInput{Name: "Test", GameVersionID: "1.20"}); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
	if allocation.allocateCalls != 1 || allocation.committed || allocation.rolledBack {
		t.Fatalf("allocation calls=%d committed=%v rolledBack=%v", allocation.allocateCalls, allocation.committed, allocation.rolledBack)
	}
	if repository.saved != nil || len(events.events) != 0 || telemetryCount != 0 {
		t.Fatalf("saved=%v events=%d telemetry=%d", repository.saved, len(events.events), telemetryCount)
	}
}
