package instancedirectory_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/filesystem"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/instancedirectory"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/versions"
)

type repository struct {
	mu          sync.Mutex
	items       []instances.Instance
	saveErr     error
	saveEntered chan struct{}
	allowSave   chan struct{}
}

func (repository *repository) ListInstances(context.Context) ([]instances.Instance, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return append([]instances.Instance(nil), repository.items...), nil
}

func (repository *repository) SaveInstance(_ context.Context, instance instances.Instance) error {
	if repository.saveEntered != nil {
		select {
		case repository.saveEntered <- struct{}{}:
		default:
		}
		select {
		case <-repository.allowSave:
		case <-time.After(5 * time.Second):
			return errors.New("timed out waiting to allow instance save")
		}
	}
	if repository.saveErr != nil {
		return repository.saveErr
	}
	repository.mu.Lock()
	repository.items = append(repository.items, instance)
	repository.mu.Unlock()
	return nil
}

func (repository *repository) IsDirectoryUsed(_ context.Context, directory, _ string) (bool, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for _, instance := range repository.items {
		if instance.Directory == directory {
			return true, nil
		}
	}
	return false, nil
}

type versionsReader struct{}

func (versionsReader) Get(context.Context, string) (versions.GameVersion, error) {
	return versions.GameVersion{}, nil
}

func creator(repository *repository, storage *instancedirectory.Storage, root string, id func() string) *instances.CreateService {
	return instances.NewCreateService(
		repository,
		versionsReader{},
		nil,
		func(context.Context) (string, error) { return "en", nil },
		&testGate{},
		storage,
		nil,
		nil,
		root,
		func() time.Time { return time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC) },
		id,
	)
}

type testGate struct{}

func (*testGate) Begin() error { return nil }
func (*testGate) End()         {}

type blockingLayout struct {
	delegate instancedirectory.Layout
	entered  chan struct{}
	release  <-chan struct{}
}

func (layout blockingLayout) EnsureLayout(directory string) error {
	close(layout.entered)
	<-layout.release
	return layout.delegate.EnsureLayout(directory)
}

func TestStorageCreatesLayoutAndExactMarker(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "custom")
	service := creator(&repository{}, instancedirectory.New(filesystem.ModFileManager{}), root, func() string { return "exact-id" })

	instance, err := service.Create(context.Background(), instances.CreateInput{Name: "Test", GameVersionID: "1.20", Directory: directory})
	if err != nil {
		t.Fatal(err)
	}
	if instance.Directory != directory {
		t.Fatalf("directory = %q", instance.Directory)
	}
	for _, name := range []string{"Mods", "ModsDisabled", "Logs"} {
		info, err := os.Stat(filepath.Join(directory, name))
		if err != nil || !info.IsDir() {
			t.Fatalf("layout %s: %v", name, err)
		}
	}
	marker := filepath.Join(directory, ".waxlight-instance")
	content, err := os.ReadFile(marker)
	if err != nil || string(content) != "exact-id" {
		t.Fatalf("marker = %q, %v", content, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(marker)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("marker mode = %v", info.Mode().Perm())
		}
		logs, err := os.Stat(filepath.Join(directory, "Logs"))
		if err != nil {
			t.Fatal(err)
		}
		if logs.Mode().Perm() != 0o700 {
			t.Fatalf("logs mode = %v", logs.Mode().Perm())
		}
	}
}

func TestStorageRollsBackOwnedDirectoryAfterPersistenceFailure(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "owned")
	repository := &repository{saveErr: errors.New("database unavailable")}
	service := creator(repository, instancedirectory.New(filesystem.ModFileManager{}), root, func() string { return "failed-id" })

	if _, err := service.Create(context.Background(), instances.CreateInput{Name: "Test", GameVersionID: "1.20", Directory: directory}); err == nil {
		t.Fatal("expected persistence failure")
	}
	if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned directory survived rollback: %v", err)
	}
}

func TestStoragePreservesPreExistingCustomDirectoryAfterPersistenceFailure(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "custom")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(directory, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("user data"), 0o600); err != nil {
		t.Fatal(err)
	}
	repository := &repository{saveErr: errors.New("database unavailable")}
	service := creator(repository, instancedirectory.New(filesystem.ModFileManager{}), root, func() string { return "failed-id" })

	if _, err := service.Create(context.Background(), instances.CreateInput{Name: "Test", GameVersionID: "1.20", Directory: directory}); err == nil {
		t.Fatal("expected persistence failure")
	}
	content, err := os.ReadFile(sentinel)
	if err != nil || string(content) != "user data" {
		t.Fatalf("pre-existing content = %q, %v", content, err)
	}
	if _, err := os.Stat(filepath.Join(directory, ".waxlight-instance")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("marker survived rollback: %v", err)
	}
	for _, name := range []string{"Mods", "ModsDisabled", "Logs"} {
		if _, err := os.Stat(filepath.Join(directory, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("new %s directory survived rollback: %v", name, err)
		}
	}
}

