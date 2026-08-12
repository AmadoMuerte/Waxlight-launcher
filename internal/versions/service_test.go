package versions

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/downloads"
	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/mutations"
	"github.com/waxlight/waxlight-launcher/internal/operations"
	"github.com/waxlight/waxlight-launcher/internal/platform/versionfs"
)

type testRepository struct {
	mu         sync.Mutex
	versions   map[string]GameVersion
	operations map[string]operations.Operation
	reference  string
	saveErr    error
}

func newTestRepository() *testRepository {
	return &testRepository{versions: make(map[string]GameVersion), operations: make(map[string]operations.Operation)}
}

func (repository *testRepository) ListVersions(context.Context) ([]GameVersion, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	result := make([]GameVersion, 0, len(repository.versions))
	for _, version := range repository.versions {
		result = append(result, version)
	}
	return result, nil
}

func (repository *testRepository) GetVersion(_ context.Context, id string) (GameVersion, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	version, ok := repository.versions[id]
	if !ok {
		return GameVersion{}, errs.NewError(errs.ErrVersionNotFound, "Game version not found")
	}
	return version, nil
}

func (repository *testRepository) SaveVersion(_ context.Context, version GameVersion) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.saveErr != nil {
		return repository.saveErr
	}
	if _, exists := repository.versions[version.ID]; exists {
		return errs.NewError(errs.ErrVersionExists, "This game version is already installed")
	}
	repository.versions[version.ID] = version
	return nil
}

func (repository *testRepository) UpdateVersion(_ context.Context, version GameVersion) error {
	repository.mu.Lock()
	repository.versions[version.ID] = version
	repository.mu.Unlock()
	return nil
}

func (repository *testRepository) DeleteVersion(_ context.Context, id string) error {
	repository.mu.Lock()
	delete(repository.versions, id)
	repository.mu.Unlock()
	return nil
}

func (repository *testRepository) VersionReference(context.Context, string) (string, bool, error) {
	return repository.reference, repository.reference != "", nil
}

func (repository *testRepository) ListOperations(context.Context, int) ([]operations.Operation, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	result := make([]operations.Operation, 0, len(repository.operations))
	for _, operation := range repository.operations {
		result = append(result, operation)
	}
	return result, nil
}

func (repository *testRepository) SaveOperation(_ context.Context, operation operations.Operation) error {
	repository.mu.Lock()
	repository.operations[operation.ID] = operation
	repository.mu.Unlock()
	return nil
}

func (repository *testRepository) ReconcileInterruptedOperations(_ context.Context, finished time.Time, code, message string) (int64, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	var count int64
	for id, operation := range repository.operations {
		if operation.Status != operations.StatusQueued && operation.Status != operations.StatusRunning {
			continue
		}
		operation.Status = operations.StatusFailed
		operation.FinishedAt = &finished
		operation.ErrorCode = &code
		operation.ErrorMessage = &message
		repository.operations[id] = operation
		count++
	}
	return count, nil
}

func (repository *testRepository) DeleteFinishedOperation(_ context.Context, id string) error {
	repository.mu.Lock()
	delete(repository.operations, id)
	repository.mu.Unlock()
	return nil
}

func (repository *testRepository) ClearFinishedOperations(context.Context) (int64, error) {
	return 0, nil
}

type testOwner struct {
	ctx context.Context
}

func (owner testOwner) Go(worker func(context.Context)) bool {
	go worker(owner.ctx)
	return true
}

type testCatalog []AvailableGameVersion

func (catalog testCatalog) List(context.Context) ([]AvailableGameVersion, error) {
	return append([]AvailableGameVersion(nil), catalog...), nil
}

type testGate struct{ err error }

func (gate testGate) Begin() error { return gate.err }
func (testGate) End()              {}

type testSpace int64

func (space testSpace) Available(string) (int64, error) { return int64(space), nil }

type testFilesystem struct {
	mu               sync.Mutex
	executableExists bool
	removedDownload  bool
	removedInstall   bool
	removedVersion   bool
	markerErr        error
}

