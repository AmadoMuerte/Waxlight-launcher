package instances

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/snapshots"
)

type mutationRepository struct {
	instance  Instance
	getErr    error
	saveErr   error
	deleteErr error
	saved     *Instance
	deleted   string
	calls     *[]string
	onDelete  func()
}

func (repository *mutationRepository) GetInstance(context.Context, string) (Instance, error) {
	*repository.calls = append(*repository.calls, "get")
	return repository.instance, repository.getErr
}

func (repository *mutationRepository) SaveInstance(_ context.Context, instance Instance) error {
	*repository.calls = append(*repository.calls, "save")
	repository.saved = &instance
	return repository.saveErr
}

func (repository *mutationRepository) DeleteInstance(_ context.Context, id string) error {
	*repository.calls = append(*repository.calls, "delete")
	if repository.onDelete != nil {
		repository.onDelete()
	}
	if repository.deleteErr == nil {
		repository.deleted = id
	}
	return repository.deleteErr
}

// testLock implements MutationLock with a single guard callback that records
// its calls.
type testLock struct {
	guard func(string, string, string) (func(), error)
}

func (lock *testLock) Guard(instanceID, marker, runningMessage string) (func(), error) {
	if lock.guard == nil {
		return func() {}, nil
	}
	return lock.guard(instanceID, marker, runningMessage)
}

func (lock *testLock) Lock(instanceID, marker string) (func(), error) {
	return lock.Guard(instanceID, marker, "")
}

