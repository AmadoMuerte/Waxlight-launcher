package application_test

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/app"
	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/downloads"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/filesystem"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/versionfs"
	"github.com/waxlight/waxlight-launcher/internal/mutations"
	"github.com/waxlight/waxlight-launcher/internal/operations"
	"github.com/waxlight/waxlight-launcher/internal/platform/sqlite"
	settingscore "github.com/waxlight/waxlight-launcher/internal/settings"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

type recordingLauncher struct {
	executable       string
	arguments        []string
	workingDirectory string
	environment      map[string]string
	process          *controllableProcess
	startErr         error
}

func (launcher *recordingLauncher) Start(
	_ context.Context,
	executable string,
	arguments []string,
	workingDirectory string,
	environment map[string]string,
	_ io.Writer,
) (application.RunningProcess, error) {
	launcher.executable = executable
	launcher.arguments = append([]string(nil), arguments...)
	launcher.workingDirectory = workingDirectory
	launcher.environment = environment
	if launcher.startErr != nil {
		return nil, launcher.startErr
	}

	if launcher.process == nil {
		launcher.process = newControllableProcess()
	}

	return launcher.process, nil
}

type processResult struct {
	exitCode int
	err      error
}

type controllableProcess struct {
	result chan processResult
}

type staticVersionCatalog struct {
	versions []versions.AvailableGameVersion
}

func (catalog staticVersionCatalog) List(
	_ context.Context,
) ([]versions.AvailableGameVersion, error) {
	return append([]versions.AvailableGameVersion(nil), catalog.versions...), nil
}

type recordingDownloader struct {
	waitForCancellation bool
	cleanupFailure      bool
}

type switchingDownloader struct {
	mu      sync.RWMutex
	current downloads.Downloader
}

func (downloader *switchingDownloader) Set(current downloads.Downloader) {
	downloader.mu.Lock()
	downloader.current = current
	downloader.mu.Unlock()
}

func (downloader *switchingDownloader) Download(ctx context.Context, request downloads.Request, progress chan<- downloads.Progress) error {
	downloader.mu.RLock()
	current := downloader.current
	downloader.mu.RUnlock()
	return current.Download(ctx, request, progress)
}

func (downloader *switchingDownloader) ContentLength(ctx context.Context, url string) (int64, error) {
	downloader.mu.RLock()
	current := downloader.current
	downloader.mu.RUnlock()
	return current.ContentLength(ctx, url)
}