func (filesystem *testFilesystem) DownloadPath(filename string) string {
	return "/downloads/" + filename
}
func (filesystem *testFilesystem) VersionPath(id string) string     { return "/versions/" + id }
func (filesystem *testFilesystem) ExecutableExists(string) bool     { return filesystem.executableExists }
func (filesystem *testFilesystem) MakeExecutable(string) error      { return nil }
func (filesystem *testFilesystem) WriteMarker(string, string) error { return filesystem.markerErr }
func (filesystem *testFilesystem) RemoveVersionsRootIfEmpty(string) error {
	return nil
}
func (filesystem *testFilesystem) RemoveDownload(string) error {
	filesystem.mu.Lock()
	filesystem.removedDownload = true
	filesystem.mu.Unlock()
	return nil
}
func (filesystem *testFilesystem) RemoveInstallTarget(string, string) error {
	filesystem.mu.Lock()
	filesystem.removedInstall = true
	filesystem.mu.Unlock()
	return nil
}
func (filesystem *testFilesystem) RemoveVersion(string, string) error {
	filesystem.mu.Lock()
	filesystem.removedVersion = true
	filesystem.mu.Unlock()
	return nil
}

type testLocalInstaller struct {
	started chan struct{}
	release chan struct{}
}

func (installer testLocalInstaller) Install(ctx context.Context, _, target, _, _ string, _ func(int64, int64)) (string, int64, error) {
	if installer.started != nil {
		close(installer.started)
	}
	if installer.release != nil {
		select {
		case <-installer.release:
		case <-ctx.Done():
			return "", 0, ctx.Err()
		}
	}
	return target + "/Vintagestory", 4, nil
}

func (testLocalInstaller) FindExecutable(root, _ string) (string, error) {
	return root + "/Vintagestory", nil
}

type testPackageInstaller struct{}

func (testPackageInstaller) Install(_ context.Context, _, target string, progress func(int64, int64)) (string, int64, error) {
	progress(4, 4)
	return target + "/Vintagestory", 4, nil
}

type testDownloader struct {
	started chan struct{}
	block   bool
	calls   int
	mu      sync.Mutex
}

func (downloader *testDownloader) Download(ctx context.Context, _ downloads.Request, progress chan<- downloads.Progress) error {
	downloader.mu.Lock()
	downloader.calls++
	downloader.mu.Unlock()
	if downloader.started != nil {
		close(downloader.started)
	}
	if downloader.block {
		<-ctx.Done()
		return ctx.Err()
	}
	progress <- downloads.Progress{DownloadedBytes: 4, TotalBytes: 4}
	return nil
}

func (*testDownloader) ContentLength(context.Context, string) (int64, error) { return 0, nil }

type pathSafeTestFilesystem struct {
	*testFilesystem
	paths versionfs.Filesystem
}

func (filesystem pathSafeTestFilesystem) VersionPath(id string) string {
	return filesystem.paths.VersionPath(id)
}

type concurrentLocalInstaller struct {
	started chan string
	release chan struct{}
}

func (installer concurrentLocalInstaller) Install(ctx context.Context, _, target, _, _ string, _ func(int64, int64)) (string, int64, error) {
	installer.started <- target
	select {
	case <-installer.release:
		return filepath.Join(target, "Vintagestory"), 4, nil
	case <-ctx.Done():
		return "", 0, ctx.Err()
	}
}

func (concurrentLocalInstaller) FindExecutable(root, _ string) (string, error) {
	return filepath.Join(root, "Vintagestory"), nil
}

func newTestService(repository *testRepository, catalog Catalog, downloader Downloader, local LocalInstaller, filesystem Filesystem) (*Capabilities, *operations.Manager) {
	manager := operations.NewManager(repository, testOwner{ctx: context.Background()}, nil)
	runtime := NewInstallRuntime(filesystem, testGate{}, manager, time.Now, func() string {
		return time.Now().Format("150405.000000000")
	})
	query := NewQueryService(repository, catalog, local, filesystem, time.Now)
	service := NewCapabilities(
		query,
		NewLocalInstallService(repository, local, runtime, "linux", "amd64"),
		NewCatalogInstallService(repository, query, downloader, testPackageInstaller{}, testSpace(1<<30), runtime, nil, "/data"),
		NewRemovalService(repository, repository, filesystem, testGate{}, nil),
	)
	return service, manager
}

func catalogVersion(id string) AvailableGameVersion {
	return AvailableGameVersion{ID: id, Name: id, Filename: id + ".tar.gz", DownloadURL: "https://example.test/" + id, DownloadSize: 4}
}