func TestUpdateServicePreservesOwnedFieldsAndOrdersSideEffects(t *testing.T) {
	var calls []string
	oldAccount := "old-account"
	newAccount := "new-account"
	createdAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	lastPlayedAt := createdAt.Add(time.Hour)
	repository := &mutationRepository{
		instance: Instance{
			ID: "instance", Name: "Old", GameVersionID: "1.19", DefaultAccountID: &oldAccount,
			Directory: "/instances/owned", Status: StatusRunning, CreatedAt: createdAt, LastPlayedAt: &lastPlayedAt,
		},
		calls: &calls,
	}
	gate := &testGate{}
	events := &eventRecorder{onPublish: func() { calls = append(calls, "publish") }}
	lock := &testLock{guard: func(instanceID, marker, _ string) (func(), error) {
		calls = append(calls, "lock")
		return func() { calls = append(calls, "release") }, nil
	}}
	snapshotter := &testSnapshotter{onCreate: func() { calls = append(calls, "snapshot") }}
	service := NewUpdateService(
		repository,
		versionReader{},
		gate,
		lock,
		snapshotter,
		func(path string) error {
			calls = append(calls, "clear")
			if path != filepath.Join(repository.instance.Directory, "clientsettings.json") {
				t.Fatalf("clear path = %q", path)
			}
			return nil
		},
		events,
		func() time.Time { return time.Date(2026, 8, 11, 13, 0, 0, 0, time.FixedZone("test", 3600)) },
	)

	coverPath := "/covers/new.png"
	updated, err := service.Update(context.Background(), Instance{
		ID: "instance", Name: "  New name  ", Description: "Description", GameVersionID: "1.20",
		DefaultAccountID: &newAccount, Directory: "/ignored", CoverPath: &coverPath, Status: StatusReady,
		LaunchArguments: []string{"--foo"}, CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "New name" || updated.Directory != repository.instance.Directory || updated.Status != StatusRunning {
		t.Fatalf("updated instance = %+v", updated)
	}
	if !updated.CreatedAt.Equal(createdAt) || updated.LastPlayedAt != &lastPlayedAt || updated.CoverPath != &coverPath {
		t.Fatalf("preserved fields = %+v", updated)
	}
	wantUpdatedAt := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	if !updated.UpdatedAt.Equal(wantUpdatedAt) {
		t.Fatalf("UpdatedAt = %v, want %v", updated.UpdatedAt, wantUpdatedAt)
	}
	if repository.saved == nil || !reflect.DeepEqual(*repository.saved, updated) {
		t.Fatalf("saved = %+v", repository.saved)
	}
	if !reflect.DeepEqual(calls, []string{"get", "lock", "snapshot", "clear", "save", "publish", "release"}) {
		t.Fatalf("calls = %v", calls)
	}
	if len(events.events) != 1 || events.events[0].name != "instance:updated" || !reflect.DeepEqual(events.events[0].payload, updated) {
		t.Fatalf("events = %+v", events.events)
	}
	if !gate.ended {
		t.Fatal("mutation gate was not released")
	}
}

// testSnapshotter records snapshot creation calls.
type testSnapshotter struct {
	onCreate func()
	err      error
}

func (snapshotter *testSnapshotter) Create(_ context.Context, _ string, _ snapshots.Reason, _ map[string]string) error {
	if snapshotter.onCreate != nil {
		snapshotter.onCreate()
	}
	return snapshotter.err
}

func TestUpdateServiceStopsBeforeSaveOnValidationFailure(t *testing.T) {
	var calls []string
	repository := &mutationRepository{instance: Instance{ID: "instance"}, calls: &calls}
	gate := &testGate{}
	service := NewUpdateService(repository, versionReader{}, gate, &testLock{}, nil, nil, nil, time.Now)

	if _, err := service.Update(context.Background(), Instance{ID: "instance", Name: "  "}); err == nil {
		t.Fatal("Update() error = nil")
	}
	if !reflect.DeepEqual(calls, []string{"get"}) || repository.saved != nil || !gate.ended {
		t.Fatalf("calls = %v, saved = %+v, gate ended = %v", calls, repository.saved, gate.ended)
	}
}

func TestUpdateServiceRejectsMissingVersionChangeLock(t *testing.T) {
	tests := []struct {
		name string
		lock MutationLock
	}{
		{name: "missing lock"},
		{name: "missing release", lock: &testLock{guard: func(string, string, string) (func(), error) {
			return nil, nil
		}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls []string
			repository := &mutationRepository{
				instance: Instance{ID: "instance", Name: "Old", GameVersionID: "1.19"},
				calls:    &calls,
			}
			service := NewUpdateService(repository, versionReader{}, &testGate{}, test.lock, nil, nil, nil, time.Now)

			if _, err := service.Update(context.Background(), Instance{ID: "instance", Name: "New", GameVersionID: "1.20"}); err == nil {
				t.Fatal("Update() error = nil")
			}
			if !reflect.DeepEqual(calls, []string{"get"}) || repository.saved != nil {
				t.Fatalf("calls = %v, saved = %+v", calls, repository.saved)
			}
		})
	}
}

func TestDeleteServiceStagesFilesDeletesRecordAndCleansTrash(t *testing.T) {
	var calls []string
	dataRoot := t.TempDir()
	directory := filepath.Join(dataRoot, "instance")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, ".waxlight-instance"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	repository := &mutationRepository{
		instance: Instance{ID: "instance", Directory: directory},
		calls:    &calls,
	}
	events := &eventRecorder{onPublish: func() { calls = append(calls, "publish") }}
	var telemetryEvents []string
	service := NewDeleteService(
		repository,
		&testGate{},
		&testLock{guard: func(string, string, string) (func(), error) {
			calls = append(calls, "guard")
			return func() { calls = append(calls, "release") }, nil
		}},
		func(path string) (func() error, func() error, error) {
			calls = append(calls, "stage")
			return StageDirectoryRemoval(path, dataRoot, ".waxlight-instance")
		},
		func(path string) error { calls = append(calls, "clear"); return nil },
		func(context.Context, string) error {
			calls = append(calls, "recovery")
			return errors.New("already removed")
		},
		events,
		func(_ context.Context, name string) {
			calls = append(calls, "telemetry")
			telemetryEvents = append(telemetryEvents, name)
		},
	)

	if err := service.Delete(context.Background(), "instance", true); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"guard", "get", "stage", "delete", "recovery", "publish", "telemetry", "release"}) {
		t.Fatalf("calls = %v", calls)
	}
	if repository.deleted != "instance" || len(telemetryEvents) != 1 || telemetryEvents[0] != telemetryEventInstanceDeleted {
		t.Fatalf("deleted = %q, telemetry = %v", repository.deleted, telemetryEvents)
	}
	payload, ok := events.events[0].payload.(map[string]string)
	if len(events.events) != 1 || events.events[0].name != "instance:deleted" || !ok || payload["id"] != "instance" {
		t.Fatalf("events = %+v", events.events)
	}
	if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("original directory still exists: %v", err)
	}
	if trash, err := filepath.Glob(filepath.Join(dataRoot, ".instance.waxlight-delete-*")); err != nil || len(trash) != 0 {
		t.Fatalf("staged directories = %v, error = %v", trash, err)
	}
}

