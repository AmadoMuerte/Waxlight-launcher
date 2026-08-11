package instances

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/mods"
)

type cloneRepository struct {
	source    Instance
	getErr    error
	saveErr   error
	deleteErr error
	deleteCtx error
	saved     *Instance
	deleted   string
	calls     *[]string
}

func (repository *cloneRepository) GetInstance(context.Context, string) (Instance, error) {
	*repository.calls = append(*repository.calls, "get")
	return repository.source, repository.getErr
}

func (repository *cloneRepository) SaveInstance(_ context.Context, instance Instance) error {
	*repository.calls = append(*repository.calls, "save-instance")
	repository.saved = &instance
	return repository.saveErr
}

func (repository *cloneRepository) DeleteInstance(ctx context.Context, id string) error {
	*repository.calls = append(*repository.calls, "delete-instance")
	repository.deleted = id
	repository.deleteCtx = ctx.Err()
	return repository.deleteErr
}

type cloneModRepository struct {
	mods    []mods.InstalledMod
	listErr error
	saveErr error
	saved   []mods.InstalledMod
	calls   *[]string
}

func (repository *cloneModRepository) ListMods(context.Context, string) ([]mods.InstalledMod, error) {
	*repository.calls = append(*repository.calls, "list-mods")
	return repository.mods, repository.listErr
}

func (repository *cloneModRepository) SaveMod(_ context.Context, mod mods.InstalledMod) error {
	*repository.calls = append(*repository.calls, "save-mod")
	if repository.saveErr != nil {
		return repository.saveErr
	}
	repository.saved = append(repository.saved, mod)
	return nil
}

type cloneCreator struct {
	clone Instance
	err   error
	input CreateInput
	calls *[]string
}

func (creator *cloneCreator) Create(_ context.Context, input CreateInput) (Instance, error) {
	*creator.calls = append(*creator.calls, "create")
	creator.input = input
	return creator.clone, creator.err
}

type cloneStorage struct {
	copyErr    error
	copiedPath string
	copied     bool
	calls      *[]string
}

func (storage *cloneStorage) Copy(context.Context, string, string) error {
	*storage.calls = append(*storage.calls, "copy")
	return storage.copyErr
}

func (storage *cloneStorage) CopiedPath(string, string, string) (string, bool) {
	*storage.calls = append(*storage.calls, "cover")
	return storage.copiedPath, storage.copied
}

func TestCloneServiceCopiesMetadataModsAndCover(t *testing.T) {
	var calls []string
	accountID := "account"
	coverPath := "/instances/source/cover.png"
	source := Instance{
		ID: "source", Name: "Source", Description: "Description", GameVersionID: "1.20",
		DefaultAccountID: &accountID, Directory: "/instances/source", CoverPath: &coverPath,
		LaunchArguments: []string{"--foo"},
	}
	clone := Instance{ID: "clone", Name: "Clone", Directory: "/instances/clone"}
	repository := &cloneRepository{source: source, calls: &calls}
	mods := &cloneModRepository{calls: &calls, mods: []mods.InstalledMod{{
		ID: "source-mod", InstanceID: source.ID, FilePath: filepath.Join(source.Directory, "Mods", "test.zip"),
	}}}
	creator := &cloneCreator{clone: clone, calls: &calls}
	storage := &cloneStorage{calls: &calls, copied: true, copiedPath: filepath.Join(clone.Directory, "cover.png")}
	gate := &testGate{}
	service := NewCloneService(
		repository,
		mods,
		creator,
		gate,
		&testLock{guard: func(id, marker, _ string) (func(), error) {
			calls = append(calls, "guard")
			return func() { calls = append(calls, "release") }, nil
		}},
		storage,
		func(string) error { calls = append(calls, "remove"); return nil },
		func() time.Time { return time.Date(2026, 8, 11, 13, 0, 0, 0, time.FixedZone("test", 3600)) },
		func() string { return "clone-mod" },
	)

	result, err := service.Clone(context.Background(), source.ID, "Clone")
	if err != nil {
		t.Fatal(err)
	}
	if creator.input.Name != "Clone" || creator.input.Description != source.Description ||
		creator.input.GameVersionID != source.GameVersionID || creator.input.DefaultAccountID != source.DefaultAccountID ||
		!reflect.DeepEqual(creator.input.LaunchArguments, source.LaunchArguments) {
		t.Fatalf("create input = %+v", creator.input)
	}
	creator.input.LaunchArguments[0] = "changed"
	if source.LaunchArguments[0] != "--foo" {
		t.Fatal("source launch arguments share clone input storage")
	}
	if len(mods.saved) != 1 || mods.saved[0].ID != "clone-mod" || mods.saved[0].InstanceID != clone.ID ||
		mods.saved[0].FilePath != filepath.Join(clone.Directory, "Mods", "test.zip") {
		t.Fatalf("saved mods = %+v", mods.saved)
	}
	if result.CoverPath == nil || *result.CoverPath != storage.copiedPath || repository.saved == nil {
		t.Fatalf("result = %+v, saved = %+v", result, repository.saved)
	}
	wantTime := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	if !result.UpdatedAt.Equal(wantTime) {
		t.Fatalf("UpdatedAt = %v, want %v", result.UpdatedAt, wantTime)
	}
	wantCalls := []string{"guard", "get", "create", "copy", "list-mods", "save-mod", "cover", "save-instance", "release"}
	if !reflect.DeepEqual(calls, wantCalls) || !gate.ended {
		t.Fatalf("calls = %v, gate ended = %v", calls, gate.ended)
	}
}