func TestStorageRejectsSymlinkInstanceRoot(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "instance-link")
	if err := os.Symlink(target, directory); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink unavailable: %v", err)
		}
		t.Fatal(err)
	}
	service := creator(&repository{}, instancedirectory.New(filesystem.ModFileManager{}), root, func() string { return "symlink-id" })

	if _, err := service.Create(context.Background(), instances.CreateInput{Name: "Test", GameVersionID: "1.20", Directory: directory}); err == nil {
		t.Fatal("expected symlink root rejection")
	}
	for _, name := range []string{"Mods", "ModsDisabled", "Logs", ".waxlight-instance"} {
		if _, err := os.Stat(filepath.Join(target, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("target path %s was created: %v", name, err)
		}
	}
}

func TestStorageSerializesConcurrentSameDirectoryCreation(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "shared")
	repository := &repository{saveEntered: make(chan struct{}, 1), allowSave: make(chan struct{})}
	storage := instancedirectory.New(filesystem.ModFileManager{})
	var idMu sync.Mutex
	nextID := 0
	service := creator(repository, storage, root, func() string {
		idMu.Lock()
		defer idMu.Unlock()
		nextID++
		return []string{"first", "second"}[nextID-1]
	})

	results := make(chan error, 2)
	go func() {
		_, err := service.Create(context.Background(), instances.CreateInput{Name: "First", GameVersionID: "1.20", Directory: directory})
		results <- err
	}()
	select {
	case <-repository.saveEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first save")
	}
	go func() {
		_, err := service.Create(context.Background(), instances.CreateInput{Name: "Second", GameVersionID: "1.20", Directory: directory})
		results <- err
	}()
	close(repository.allowSave)

	var success, conflicts int
	for range 2 {
		var err error
		select {
		case err = <-results:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for creation result")
		}
		if err == nil {
			success++
			continue
		}
		var appError *errs.AppError
		if errors.As(err, &appError) && appError.Code == instances.ErrDirectoryConflict {
			conflicts++
			continue
		}
		t.Fatalf("unexpected error: %v", err)
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", success, conflicts)
	}
}

func TestIndependentStoragesReserveSameDirectoryBeforeLayout(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "shared")
	entered := make(chan struct{})
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()

	firstStorage := instancedirectory.New(blockingLayout{
		delegate: filesystem.ModFileManager{},
		entered:  entered,
		release:  release,
	})
	secondStorage := instancedirectory.New(filesystem.ModFileManager{})
	firstService := creator(&repository{}, firstStorage, root, func() string { return "winner-id" })
	secondService := creator(&repository{}, secondStorage, root, func() string { return "loser-id" })

	firstResult := make(chan error, 1)
	go func() {
		_, err := firstService.Create(context.Background(), instances.CreateInput{Name: "First", GameVersionID: "1.20", Directory: directory})
		firstResult <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first allocator to reserve the directory")
	}

	marker := filepath.Join(directory, ".waxlight-instance")
	content, err := os.ReadFile(marker)
	if err != nil || string(content) != "winner-id" {
		t.Fatalf("reserved marker = %q, %v", content, err)
	}
	for _, name := range []string{"Mods", "ModsDisabled", "Logs"} {
		if _, err := os.Stat(filepath.Join(directory, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("layout %s mutated before reservation conflict: %v", name, err)
		}
	}

	_, secondErr := secondService.Create(context.Background(), instances.CreateInput{Name: "Second", GameVersionID: "1.20", Directory: directory})
	var appError *errs.AppError
	if !errors.As(secondErr, &appError) || appError.Code != instances.ErrDirectoryConflict {
		t.Fatalf("second allocator error = %v", secondErr)
	}
	close(release)
	select {
	case err := <-firstResult:
		if err != nil {
			t.Fatalf("first allocator failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first allocator")
	}

	content, err = os.ReadFile(marker)
	if err != nil || string(content) != "winner-id" {
		t.Fatalf("winner marker = %q, %v", content, err)
	}
	for _, name := range []string{"Mods", "ModsDisabled", "Logs"} {
		info, err := os.Stat(filepath.Join(directory, name))
		if err != nil || !info.IsDir() {
			t.Fatalf("winner layout %s: %v", name, err)
		}
	}
}