func TestDeleteServiceRestoresFilesWhenDatabaseDeleteFails(t *testing.T) {
	var calls []string
	dataRoot := t.TempDir()
	directory := filepath.Join(dataRoot, "instance")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{".waxlight-instance": "", "world.txt": "user data"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want := errors.New("database unavailable")
	repository := &mutationRepository{
		instance:  Instance{ID: "instance", Directory: directory},
		deleteErr: want,
		calls:     &calls,
		onDelete: func() {
			if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("directory was not staged before DB delete: %v", err)
			}
			trash, err := filepath.Glob(filepath.Join(dataRoot, ".instance.waxlight-delete-*"))
			if err != nil || len(trash) != 1 {
				t.Fatalf("staged directories = %v, error = %v", trash, err)
			}
			if content, err := os.ReadFile(filepath.Join(trash[0], "world.txt")); err != nil || string(content) != "user data" {
				t.Fatalf("staged user data = %q, error = %v", content, err)
			}
		},
	}
	service := NewDeleteService(repository, &testGate{}, &testLock{}, func(path string) (func() error, func() error, error) {
		return StageDirectoryRemoval(path, dataRoot, ".waxlight-instance")
	}, nil, nil, nil, nil)

	if err := service.Delete(context.Background(), "instance", true); !errors.Is(err, want) {
		t.Fatalf("Delete() error = %v, want %v", err, want)
	}
	content, err := os.ReadFile(filepath.Join(directory, "world.txt"))
	if err != nil || string(content) != "user data" {
		t.Fatalf("restored user data = %q, error = %v", content, err)
	}
	if repository.deleted != "" {
		t.Fatal("database record was removed")
	}
}

func TestDeleteServiceRenameFailureDoesNotDeleteRecord(t *testing.T) {
	var calls []string
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "world.txt"), []byte("user data"), 0o600); err != nil {
		t.Fatal(err)
	}
	repository := &mutationRepository{instance: Instance{ID: "instance", Directory: directory}, calls: &calls}
	service := NewDeleteService(repository, &testGate{}, &testLock{}, func(path string) (func() error, func() error, error) {
		if path != directory {
			t.Fatalf("stage path = %q, want %q", path, directory)
		}
		return nil, nil, errors.New("rename failed")
	}, nil, nil, nil, nil)

	if err := service.Delete(context.Background(), "instance", true); err == nil {
		t.Fatal("Delete() error = nil")
	}
	if !reflect.DeepEqual(calls, []string{"get"}) || repository.deleted != "" {
		t.Fatalf("calls = %v, deleted = %q", calls, repository.deleted)
	}
	content, err := os.ReadFile(filepath.Join(directory, "world.txt"))
	if err != nil || string(content) != "user data" {
		t.Fatalf("original data = %q, error = %v", content, err)
	}
}

func TestDeleteServiceWithoutFilesPreservesDirectory(t *testing.T) {
	var calls []string
	directory := t.TempDir()
	repository := &mutationRepository{instance: Instance{ID: "instance", Directory: directory}, calls: &calls}
	service := NewDeleteService(repository, &testGate{}, &testLock{}, nil, nil, nil, nil, nil)

	if err := service.Delete(context.Background(), "instance", false); err != nil {
		t.Fatal(err)
	}
	if repository.deleted != "instance" {
		t.Fatalf("deleted = %q", repository.deleted)
	}
	if _, err := os.Stat(directory); err != nil {
		t.Fatalf("directory was removed: %v", err)
	}
}

func TestDeleteServiceReportsDatabaseAndRollbackFailures(t *testing.T) {
	var calls []string
	databaseErr := errors.New("database unavailable")
	restoreErr := errors.New("restore unavailable")
	repository := &mutationRepository{
		instance:  Instance{ID: "instance", Directory: "/instances/owned"},
		deleteErr: databaseErr,
		calls:     &calls,
	}
	service := NewDeleteService(repository, &testGate{}, &testLock{}, func(string) (func() error, func() error, error) {
		return func() error { return restoreErr }, func() error { return nil }, nil
	}, nil, nil, nil, nil)

	err := service.Delete(context.Background(), "instance", true)
	if !errors.Is(err, databaseErr) || !errors.Is(err, restoreErr) {
		t.Fatalf("Delete() error = %v, want database and restore failures", err)
	}
}