func TestCloneServiceRollsBackWithoutReplacingPrimaryError(t *testing.T) {
	want := errors.New("copy failed")
	cleanupErr := errors.New("cleanup failed")
	var calls []string
	repository := &cloneRepository{source: Instance{ID: "source"}, deleteErr: cleanupErr, calls: &calls}
	creator := &cloneCreator{clone: Instance{ID: "clone", Name: "Clone", Directory: "/clone"}, calls: &calls}
	storage := &cloneStorage{copyErr: want, calls: &calls}
	service := NewCloneService(
		repository,
		&cloneModRepository{calls: &calls},
		creator,
		&testGate{},
		&testLock{guard: func(id, marker, _ string) (func(), error) {
			calls = append(calls, "guard")
			return func() { calls = append(calls, "release") }, nil
		}},
		storage,
		func(string) error { calls = append(calls, "remove"); return cleanupErr },
		time.Now,
		func() string { return "id" },
	)

	if _, err := service.Clone(context.Background(), "source", "Clone"); !errors.Is(err, want) {
		t.Fatalf("Clone() error = %v, want %v", err, want)
	}
	if repository.deleted != "" {
		t.Fatalf("record was deleted after directory cleanup failure: %q", repository.deleted)
	}
	wantCalls := []string{"guard", "get", "create", "copy", "remove", "release"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %v", calls)
	}
}

func TestCloneServiceRejectsBeforeReadingSource(t *testing.T) {
	want := errors.New("source is running")
	var calls []string
	repository := &cloneRepository{calls: &calls}
	gate := &testGate{}
	service := NewCloneService(
		repository,
		&cloneModRepository{calls: &calls},
		&cloneCreator{calls: &calls},
		gate,
		&testLock{guard: func(id, marker, _ string) (func(), error) {
			calls = append(calls, "guard")
			return nil, want
		}},
		&cloneStorage{calls: &calls},
		func(string) error { return nil },
		time.Now,
		func() string { return "id" },
	)

	if _, err := service.Clone(context.Background(), "source", "Clone"); !errors.Is(err, want) {
		t.Fatalf("Clone() error = %v, want %v", err, want)
	}
	if !reflect.DeepEqual(calls, []string{"guard"}) || !gate.ended {
		t.Fatalf("calls = %v, gate ended = %v", calls, gate.ended)
	}
}

func TestCloneServiceCleansRecordWithLiveContextAfterCancellation(t *testing.T) {
	var calls []string
	repository := &cloneRepository{source: Instance{ID: "source"}, calls: &calls}
	service := NewCloneService(
		repository,
		&cloneModRepository{calls: &calls},
		&cloneCreator{clone: Instance{ID: "clone", Directory: "/clone"}, calls: &calls},
		&testGate{},
		&testLock{},
		&cloneStorage{copyErr: context.Canceled, calls: &calls},
		func(string) error { return nil },
		time.Now,
		func() string { return "id" },
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := service.Clone(ctx, "source", "Clone"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Clone() error = %v, want %v", err, context.Canceled)
	}
	if repository.deleted != "clone" || repository.deleteCtx != nil {
		t.Fatalf("deleted = %q, cleanup context error = %v", repository.deleted, repository.deleteCtx)
	}
}

func TestCloneServiceRejectsExternalModPath(t *testing.T) {
	var calls []string
	wantSource := Instance{ID: "source", Directory: filepath.Join("root", "source")}
	repository := &cloneRepository{source: wantSource, calls: &calls}
	mods := &cloneModRepository{calls: &calls, mods: []mods.InstalledMod{{FilePath: filepath.Join("root", "external.zip")}}}
	service := NewCloneService(
		repository,
		mods,
		&cloneCreator{clone: Instance{ID: "clone", Directory: filepath.Join("root", "clone")}, calls: &calls},
		&testGate{},
		&testLock{},
		&cloneStorage{calls: &calls},
		func(string) error { return nil },
		time.Now,
		func() string { return "id" },
	)

	if _, err := service.Clone(context.Background(), "source", "Clone"); err == nil {
		t.Fatal("Clone() error = nil")
	}
	if len(mods.saved) != 0 || repository.deleted != "clone" {
		t.Fatalf("saved mods = %+v, deleted = %q", mods.saved, repository.deleted)
	}
}