func (downloader recordingDownloader) Download(
	ctx context.Context,
	request downloads.Request,
	progress chan<- downloads.Progress,
) error {
	if downloader.waitForCancellation {
		if err := os.MkdirAll(filepath.Dir(request.DestinationPath), 0o755); err != nil {
			return err
		}
		if downloader.cleanupFailure {
			partial := request.DestinationPath + ".partial"
			if err := os.Mkdir(partial, 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(partial, "busy"), []byte("busy"), 0o644); err != nil {
				return err
			}
		} else {
			if err := os.WriteFile(
				request.DestinationPath+".partial",
				[]byte("unfinished package"),
				0o644,
			); err != nil {
				return err
			}
		}
		<-ctx.Done()
		return ctx.Err()
	}
	if err := os.MkdirAll(filepath.Dir(request.DestinationPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(request.DestinationPath, []byte("package"), 0o644); err != nil {
		return err
	}
	progress <- downloads.Progress{
		DownloadedBytes: 7,
		TotalBytes:      7,
		BytesPerSecond:  7,
	}
	return nil
}

func (downloader recordingDownloader) ContentLength(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

type fakeGamePackageInstaller struct{}

type fixedDiskSpace int64

func (space fixedDiskSpace) Available(string) (int64, error) {
	return int64(space), nil
}

type switchingDiskSpace struct {
	mu      sync.RWMutex
	current application.DiskSpaceChecker
}

func (space *switchingDiskSpace) Set(current application.DiskSpaceChecker) {
	space.mu.Lock()
	space.current = current
	space.mu.Unlock()
}

func (space *switchingDiskSpace) Available(path string) (int64, error) {
	space.mu.RLock()
	current := space.current
	space.mu.RUnlock()
	return current.Available(path)
}

func (fakeGamePackageInstaller) Install(
	_ context.Context,
	_ string,
	targetPath string,
	_ func(copied, total int64),
) (string, int64, error) {
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		return "", 0, err
	}
	executablePath := filepath.Join(targetPath, "Vintagestory")
	if err := os.WriteFile(executablePath, []byte("game"), 0o755); err != nil {
		return "", 0, err
	}
	return executablePath, 4, nil
}

func newControllableProcess() *controllableProcess {
	return &controllableProcess{
		result: make(chan processResult, 1),
	}
}

func (process *controllableProcess) PID() int {
	return 42
}

func (process *controllableProcess) Wait() (int, error) {
	result := <-process.result
	return result.exitCode, result.err
}

func (process *controllableProcess) Stop() error {
	process.result <- processResult{}
	return nil
}

func (process *controllableProcess) Kill() error {
	process.result <- processResult{
		exitCode: -1,
		err:      errors.New("killed"),
	}
	return nil
}

type testFixture struct {
	service    *application.Service
	store      *sqlite.SQLiteStore
	root       string
	executable string
	launcher   *recordingLauncher
	lifecycle  *app.Lifecycle
	operations *operations.Manager
	versions   *versions.Capabilities
	downloader *switchingDownloader
	diskSpace  *switchingDiskSpace
	settings   *settingscore.Reader
	updates    *settingscore.Service
	gate       *mutations.Gate
}

func newTestFixture(t *testing.T) testFixture {
	return newTestFixtureWithVersionDependencies(t, nil, recordingDownloader{}, fakeGamePackageInstaller{})
}

func newTestFixtureWithVersionDependencies(
	t *testing.T,
	catalog versions.Catalog,
	downloader downloads.Downloader,
	packageInstaller versions.PackageInstaller,
) testFixture {
	t.Helper()

	root := t.TempDir()
	store, err := sqlite.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}

	launcher := &recordingLauncher{}
	lifecycle := app.NewLifecycle()
	lifecycle.Startup(context.Background())
	operationManager := operations.NewManager(store, lifecycle, nil)
	gate := &mutations.Gate{}
	downloadSwitch := &switchingDownloader{current: downloader}
	diskSpace := &switchingDiskSpace{current: fixedDiskSpace(1 << 62)}
	settingsReader := settingscore.NewReader(store)
	settingsService := settingscore.NewService(store, settingsReader, nil, nil, nil)
	versionFilesystem := versionfs.New(root)
	archiveInstaller := filesystem.ArchiveInstaller{}
	versionRuntime := versions.NewInstallRuntime(versionFilesystem, gate, operationManager, time.Now, func() string { return fmt.Sprintf("version-%d", time.Now().UnixNano()) })
	versionQueries := versions.NewQueryService(store, catalog, archiveInstaller, versionFilesystem, time.Now)
	versionService := versions.NewCapabilities(
		versionQueries,
		versions.NewLocalInstallService(store, archiveInstaller, versionRuntime, runtime.GOOS, runtime.GOARCH),
		versions.NewCatalogInstallService(store, versionQueries, downloadSwitch, packageInstaller, diskSpace, versionRuntime, nil, root),
		versions.NewRemovalService(store, store, versionFilesystem, gate, nil),
	)
	service := application.NewService(
		store,
		filesystem.ModFileManager{},
		launcher,
		root,
		operationManager,
		versionService,
		downloadSwitch,
		diskSpace,
		gate,
		settingsReader,
	)
	t.Cleanup(func() {
		lifecycle.Shutdown()
		_ = service.Close()
	})

	versionDirectory := filepath.Join(root, "versions", "1.20")
	if err := os.MkdirAll(versionDirectory, 0o755); err != nil {
		t.Fatal(err)
	}

	executable := filepath.Join(versionDirectory, "Vintagestory")
	if err := os.WriteFile(executable, []byte("game"), 0o755); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	version := versions.GameVersion{
		ID:              "1.20",
		Name:            "1.20",
		Channel:         "stable",
		Platform:        "linux",
		Architecture:    "amd64",
		InstallationDir: versionDirectory,
		ExecutablePath:  executable,
		Status:          "installed",
		InstalledAt:     now,
	}
	if err := store.SaveVersion(context.Background(), version); err != nil {
		t.Fatal(err)
	}

	return testFixture{
		service:    service,
		store:      store,
		root:       root,
		executable: executable,
		launcher:   launcher,
		lifecycle:  lifecycle,
		operations: operationManager,
		versions:   versionService,
		downloader: downloadSwitch,
		diskSpace:  diskSpace,
		settings:   settingsReader,
		updates:    settingsService,
		gate:       gate,
	}
}

func (fixture testFixture) setDownloader(downloader downloads.Downloader) {
	fixture.downloader.Set(downloader)
}

func (fixture testFixture) setDiskSpace(space application.DiskSpaceChecker) {
	fixture.diskSpace.Set(space)
}

func TestCreateInstanceAndDirectoryConflict(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	customDirectory := filepath.Join(fixture.root, "custom")

	instance, err := fixture.service.CreateInstance(
		ctx,
		application.CreateInstanceInput{
			Name:          "Warm world",
			GameVersionID: "1.20",
			Directory:     customDirectory,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if instance.Status != "ready" {
		t.Fatalf("unexpected status %q", instance.Status)
	}

	for _, directoryName := range []string{"Mods", "ModsDisabled", "Logs"} {
		directoryPath := filepath.Join(customDirectory, directoryName)
		if _, err := os.Stat(directoryPath); err != nil {
			t.Fatalf("expected directory %q: %v", directoryName, err)
		}
	}

	_, err = fixture.service.CreateInstance(
		ctx,
		application.CreateInstanceInput{
			Name:          "Duplicate",
			GameVersionID: "1.20",
			Directory:     customDirectory,
		},
	)
	if err == nil {
		t.Fatal("expected a directory conflict")
	}
}

func TestCreateInstanceDefaultNameAndSuffixes(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	first, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{GameVersionID: "1.20"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Name != "Instance" {
		t.Fatalf("expected default name %q, got %q", "Instance", first.Name)
	}

	second, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{GameVersionID: "1.20"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Name != "Instance-2" {
		t.Fatalf("expected suffixed name %q, got %q", "Instance-2", second.Name)
	}

	third, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{GameVersionID: "1.20"})
	if err != nil {
		t.Fatal(err)
	}
	if third.Name != "Instance-3" {
		t.Fatalf("expected suffixed name %q, got %q", "Instance-3", third.Name)
	}

	// An explicitly typed name is never renumbered, even when it collides.
	explicit, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name:          "Instance",
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}
	if explicit.Name != "Instance" {
		t.Fatalf("explicit name was changed to %q", explicit.Name)
	}
}

func TestCreateInstanceLocalizedDefaultNames(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	if err := fixture.store.SaveSettings(ctx, settingscore.Settings{
		Language:              "ru",
		DownloadsParallel:     3,
		ConfirmDeletion:       true,
		GlobalLaunchArguments: []string{},
		CheckForUpdates:       true,
		UpdateChannel:         "stable",
	}); err != nil {
		t.Fatal(err)
	}

	first, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{GameVersionID: "1.20"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Name != "Сборка" {
		t.Fatalf("expected localized default name %q, got %q", "Сборка", first.Name)
	}

	second, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{GameVersionID: "1.20"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Name != "Сборка-2" {
		t.Fatalf("expected localized suffixed name %q, got %q", "Сборка-2", second.Name)
	}
}

func TestStartupReconciliationHardensExistingLogs(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{Name: "Logs", GameVersionID: "1.20"})
	if err != nil {
		t.Fatal(err)
	}
	logs := filepath.Join(instance.Directory, "Logs")
	logPath := filepath.Join(logs, "legacy.log")
	if err := os.WriteFile(logPath, []byte("safe log"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	accountService := newTestAccountService(fixture.store, &fakeAuthClient{}, newMemorySecretStore(), fixture.service.ClearAccountFromInstances)
	fixture.service.ConfigureAuthentication(accountService, filesystem.ClientSettingsService{})
	if err := fixture.service.ReconcileInjectedCredentials(ctx); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		directoryInfo, _ := os.Stat(logs)
		fileInfo, _ := os.Stat(logPath)
		if directoryInfo.Mode().Perm() != 0o700 || fileInfo.Mode().Perm() != 0o600 {
			t.Fatalf("logs were not hardened: directory=%o file=%o", directoryInfo.Mode().Perm(), fileInfo.Mode().Perm())
		}
	}
}

func TestDeleteVersionRemovesFilesAndEmptyLibraryDirectory(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	markerPath := filepath.Join(filepath.Dir(fixture.executable), ".waxlight-version")
	if err := os.WriteFile(markerPath, []byte("1.20"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(filepath.Dir(fixture.executable), "readme.txt"),
		[]byte("installed game"),
		0o444,
	); err != nil {
		t.Fatal(err)
	}

	if err := fixture.versions.Remove(ctx, "1.20", true); err != nil {
		t.Fatal(err)
	}
	versionDirectory := filepath.Dir(fixture.executable)
	if _, err := os.Stat(versionDirectory); !os.IsNotExist(err) {
		t.Fatalf("version directory still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(versionDirectory)); !os.IsNotExist(err) {
		t.Fatalf("empty versions directory still exists: %v", err)
	}
	if _, err := fixture.store.GetVersion(ctx, "1.20"); err == nil {
		t.Fatal("deleted version is still stored")
	}
}

func TestLocalModLifecycle(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	instance, err := fixture.service.CreateInstance(
		ctx,
		application.CreateInstanceInput{
			Name:          "Modded",
			GameVersionID: "1.20",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	sourcePath := filepath.Join(fixture.root, "sample.zip")
	if err := os.WriteFile(sourcePath, []byte("mod"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = fixture.service.InstallModFile(
		ctx,
		instance.ID,
		sourcePath,
		"Sample",
		"1.0",
	)
	if err != nil {
		t.Fatal(err)
	}

	mods, err := fixture.service.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 1 {
		t.Fatalf("expected one installed mod, got %d", len(mods))
	}
	if mods[0].Name != "Sample" || mods[0].Version != "1.0" {
		t.Fatalf("stored mod metadata was replaced during scan: %#v", mods[0])
	}

	disabledMod, err := fixture.service.SetModEnabled(ctx, mods[0].ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if disabledMod.Enabled {
		t.Fatal("the mod should be disabled")
	}
	if directoryName := filepath.Base(filepath.Dir(disabledMod.FilePath)); directoryName != "ModsDisabled" {
		t.Fatalf("unexpected disabled mod directory %q", directoryName)
	}

	if err := fixture.service.DeleteMod(ctx, disabledMod.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(disabledMod.FilePath); !os.IsNotExist(err) {
		t.Fatal("the deleted mod file still exists")
	}
}

func TestInstallModFilesBatch(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	instance, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name: "Batch", GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}

	makeMod := func(name string) string {
		path := filepath.Join(fixture.root, name+".zip")
		if err := os.WriteFile(path, []byte("mod"), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	first := makeMod("first")
	second := makeMod("second")
	unsupported := filepath.Join(fixture.root, "not-a-mod.txt")
	if err := os.WriteFile(unsupported, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := fixture.service.InstallModFiles(ctx, instance.ID, []string{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Installed) != 2 || len(result.Skipped) != 0 || len(result.Failed) != 0 {
		t.Fatalf("unexpected batch result: %#v", result)
	}

	duplicateResult, err := fixture.service.InstallModFiles(
		ctx,
		instance.ID,
		[]string{first, second},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(duplicateResult.Installed) != 0 {
		t.Fatalf("expected no new installs, got %#v", duplicateResult.Installed)
	}
	if len(duplicateResult.Skipped) != 2 {
		t.Fatalf("expected two skipped duplicates, got %#v", duplicateResult.Skipped)
	}
	if len(duplicateResult.Failed) != 0 {
		t.Fatalf("expected no failures, got %#v", duplicateResult.Failed)
	}

	partialResult, err := fixture.service.InstallModFiles(
		ctx,
		instance.ID,
		[]string{first, unsupported},
	)
	if err == nil {
		t.Fatal("expected an error when nothing could be installed")
	}
	if len(partialResult.Installed) != 0 || len(partialResult.Skipped) != 1 ||
		len(partialResult.Failed) != 1 || partialResult.Failed[0].Path != unsupported {
		t.Fatalf("unexpected partial result: %#v", partialResult)
	}

	mods, err := fixture.service.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 2 {
		t.Fatalf("expected two installed mods, got %d", len(mods))
	}

	allFailResult, err := fixture.service.InstallModFiles(ctx, instance.ID, []string{unsupported})
	if err == nil {
		t.Fatal("expected an error when nothing could be installed")
	}
	if len(allFailResult.Installed) != 0 || len(allFailResult.Failed) != 1 {
		t.Fatalf("unexpected all-fail result: %#v", allFailResult)
	}
}

func TestListModsReconcilesFilesAddedOutsideLauncher(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name: "Imported", GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}

	archivePath := filepath.Join(instance.Directory, "Mods", "smithingplus.zip")
	writeVintageStoryMod(t, archivePath, `{"modid":"smithingplus","name":"Smithing Plus","version":"2.4.1"}`)
	disabledPath := filepath.Join(instance.Directory, "ModsDisabled", "utility.dll")
	if err := os.WriteFile(disabledPath, []byte("mod"), 0o644); err != nil {
		t.Fatal(err)
	}

	mods, err := fixture.service.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 2 {
		t.Fatalf("expected two imported mods, got %#v", mods)
	}
	if mods[0].Name != "Smithing Plus" || mods[0].Version != "2.4.1" || !mods[0].Enabled || mods[0].Managed {
		t.Fatalf("unexpected imported archive: %#v", mods[0])
	}
	importedID := mods[0].ID

	movedPath := filepath.Join(instance.Directory, "ModsDisabled", filepath.Base(archivePath))
	if err := os.Rename(archivePath, movedPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(disabledPath); err != nil {
		t.Fatal(err)
	}
	mods, err = fixture.service.ListMods(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 1 || mods[0].ID != importedID || mods[0].Enabled || mods[0].FilePath != movedPath {
		t.Fatalf("filesystem changes were not reconciled: %#v", mods)
	}
	persisted, err := fixture.store.ListMods(ctx, instance.ID)
	if err != nil || len(persisted) != 1 {
		t.Fatalf("unexpected persisted mods: %#v, %v", persisted, err)
	}
}

func writeVintageStoryMod(t *testing.T, path, metadata string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create("modinfo.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(metadata)); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchUsesDetectedExecutableAndIsolatedDataPath(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	instance, err := fixture.service.CreateInstance(
		ctx,
		application.CreateInstanceInput{
			Name:            "Vanilla",
			GameVersionID:   "1.20",
			LaunchArguments: []string{"--debug"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	validation, err := fixture.service.ValidateLaunch(ctx, instance.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !validation.Valid {
		t.Fatalf("expected a valid launch, got issues: %v", validation.Issues)
	}
	if validation.Issues == nil || validation.Warnings == nil {
		t.Fatal("validation arrays must not be nil")
	}

	_, err = fixture.service.Launch(ctx, instance.ID, nil)
	if err != nil {
		t.Fatal(err)
	}

	if fixture.launcher.executable != fixture.executable {
		t.Fatalf("unexpected executable %q", fixture.launcher.executable)
	}
	expectedWorkingDirectory := filepath.Dir(fixture.executable)
	if fixture.launcher.workingDirectory != expectedWorkingDirectory {
		t.Fatalf(
			"unexpected working directory %q",
			fixture.launcher.workingDirectory,
		)
	}
	expectedArguments := []string{"--debug", "--dataPath", instance.Directory}
	if !reflect.DeepEqual(fixture.launcher.arguments, expectedArguments) {
		t.Fatalf("unexpected launch arguments: %v", fixture.launcher.arguments)
	}
	if fixture.launcher.environment["WAXLIGHT_INSTANCE_DIR"] != instance.Directory {
		t.Fatal("the instance environment variable was not provided")
	}

	if err := fixture.service.Stop(ctx, instance.ID, false); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchServerPassesConnectArgumentToSelectedInstance(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	instance, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{
		Name:          "Server instance",
		GameVersionID: "1.20",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.service.LaunchServer(ctx, instance.ID, nil, "example.org:42420"); err != nil {
		t.Fatal(err)
	}
	want := []string{"--dataPath", instance.Directory, "--connect", "example.org:42420"}
	if !reflect.DeepEqual(fixture.launcher.arguments, want) {
		t.Fatalf("unexpected server launch arguments: %v", fixture.launcher.arguments)
	}
	if err := fixture.service.Stop(ctx, instance.ID, false); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchServerRejectsUnsafeAddress(t *testing.T) {
	fixture := newTestFixture(t)
	if _, err := fixture.service.LaunchServer(context.Background(), "instance", nil, "example.org/unsafe"); err == nil {
		t.Fatal("expected invalid server address error")
	}
}

func TestSaveFavoriteServerAllowsWhitelistListingWithoutAddress(t *testing.T) {
	fixture := newTestFixture(t)
	server, err := fixture.service.SaveFavoriteServer(
		context.Background(),
		application.SaveFavoriteServerInput{Name: "Whitelist server"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if server.Address != "" || server.Name != "Whitelist server" {
		t.Fatalf("unexpected saved server: %#v", server)
	}
}

func TestAuthenticatedLaunchValidatesAndPatchesClientSettings(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	authClient := &fakeAuthClient{
		session: accounts.Session{
			SessionKey:       "session-key",
			SessionSignature: "session-signature",
			UID:              "player-uid",
			PlayerName:       "Waxlighter",
		},
		validateResult: true,
	}
	accountService := newTestAccountService(
		fixture.store,
		authClient,
		newMemorySecretStore(),
		fixture.service.ClearAccountFromInstances,
	)
	fixture.service.ConfigureAuthentication(
		accountService,
		filesystem.ClientSettingsService{},
	)
	login, err := accountService.Login(ctx, "player@example.com", "password")
	if err != nil || login.Account == nil {
		t.Fatalf("login failed: %#v, %v", login, err)
	}
	accountID := login.Account.ID
	instance, err := fixture.service.CreateInstance(
		ctx,
		application.CreateInstanceInput{
			Name:             "Authenticated",
			GameVersionID:    "1.20",
			DefaultAccountID: &accountID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(instance.Directory, "clientsettings.json")
	if err := os.WriteFile(
		settingsPath,
		[]byte(`{"intsettings":{"windowWidth":1920},"stringsettings":{"language":"en"}}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	session, err := fixture.service.Launch(ctx, instance.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if session.AccountID == nil || *session.AccountID != accountID {
		t.Fatalf("unexpected session account: %#v", session.AccountID)
	}
	if authClient.validationUID != "player-uid" || authClient.validationSecret != "session-key" {
		t.Fatal("launch did not validate the selected session")
	}
	contents, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]map[string]any
	if err := json.Unmarshal(contents, &settings); err != nil {
		t.Fatal(err)
	}
	if settings["intsettings"]["windowWidth"] != float64(1920) ||
		settings["stringsettings"]["language"] != "en" ||
		settings["stringsettings"]["sessionkey"] != "session-key" ||
		settings["stringsettings"]["sessionsignature"] != "session-signature" ||
		settings["stringsettings"]["playeruid"] != "player-uid" ||
		settings["stringsettings"]["playername"] != "Waxlighter" {
		t.Fatalf("unexpected patched settings: %#v", settings)
	}
	if err := fixture.service.Stop(ctx, instance.ID, false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		contents, err = os.ReadFile(settingsPath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(contents), "session-key") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("credentials were not cleaned after process exit")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if strings.Contains(strings.Join(fixture.launcher.arguments, " "), "session-key") ||
		strings.Contains(fmt.Sprint(fixture.launcher.environment), "session-key") {
		t.Fatal("credentials were passed through process arguments or environment")
	}
	if _, err := (filesystem.ClientSettingsService{}).Inject(settingsPath, accounts.Account{
		SessionKey: "stale-key", SessionSignature: "stale-signature", UID: "player-uid", Username: "Waxlighter",
	}); err != nil {
		t.Fatal(err)
	}
	if err := accountService.RemoveAccount(ctx, accountID); err != nil {
		t.Fatal(err)
	}
	contents, err = os.ReadFile(settingsPath)
	if err != nil || strings.Contains(string(contents), "stale-key") {
		t.Fatalf("account removal left instance credentials: %s, %v", contents, err)
	}
	updatedInstance, err := fixture.store.GetInstance(ctx, instance.ID)
	if err != nil || updatedInstance.DefaultAccountID != nil {
		t.Fatalf("account reference was not cleared: %#v, %v", updatedInstance, err)
	}
}

func TestAuthenticatedLaunchFailureCleansInjectedCredentials(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	authClient := &fakeAuthClient{session: accounts.Session{SessionKey: "WAXLIGHT_TEST_SESSION_KEY_DO_NOT_LEAK", SessionSignature: "signature", UID: "uid", PlayerName: "player"}, validateResult: true}
	accountService := newTestAccountService(fixture.store, authClient, newMemorySecretStore(), fixture.service.ClearAccountFromInstances)
	fixture.service.ConfigureAuthentication(accountService, filesystem.ClientSettingsService{})
	login, err := accountService.Login(ctx, "player@example.com", "password")
	if err != nil {
		t.Fatal(err)
	}
	instance, err := fixture.service.CreateInstance(ctx, application.CreateInstanceInput{Name: "Failure cleanup", GameVersionID: "1.20", DefaultAccountID: &login.Account.ID})
	if err != nil {
		t.Fatal(err)
	}
	fixture.launcher.startErr = errors.New("injected process start failure")
	if _, err := fixture.service.Launch(ctx, instance.ID, nil); err == nil {
		t.Fatal("expected launch failure")
	}
	contents, err := os.ReadFile(filepath.Join(instance.Directory, "clientsettings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "WAXLIGHT_TEST_SESSION_KEY_DO_NOT_LEAK") {
		t.Fatal("credential remained after launch failure")
	}
	if err := filepath.Walk(instance.Directory, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return walkErr
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(data), "WAXLIGHT_TEST_SESSION_KEY_DO_NOT_LEAK") {
			t.Fatalf("credential leaked to application file %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestExpiredSessionBlocksLaunch(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	authClient := &fakeAuthClient{
		session: accounts.Session{
			SessionKey:       "session-key",
			SessionSignature: "session-signature",
			UID:              "player-uid",
			PlayerName:       "Waxlighter",
		},
		validateResult: false,
	}
	accountService := newTestAccountService(
		fixture.store,
		authClient,
		newMemorySecretStore(),
		fixture.service.ClearAccountFromInstances,
	)
	fixture.service.ConfigureAuthentication(
		accountService,
		filesystem.ClientSettingsService{},
	)
	login, err := accountService.Login(ctx, "player@example.com", "password")
	if err != nil || login.Account == nil {
		t.Fatal(err)
	}
	accountID := login.Account.ID
	instance, err := fixture.service.CreateInstance(
		ctx,
		application.CreateInstanceInput{
			Name:             "Expired",
			GameVersionID:    "1.20",
			DefaultAccountID: &accountID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.Launch(ctx, instance.ID, nil); err == nil {
		t.Fatal("expired session did not block launch")
	}
	if fixture.launcher.executable != "" {
		t.Fatal("game process was started with an expired session")
	}
	account, err := fixture.store.GetAccount(ctx, accountID)
	if err != nil || account.Status != accounts.StatusExpired {
		t.Fatalf("account was not marked expired: %#v, %v", account, err)
	}
}

func TestValidationRepairsLegacyExecutablePath(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	version, err := fixture.store.GetVersion(ctx, "1.20")
	if err != nil {
		t.Fatal(err)
	}
	version.ExecutablePath = filepath.Join(version.InstallationDir, "vintagestory")
	if err := fixture.store.UpdateVersion(ctx, version); err != nil {
		t.Fatal(err)
	}

	instance, err := fixture.service.CreateInstance(
		ctx,
		application.CreateInstanceInput{
			Name:          "Legacy installation",
			GameVersionID: "1.20",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	validation, err := fixture.service.ValidateLaunch(ctx, instance.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !validation.Valid {
		t.Fatalf("legacy executable path was not repaired: %v", validation.Issues)
	}

	repairedVersion, err := fixture.store.GetVersion(ctx, "1.20")
	if err != nil {
		t.Fatal(err)
	}
	if repairedVersion.ExecutablePath != fixture.executable {
		t.Fatalf(
			"unexpected repaired executable path %q",
			repairedVersion.ExecutablePath,
		)
	}
}

func TestInstallingAnExistingVersionIsRejected(t *testing.T) {
	fixture := newTestFixture(t)

	_, err := fixture.versions.InstallLocal(
		context.Background(),
		"1.20",
		"Renamed version",
		filepath.Dir(fixture.executable),
		"",
		"",
	)
	if err == nil {
		t.Fatal("expected duplicate version installation to fail")
	}

	var applicationError *domain.AppError
	if !errors.As(err, &applicationError) {
		t.Fatalf("expected an application error, got %T", err)
	}
	if applicationError.Code != domain.ErrVersionExists {
		t.Fatalf("unexpected error code %q", applicationError.Code)
	}

	version, err := fixture.store.GetVersion(context.Background(), "1.20")
	if err != nil {
		t.Fatal(err)
	}
	if version.Name != "1.20" {
		t.Fatalf("the existing version was modified: %+v", version)
	}
}

func isErrorCode(err error, code string) bool {
	var applicationError *domain.AppError
	return errors.As(err, &applicationError) && applicationError.Code == code
}

func waitForOperationStatus(
	t *testing.T,
	store *sqlite.SQLiteStore,
	operationID string,
	wantedStatus string,
) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		operations, err := store.ListOperations(context.Background(), 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, operation := range operations {
			if operation.ID == operationID && operation.Status == wantedStatus {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("operation %s did not reach %s", operationID, wantedStatus)
}

func TestStatisticsAreCalculatedByBackend(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	instance, err := fixture.service.CreateInstance(
		ctx,
		application.CreateInstanceInput{
			Name:          "Stats",
			GameVersionID: "1.20",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	durations := map[string]int64{
		"first":  90,
		"second": 30,
	}
	for sessionID, duration := range durations {
		endedAt := now.Add(time.Duration(duration) * time.Second)
		session := domain.PlaySession{
			ID:          sessionID,
			InstanceID:  instance.ID,
			VersionID:   "1.20",
			StartedAt:   now,
			EndedAt:     &endedAt,
			DurationSec: duration,
		}
		if err := fixture.store.SaveSession(ctx, session); err != nil {
			t.Fatal(err)
		}
	}

	statistics, err := fixture.service.GetStatistics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if statistics.TotalPlaytimeSeconds != 120 {
		t.Fatalf("unexpected total playtime: %+v", statistics)
	}
	if statistics.LaunchCount != 2 {
		t.Fatalf("unexpected launch count: %+v", statistics)
	}
	if statistics.AverageSessionSeconds != 60 {
		t.Fatalf("unexpected average session: %+v", statistics)
	}
}

func TestSettingsLanguageNormalization(t *testing.T) {
	cases := map[string]string{
		"en": "en", "EN": "en", "en-US": "en",
		"ru": "ru", "RU": "ru", "ru_RU": "ru",
		"be": "be", "BE": "be", "be_BY": "be",
		"by": "be", "BY": "be", "by-BY": "be",
		"  ru-RU  ": "ru", "fr": "fr", "": "en", "it": "en",
	}
	for input, expected := range cases {
		t.Run(input, func(t *testing.T) {
			fixture := newTestFixture(t)
			ctx := context.Background()
			settings, err := fixture.settings.Get(ctx)
			if err != nil {
				t.Fatal(err)
			}
			settings.Language = input
			settings.DownloadsParallel = 4
			saved, err := fixture.updates.Update(ctx, settings)
			if err != nil {
				t.Fatal(err)
			}
			if saved.Language != expected {
				t.Fatalf("normalize %q: got %q, want %q", input, saved.Language, expected)
			}
			if saved.DownloadsParallel != 4 {
				t.Fatal("language save changed unrelated settings")
			}
		})
	}
}

func TestGetSettingsRepairsAndPersistsInvalidLanguage(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	settings, err := fixture.store.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.Language = "unsupported"
	if err := fixture.store.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}

	repaired, err := fixture.settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if repaired.Language != "en" {
		t.Fatalf("got %q", repaired.Language)
	}
	persisted, err := fixture.store.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Language != "en" {
		t.Fatalf("repair was not persisted: %q", persisted.Language)
	}
}

func TestSettingsUpdatePreferencesDefaultAndValidate(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	settings, err := fixture.settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !settings.CheckForUpdates || settings.UpdateChannel != "stable" {
		t.Fatalf("unexpected update defaults: %+v", settings)
	}
	settings.UpdateChannel = "nightly"
	if _, err := fixture.updates.Update(ctx, settings); err == nil {
		t.Fatal("expected invalid update channel to be rejected")
	}
}

func TestTelemetrySettingDefaultsDisabled(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	settings, err := fixture.settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if settings.TelemetryEnabled {
		t.Fatal("telemetry must default to disabled for new installations")
	}
}

func TestTelemetryExplicitEnableSurvivesReload(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	settings, err := fixture.settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.TelemetryEnabled = true
	if _, err := fixture.updates.Update(ctx, settings); err != nil {
		t.Fatal(err)
	}
	reloaded, err := fixture.settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.TelemetryEnabled {
		t.Fatal("explicitly enabled telemetry was silently disabled")
	}
}

func TestTelemetrySettingPersistsAcrossSaves(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	settings, err := fixture.settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.TelemetryEnabled = true
	saved, err := fixture.updates.Update(ctx, settings)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.TelemetryEnabled {
		t.Fatal("telemetry setting was not persisted")
	}
	reloaded, err := fixture.settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.TelemetryEnabled {
		t.Fatal("telemetry setting did not survive a reload")
	}

	reloaded.TelemetryEnabled = false
	if _, err := fixture.updates.Update(ctx, reloaded); err != nil {
		t.Fatal(err)
	}
	final, err := fixture.settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if final.TelemetryEnabled {
		t.Fatal("telemetry setting could not be disabled again")
	}
}

func TestSettingValuesRoundtrip(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()

	value, err := fixture.store.GetSettingValue(ctx, "missing_key")
	if err != nil {
		t.Fatalf("missing key returned an error: %v", err)
	}
	if value != "" {
		t.Fatalf("missing key returned %q", value)
	}

	if err := fixture.store.SetSettingValue(ctx, "telemetry_installation_id", "550e8400-e29b-41d4-a716-446655440000"); err != nil {
		t.Fatal(err)
	}
	value, err = fixture.store.GetSettingValue(ctx, "telemetry_installation_id")
	if err != nil {
		t.Fatal(err)
	}
	if value != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("roundtrip mismatch: %q", value)
	}
}

func TestRelocationGuardRejectsDiskOperations(t *testing.T) {
	fixture := newTestFixture(t)

	if err := fixture.service.CheckDataRootRelocation(context.Background()); err != nil {
		t.Fatalf("relocation should be allowed initially: %v", err)
	}

	if err := fixture.gate.BeginRelocation(); err != nil {
		t.Fatal(err)
	}

	if err := fixture.gate.BeginRelocation(); err == nil {
		t.Fatal("expected relocation to be rejected while busy")
	}

	_, err := fixture.service.CreateInstance(context.Background(), application.CreateInstanceInput{
		Name:          "blocked",
		GameVersionID: "1.20",
	})
	if err == nil {
		t.Fatal("expected instance creation to be rejected while relocating")
	}
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != domain.ErrDataFolderBusy {
		t.Fatalf("expected DATA_FOLDER_BUSY, got %v", err)
	}

	if err := fixture.service.DeleteMod(context.Background(), "missing", false); err == nil {
		t.Fatal("expected mod deletion to be rejected while relocating")
	}

	fixture.gate.EndRelocation()
	if err := fixture.service.CheckDataRootRelocation(context.Background()); err != nil {
		t.Fatalf("relocation should be allowed again: %v", err)
	}
}

type blockingClientSettings struct {
	started chan struct{}
	release chan struct{}
}

func (settings blockingClientSettings) Inject(string, accounts.Account) (func() error, error) {
	return func() error { return nil }, nil
}

func (settings blockingClientSettings) Clear(string) error {
	close(settings.started)
	<-settings.release
	return nil
}

func (blockingClientSettings) Reconcile(string) error { return nil }

func TestRelocationCannotBeginDuringAccountClientSettingsCleanup(t *testing.T) {
	fixture := newTestFixture(t)
	if _, err := fixture.service.CreateInstance(context.Background(), application.CreateInstanceInput{
		Name: "cleanup race", GameVersionID: "1.20",
	}); err != nil {
		t.Fatal(err)
	}
	settings := blockingClientSettings{started: make(chan struct{}), release: make(chan struct{})}
	fixture.service.ConfigureAuthentication(nil, settings)
	done := make(chan error, 1)
	go func() {
		done <- fixture.service.ClearAccountFromInstances(context.Background(), "account")
	}()
	<-settings.started
	if err := fixture.gate.BeginRelocation(); err == nil {
		fixture.gate.EndRelocation()
		t.Fatal("relocation began while client settings were being cleared")
	}
	close(settings.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := fixture.gate.BeginRelocation(); err != nil {
		t.Fatalf("relocation remained blocked after cleanup: %v", err)
	}
	fixture.gate.EndRelocation()
}

func TestInterruptedOperationReconciliationUnblocksRelocation(t *testing.T) {
	fixture := newTestFixture(t)
	now := time.Now().UTC()
	if err := fixture.store.SaveOperation(context.Background(), operations.Operation{
		ID: "stale", Type: "snapshot_create", Title: "Creating snapshot", Status: operations.StatusRunning, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.CheckDataRootRelocation(context.Background()); err == nil {
		t.Fatal("stale running operation did not initially block relocation")
	}
	if count, err := fixture.operations.ReconcileInterrupted(context.Background(), now.Add(time.Second)); err != nil || count != 1 {
		t.Fatalf("reconcile = (%d, %v), want (1, nil)", count, err)
	}
	if err := fixture.service.CheckDataRootRelocation(context.Background()); err != nil {
		t.Fatalf("reconciled operation still blocked relocation: %v", err)
	}
}