func TestCatalogInstallReturnsStableResult(t *testing.T) {
	repository := newTestRepository()
	filesystem := &testFilesystem{}
	service, _ := newTestService(repository, testCatalog{catalogVersion("1.22")}, &testDownloader{}, testLocalInstaller{}, filesystem)

	install, err := service.InstallCatalog(context.Background(), "1.22")
	if err != nil {
		t.Fatal(err)
	}
	installed, err := install.Wait(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if installed.ID != "1.22" || installed.ExecutablePath != "/versions/1.22/Vintagestory" {
		t.Fatalf("unexpected install result: %+v", installed)
	}
}

func TestCatalogFilenameTraversalIsRejected(t *testing.T) {
	repository := newTestRepository()
	downloader := &testDownloader{}
	release := catalogVersion("1.22")
	release.Filename = "../escape.tar.gz"
	service, _ := newTestService(repository, testCatalog{release}, downloader, testLocalInstaller{}, &testFilesystem{})

	_, err := service.InstallCatalog(context.Background(), release.ID)
	if !hasCode(err, errs.ErrVersionCatalog) {
		t.Fatalf("expected catalog error, got %v", err)
	}
	if downloader.calls != 0 {
		t.Fatal("unsafe catalog path reached downloader")
	}
}

func TestVersionIDRejectsControlCharactersAndExcessiveLength(t *testing.T) {
	repository := newTestRepository()
	service, _ := newTestService(repository, nil, nil, testLocalInstaller{}, &testFilesystem{})
	for _, id := range []string{"bad\x00id", "bad\u0085id", strings.Repeat("a", 181)} {
		if _, err := service.InstallLocal(context.Background(), id, id, "/source", "", ""); !hasCode(err, errs.ErrValidation) {
			t.Fatalf("unsafe version ID %q error = %v", id, err)
		}
	}
}

func TestLocalAndCatalogInstallsShareClaim(t *testing.T) {
	repository := newTestRepository()
	local := testLocalInstaller{started: make(chan struct{}), release: make(chan struct{})}
	service, _ := newTestService(repository, testCatalog{catalogVersion("1.22")}, &testDownloader{}, local, &testFilesystem{})
	done := make(chan error, 1)
	go func() {
		_, err := service.InstallLocal(context.Background(), "1.22", "1.22", "/source", "", "")
		done <- err
	}()
	<-local.started

	if _, err := service.InstallCatalog(context.Background(), "1.22"); !hasCode(err, errs.ErrVersionExists) {
		t.Fatalf("catalog install raced local install: %v", err)
	}
	close(local.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestUnsafeVersionIDsUseDistinctConcurrentClaimsAndSafePaths(t *testing.T) {
	repository := newTestRepository()
	installer := concurrentLocalInstaller{started: make(chan string, 2), release: make(chan struct{})}
	filesystem := pathSafeTestFilesystem{testFilesystem: &testFilesystem{}, paths: versionfs.New(t.TempDir())}
	service, _ := newTestService(repository, nil, nil, installer, filesystem)
	errors := make(chan error, 2)
	for _, id := range []string{"a/b", "a?b"} {
		id := id
		go func() {
			_, err := service.InstallLocal(context.Background(), id, id, "/source", "", "")
			errors <- err
		}()
	}
	first, second := <-installer.started, <-installer.started
	if first == second {
		t.Fatalf("concurrent unsafe IDs shared target %q", first)
	}
	root := filepath.Join(filesystem.paths.VersionPath("ordinary"), "..")
	for _, target := range []string{first, second} {
		relative, err := filepath.Rel(root, target)
		if err != nil || relative == ".." || filepath.IsAbs(relative) || len(relative) >= 3 && relative[:3] == ".."+string(filepath.Separator) {
			t.Fatalf("version target escaped root: %q (%q, %v)", target, relative, err)
		}
	}
	close(installer.release)
	for range 2 {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
}

func TestCatalogCancellationRemovesOperationAndDownload(t *testing.T) {
	repository := newTestRepository()
	filesystem := &testFilesystem{}
	downloader := &testDownloader{started: make(chan struct{}), block: true}
	service, manager := newTestService(repository, testCatalog{catalogVersion("1.22")}, downloader, testLocalInstaller{}, filesystem)
	install, err := service.InstallCatalog(context.Background(), "1.22")
	if err != nil {
		t.Fatal(err)
	}
	<-downloader.started
	if err := manager.Cancel(install.Operation.ID); err != nil {
		t.Fatal(err)
	}
	if !filesystem.removedDownload {
		t.Fatal("cancelled download was not removed")
	}
	if _, exists := repository.operations[install.Operation.ID]; exists {
		t.Fatal("cancelled operation was retained")
	}
}

func TestLocalCancellationRemovesOperationAndInstallTarget(t *testing.T) {
	repository := newTestRepository()
	filesystem := &testFilesystem{}
	installer := testLocalInstaller{started: make(chan struct{}), release: make(chan struct{})}
	service, manager := newTestService(repository, nil, nil, installer, filesystem)
	done := make(chan error, 1)
	go func() {
		_, err := service.InstallLocal(context.Background(), "1.22", "1.22", "/source", "", "")
		done <- err
	}()
	<-installer.started

	var operationID string
	repository.mu.Lock()
	for id := range repository.operations {
		operationID = id
	}
	repository.mu.Unlock()
	if err := manager.Cancel(operationID); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("InstallLocal error = %v, want cancellation", err)
	}
	if !filesystem.removedInstall {
		t.Fatal("cancelled local install target was not removed")
	}
	if _, exists := repository.operations[operationID]; exists {
		t.Fatal("cancelled local operation was retained")
	}
}

func TestLocalInstallCleansTargetAfterMarkerOrRepositoryFailure(t *testing.T) {
	for _, test := range []struct {
		name      string
		markerErr error
		saveErr   error
	}{
		{name: "marker", markerErr: errors.New("marker failed")},
		{name: "repository", saveErr: errors.New("save failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := newTestRepository()
			repository.saveErr = test.saveErr
			filesystem := &testFilesystem{markerErr: test.markerErr}
			service, _ := newTestService(repository, nil, nil, testLocalInstaller{}, filesystem)
			if _, err := service.InstallLocal(context.Background(), "1.22", "1.22", "/source", "", ""); err == nil {
				t.Fatal("expected install failure")
			}
			if !filesystem.removedInstall {
				t.Fatal("failed local install target was not removed")
			}
		})
	}
}

func TestCatalogInstallHoldsMutationGateForWorkerDuration(t *testing.T) {
	repository := newTestRepository()
	gate := &mutations.Gate{}
	filesystem := &testFilesystem{}
	downloader := &testDownloader{started: make(chan struct{}), block: true}
	manager := operations.NewManager(repository, testOwner{ctx: context.Background()}, nil)
	runtime := NewInstallRuntime(filesystem, gate, manager, time.Now, func() string { return "catalog-gated" })
	query := NewQueryService(repository, testCatalog{catalogVersion("1.22")}, testLocalInstaller{}, filesystem, time.Now)
	service := NewCatalogInstallService(repository, query, downloader, testPackageInstaller{}, testSpace(1<<30), runtime, nil, "/data")
	install, err := service.InstallCatalog(context.Background(), "1.22")
	if err != nil {
		t.Fatal(err)
	}
	<-downloader.started
	if err := gate.BeginRelocation(); err == nil {
		gate.EndRelocation()
		t.Fatal("relocation began while catalog worker was writing")
	}
	if err := manager.Cancel(install.Operation.ID); err != nil {
		t.Fatal(err)
	}
	if err := gate.BeginRelocation(); err != nil {
		t.Fatalf("gate remained held after catalog cancellation: %v", err)
	}
	gate.EndRelocation()
}

func TestListRepairsExecutableAndRemoveChecksReferences(t *testing.T) {
	repository := newTestRepository()
	repository.versions["1.20"] = GameVersion{ID: "1.20", InstallationDir: "/versions/1.20", ExecutablePath: "/missing"}
	filesystem := &testFilesystem{}
	service, _ := newTestService(repository, nil, nil, testLocalInstaller{}, filesystem)
	installed, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if installed[0].ExecutablePath != "/versions/1.20/Vintagestory" || installed[0].VerifiedAt == nil {
		t.Fatalf("version was not repaired: %+v", installed[0])
	}
	repository.reference = "World"
	if err := service.Remove(context.Background(), "1.20", true); !hasCode(err, errs.ErrValidation) {
		t.Fatalf("referenced version removal error = %v", err)
	}
	repository.reference = ""
	if err := service.Remove(context.Background(), "1.20", true); err != nil {
		t.Fatal(err)
	}
	if !filesystem.removedVersion {
		t.Fatal("version files were not removed")
	}
}

func hasCode(err error, code string) bool {
	var appError *errs.AppError
	return errors.As(err, &appError) && appError.Code == code
}
