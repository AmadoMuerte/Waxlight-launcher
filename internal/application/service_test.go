package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/auth"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/database"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/filesystem"
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
	versions []domain.AvailableGameVersion
}

func (catalog staticVersionCatalog) List(
	_ context.Context,
) ([]domain.AvailableGameVersion, error) {
	return append([]domain.AvailableGameVersion(nil), catalog.versions...), nil
}

type recordingDownloader struct {
	waitForCancellation bool
}

func (downloader recordingDownloader) Download(
	ctx context.Context,
	request application.DownloadRequest,
	progress chan<- application.DownloadProgress,
) error {
	if downloader.waitForCancellation {
		if err := os.MkdirAll(filepath.Dir(request.DestinationPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(
			request.DestinationPath+".partial",
			[]byte("unfinished package"),
			0o644,
		); err != nil {
			return err
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
	progress <- application.DownloadProgress{
		DownloadedBytes: 7,
		TotalBytes:      7,
		BytesPerSecond:  7,
	}
	return nil
}

type fakeGamePackageInstaller struct{}

type fixedDiskSpace int64

func (space fixedDiskSpace) Available(string) (int64, error) {
	return int64(space), nil
}

func (fakeGamePackageInstaller) Install(
	_ context.Context,
	_ string,
	targetPath string,
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
	store      *database.SQLiteStore
	root       string
	executable string
	launcher   *recordingLauncher
}

func newTestFixture(t *testing.T) testFixture {
	t.Helper()

	root := t.TempDir()
	store, err := database.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}

	launcher := &recordingLauncher{}
	service := application.NewService(
		store,
		filesystem.ArchiveInstaller{},
		filesystem.ModFileManager{},
		launcher,
		root,
	)
	t.Cleanup(func() {
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
	version := domain.GameVersion{
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
	}
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

	if err := fixture.service.DeleteVersion(ctx, "1.20", true); err != nil {
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

	if err := fixture.service.DeleteMod(ctx, disabledMod.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(disabledMod.FilePath); !os.IsNotExist(err) {
		t.Fatal("the deleted mod file still exists")
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

func TestAuthenticatedLaunchValidatesAndPatchesClientSettings(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	authClient := &fakeAuthClient{
		session: auth.Session{
			SessionKey:       "session-key",
			SessionSignature: "session-signature",
			UID:              "player-uid",
			PlayerName:       "Waxlighter",
		},
		validateResult: true,
	}
	accountService := application.NewAccountService(
		fixture.store,
		authClient,
		newMemorySecretStore(),
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
	if _, err := (filesystem.ClientSettingsService{}).Inject(settingsPath, domain.Account{
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
	authClient := &fakeAuthClient{session: auth.Session{SessionKey: "WAXLIGHT_TEST_SESSION_KEY_DO_NOT_LEAK", SessionSignature: "signature", UID: "uid", PlayerName: "player"}, validateResult: true}
	accountService := application.NewAccountService(fixture.store, authClient, newMemorySecretStore())
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
}

func TestExpiredSessionBlocksLaunch(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	authClient := &fakeAuthClient{
		session: auth.Session{
			SessionKey:       "session-key",
			SessionSignature: "session-signature",
			UID:              "player-uid",
			PlayerName:       "Waxlighter",
		},
		validateResult: false,
	}
	accountService := application.NewAccountService(
		fixture.store,
		authClient,
		newMemorySecretStore(),
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
	if err != nil || account.Status != domain.AccountStatusExpired {
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

	_, err := fixture.service.InstallVersion(
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

func TestAvailableVersionDownloadIsInstalledAndTracked(t *testing.T) {
	fixture := newTestFixture(t)
	release := domain.AvailableGameVersion{
		ID:                "1.22.6",
		Name:              "1.22.6",
		Channel:           "stable",
		Platform:          "linux",
		Architecture:      "amd64",
		Filename:          "vs_client_linux-x64_1.22.6.tar.gz",
		DownloadURL:       "https://cdn.vintagestory.at/gamefiles/stable/vs_client_linux-x64_1.22.6.tar.gz",
		DownloadSize:      7,
		Checksum:          "0123456789abcdef0123456789abcdef",
		ChecksumAlgorithm: "md5",
	}
	fixture.service.ConfigureVersionDownloads(
		staticVersionCatalog{versions: []domain.AvailableGameVersion{release}},
		recordingDownloader{},
		fakeGamePackageInstaller{},
	)

	operation, err := fixture.service.InstallAvailableVersion(
		context.Background(),
		release.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	waitForOperationStatus(t, fixture.store, operation.ID, "completed")

	installed, err := fixture.store.GetVersion(context.Background(), release.ID)
	if err != nil {
		t.Fatal(err)
	}
	if installed.Channel != "stable" || installed.Status != "installed" {
		t.Fatalf("unexpected installed version: %+v", installed)
	}
	if _, err := os.Stat(installed.ExecutablePath); err != nil {
		t.Fatalf("expected installed executable: %v", err)
	}
}

func TestAvailableVersionDownloadCanBeCancelled(t *testing.T) {
	fixture := newTestFixture(t)
	release := domain.AvailableGameVersion{
		ID:                "1.22.0-rc.1",
		Name:              "1.22.0-rc.1",
		Channel:           "unstable",
		Platform:          "linux",
		Architecture:      "amd64",
		Filename:          "preview.tar.gz",
		DownloadURL:       "https://cdn.vintagestory.at/gamefiles/unstable/preview.tar.gz",
		Checksum:          "0123456789abcdef0123456789abcdef",
		ChecksumAlgorithm: "md5",
	}
	fixture.service.ConfigureVersionDownloads(
		staticVersionCatalog{versions: []domain.AvailableGameVersion{release}},
		recordingDownloader{waitForCancellation: true},
		fakeGamePackageInstaller{},
	)

	operation, err := fixture.service.InstallAvailableVersion(
		context.Background(),
		release.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, duplicateErr := fixture.service.InstallAvailableVersion(
		context.Background(),
		release.ID,
	); !isErrorCode(duplicateErr, domain.ErrVersionExists) {
		t.Fatalf("expected duplicate active install to be rejected, got %v", duplicateErr)
	}
	if err := fixture.service.CancelOperation(operation.ID); err != nil {
		t.Fatal(err)
	}
	operations, err := fixture.store.ListOperations(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, stored := range operations {
		if stored.ID == operation.ID {
			t.Fatalf("cancelled operation was retained: %+v", stored)
		}
	}
	partialPath := filepath.Join(
		fixture.root,
		"downloads",
		release.Filename+".partial",
	)
	if _, err := os.Stat(partialPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial download was not removed: %v", err)
	}
	if _, err := fixture.service.InstallAvailableVersion(
		context.Background(),
		release.ID,
	); err != nil {
		t.Fatalf("version remained locked after cancellation: %v", err)
	}
}

func TestAvailableVersionChecksDiskSpaceBeforeStarting(t *testing.T) {
	fixture := newTestFixture(t)
	release := domain.AvailableGameVersion{
		ID:                "1.22.5",
		Name:              "1.22.5",
		Filename:          "game.tar.gz",
		DownloadURL:       "https://cdn.vintagestory.at/gamefiles/stable/game.tar.gz",
		DownloadSize:      1_000,
		Checksum:          "0123456789abcdef0123456789abcdef",
		ChecksumAlgorithm: "md5",
	}
	fixture.service.ConfigureVersionDownloads(
		staticVersionCatalog{versions: []domain.AvailableGameVersion{release}},
		recordingDownloader{},
		fakeGamePackageInstaller{},
	)
	fixture.service.ConfigureDiskSpaceChecker(fixedDiskSpace(1_999))

	_, err := fixture.service.InstallAvailableVersion(
		context.Background(),
		release.ID,
	)
	if !isErrorCode(err, domain.ErrInsufficientSpace) {
		t.Fatalf("expected insufficient disk space error, got %v", err)
	}
}

func isErrorCode(err error, code string) bool {
	var applicationError *domain.AppError
	return errors.As(err, &applicationError) && applicationError.Code == code
}

func waitForOperationStatus(
	t *testing.T,
	store *database.SQLiteStore,
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