func TestDeleteServiceReturnsFinalCleanupFailureAfterRecordDeletion(t *testing.T) {
	var calls []string
	dataRoot := t.TempDir()
	directory := filepath.Join(dataRoot, "instance")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{".waxlight-instance": "", "world.txt": "user data"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cleanupErr := errors.New("cleanup unavailable")
	repository := &mutationRepository{instance: Instance{ID: "instance", Directory: directory}, calls: &calls}
	service := NewDeleteService(repository, &testGate{}, &testLock{}, func(path string) (func() error, func() error, error) {
		restore, _, err := StageDirectoryRemoval(path, dataRoot, ".waxlight-instance")
		return restore, func() error { return cleanupErr }, err
	}, nil, nil, nil, nil)

	if err := service.Delete(context.Background(), "instance", true); !errors.Is(err, cleanupErr) {
		t.Fatalf("Delete() error = %v, want %v", err, cleanupErr)
	}
	if repository.deleted != "instance" {
		t.Fatalf("deleted = %q", repository.deleted)
	}
	trash, err := filepath.Glob(filepath.Join(dataRoot, ".instance.waxlight-delete-*"))
	if err != nil || len(trash) != 1 {
		t.Fatalf("recoverable staged directories = %v, error = %v", trash, err)
	}
	if content, err := os.ReadFile(filepath.Join(trash[0], "world.txt")); err != nil || string(content) != "user data" {
		t.Fatalf("recoverable user data = %q, error = %v", content, err)
	}
}

func TestDeleteServiceDeletesRecordWhenDirectoryIsMissing(t *testing.T) {
	var calls []string
	dataRoot := t.TempDir()
	repository := &mutationRepository{
		instance: Instance{ID: "instance", Directory: filepath.Join(dataRoot, "missing")},
		calls:    &calls,
	}
	service := NewDeleteService(repository, &testGate{}, &testLock{}, func(path string) (func() error, func() error, error) {
		return StageDirectoryRemoval(path, dataRoot, ".waxlight-instance")
	}, nil, nil, nil, nil)

	if err := service.Delete(context.Background(), "instance", true); err != nil {
		t.Fatal(err)
	}
	if repository.deleted != "instance" {
		t.Fatalf("deleted = %q", repository.deleted)
	}
}

func TestDeleteServiceClearFailurePreservesRecord(t *testing.T) {
	var calls []string
	want := errors.New("clear settings")
	repository := &mutationRepository{instance: Instance{ID: "instance", Directory: "/instances/owned"}, calls: &calls}
	gate := &testGate{}
	service := NewDeleteService(
		repository,
		gate,
		&testLock{},
		nil,
		func(string) error { calls = append(calls, "clear"); return want },
		nil,
		nil,
		nil,
	)

	if err := service.Delete(context.Background(), "instance", false); !errors.Is(err, want) {
		t.Fatalf("Delete() error = %v, want %v", err, want)
	}
	if !reflect.DeepEqual(calls, []string{"get", "clear"}) || repository.deleted != "" || !gate.ended {
		t.Fatalf("calls = %v, deleted = %q, gate ended = %v", calls, repository.deleted, gate.ended)
	}
}

func TestDeleteServiceRequiresSafetyDependencies(t *testing.T) {
	tests := []struct {
		name    string
		lock    MutationLock
		remover DirectoryRemovalStager
		calls   []string
	}{
		{name: "missing guard"},
		{name: "guard rejects", lock: &testLock{guard: func(string, string, string) (func(), error) {
			return nil, errors.New("running")
		}}},
		{name: "missing remover", lock: &testLock{}, calls: []string{"get"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls []string
			repository := &mutationRepository{instance: Instance{ID: "instance"}, calls: &calls}
			service := NewDeleteService(repository, &testGate{}, test.lock, test.remover, nil, nil, nil, nil)

			if err := service.Delete(context.Background(), "instance", true); err == nil {
				t.Fatal("Delete() error = nil")
			}
			if !reflect.DeepEqual(calls, test.calls) || repository.deleted != "" {
				t.Fatalf("calls = %v, deleted = %q", calls, repository.deleted)
			}
		})
	}
}

func TestMutationServicesRejectWhenGateIsBusy(t *testing.T) {
	want := errors.New("relocation active")
	var calls []string
	repository := &mutationRepository{calls: &calls}
	gate := &testGate{err: want}

	updater := NewUpdateService(repository, versionReader{}, gate, &testLock{}, nil, nil, nil, time.Now)
	if _, err := updater.Update(context.Background(), Instance{}); !errors.Is(err, want) {
		t.Fatalf("Update() error = %v, want %v", err, want)
	}
	deleter := NewDeleteService(repository, gate, &testLock{}, nil, nil, nil, nil, nil)
	if err := deleter.Delete(context.Background(), "instance", true); !errors.Is(err, want) {
		t.Fatalf("Delete() error = %v, want %v", err, want)
	}
	if len(calls) != 0 || gate.ended {
		t.Fatalf("calls = %v, gate ended = %v", calls, gate.ended)
	}
}
