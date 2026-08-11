package bootstrap

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/app"
	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/events"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/credentials"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/dataroot"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/downloader"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/filesystem"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/gameversion"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/instancedirectory"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/instancepackage"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/logging"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/modcatalog"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/modstorage"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/nativefs"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/securefs"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/servercatalog"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/updater"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/versionfs"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/vintagestory"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/launching"
	"github.com/waxlight/waxlight-launcher/internal/mutations"
	"github.com/waxlight/waxlight-launcher/internal/operations"
	"github.com/waxlight/waxlight-launcher/internal/platform/process"
	"github.com/waxlight/waxlight-launcher/internal/platform/sqlite"
	"github.com/waxlight/waxlight-launcher/internal/presentation"
	"github.com/waxlight/waxlight-launcher/internal/publishers"
	"github.com/waxlight/waxlight-launcher/internal/sessions"
	settingscore "github.com/waxlight/waxlight-launcher/internal/settings"
	"github.com/waxlight/waxlight-launcher/internal/telemetry"
	wailstransport "github.com/waxlight/waxlight-launcher/internal/transport/wails"
	"github.com/waxlight/waxlight-launcher/internal/version"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

type Container struct {
	Service        *application.Service
	AccountService *accounts.Service
	Launching      *launching.Coordinator
	DataRoot       *dataroot.Manager
	Lifecycle      *app.Lifecycle
	Events         events.Publisher
	Controllers    []any
	telemetry      *telemetry.Service
}

func New() (*Container, error) {
	dataRootManager, err := dataroot.New()
	if err != nil {
		return nil, err
	}

	dataRoot, err := dataRootManager.PrepareStartup()
	if err != nil {
		return nil, fmt.Errorf("prepare data directory: %w", err)
	}
	if err := os.MkdirAll(dataRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	if err := securefs.Apply(dataRoot, 0o700, true); err != nil {
		return nil, fmt.Errorf("secure data directory: %w", err)
	}
	slog.Info("bootstrap: data directory ready", "root", dataRoot)
	// Mirror every log line to rolling session files in <dataRoot>/logs. The
	// directory moves together with the data root during relocation. Every
	// session file opens with the launcher version and system information.
	fileHeader := fmt.Sprintf(
		"Waxlight Launcher %s\nPlatform: %s/%s\nGo: %s\nStarted: %s",
		version.Version(),
		runtime.GOOS,
		runtime.GOARCH,
		runtime.Version(),
		time.Now().UTC().Format(time.RFC3339),
	)
	logging.SetFileHeader(fileHeader)
	logging.SetLogDirectory(filepath.Join(dataRoot, "logs"), logging.DefaultMaxLogFiles)

	if err := application.PurgeStaleUpdateSessions(dataRoot); err != nil {
		return nil, fmt.Errorf("purge stale launcher update sessions: %w", err)
	}

	store, err := sqlite.Open(dataroot.DatabasePath(dataRoot))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	slog.Info("bootstrap: database opened")
	if err := dataRootManager.FinalizePrevious(func(oldRoot, newRoot string) error {
		return store.RelocatePaths(context.Background(), oldRoot, newRoot)
	}); err != nil {
		closeStoreOnError(store)
		return nil, fmt.Errorf("finish data folder relocation: %w", err)
	}
	if err := applyInstallerTelemetryConsent(context.Background(), store, dataRootManager.Home()); err != nil {
		// Consent handling fails closed: an unreadable or malformed installer
		// marker must never enable telemetry or prevent the launcher from starting.
		slog.Warn("bootstrap: could not apply installer telemetry choice", "error", err)
	}

	sessionService := sessions.NewService(store, time.Now)
	if err := sessionService.RecoverOpen(context.Background()); err != nil {
		slog.Warn("bootstrap: could not recover interrupted game sessions", "error", err)
	}
	lifecycle := app.NewLifecycle()
	eventPublisher := wailstransport.NewEventAdapter(lifecycle)
	operationManager := operations.NewManager(store, lifecycle, eventPublisher)
	if _, err := operationManager.ReconcileInterrupted(context.Background(), time.Now().UTC()); err != nil {
		closeStoreOnError(store)
		return nil, fmt.Errorf("reconcile interrupted operations: %w", err)
	}
	settingsReader := settingscore.NewReader(store)
	launcherSettings, err := settingsReader.Get(context.Background())
	if err != nil {
		closeStoreOnError(store)
		return nil, fmt.Errorf("load settings: %w", err)
	}
	downloadManager := downloader.NewManager(
		downloader.NewHTTPDownloader(),
		launcherSettings.DownloadsParallel,
	)
	mutationGate := &mutations.Gate{}
	versionFilesystem := versionfs.New(dataRoot)
	versionCatalog := vintagestory.NewVersionCatalog(nil)
	archiveInstaller := filesystem.ArchiveInstaller{}
	versionRuntime := versions.NewInstallRuntime(versionFilesystem, mutationGate, operationManager, time.Now, newVersionID)
	versionQueries := versions.NewQueryService(store, versionCatalog, archiveInstaller, versionFilesystem, time.Now)
	versionService := versions.NewCapabilities(
		versionQueries,
		versions.NewLocalInstallService(store, archiveInstaller, versionRuntime, runtime.GOOS, runtime.GOARCH),
		versions.NewCatalogInstallService(store, versionQueries, downloadManager, gameversion.NewInstaller(), filesystem.DiskSpace{}, versionRuntime, eventPublisher, dataRoot),
		versions.NewRemovalService(store, store, versionFilesystem, mutationGate, eventPublisher),
	)
	instanceQueries := instances.NewQueryService(store)
	telemetryService := telemetry.NewService(
		telemetry.NewClient(telemetry.ProductionEndpoint()),
		settingsReader,
		store,
		store,
	)
	instanceCreator := instances.NewCreateService(
		store,
		versionService,
		store,
		func(ctx context.Context) (string, error) {
			settings, err := settingsReader.Get(ctx)
			return settings.Language, err
		},
		mutationGate,
		instancedirectory.New(filesystem.ModFileManager{}),
		eventPublisher,
		telemetryService.Event,
		dataRoot,
		time.Now,
		newVersionID,
	)
	instanceSlot := mutations.NewSlot()
	launchRegistry := launching.NewRegistry(instanceSlot)
	service := application.NewService(
		store,
		filesystem.ModFileManager{},
		dataRoot,
		operationManager,
		sessionService,
		instanceQueries,
		instanceCreator,
		instancedirectory.NewCloneStorage(filesystem.SanitizeClientSettings),
		versionService,
		downloadManager,
		filesystem.DiskSpace{},
		mutationGate,
		settingsReader,
		instanceSlot,
		launchRegistry,
	)
	clientSettingsService := filesystem.ClientSettingsService{}
	// The account service needs the launch coordinator's account-cleanup hook,
	// and the coordinator needs the account reader; wire the hook after the
	// coordinator exists.
	var clearAccountsFromInstances func(context.Context, string) error
	accountService := accounts.NewService(
		store,
		vintagestory.NewAuthClient(nil),
		credentials.NewStore(dataRoot),
		credentials.NewStore(dataRoot),
		func(ctx context.Context, accountID string) error {
			if clearAccountsFromInstances != nil {
				return clearAccountsFromInstances(ctx, accountID)
			}
			return nil
		},
		func(ctx context.Context) {
			telemetryService.Error(
				ctx,
				telemetry.ErrorAuthServerUnavailable,
				telemetry.ComponentAuthentication,
				telemetry.OperationAuthenticate,
			)
		},
		mutationGate,
	)
	launchCoordinator := launching.NewCoordinator(
		launchRegistry,
		mutationGate,
		store,
		versionService,
		accountService,
		clientSettingsService,
		settingsReader,
		filesystem.ModFileManager{},
		sessionService,
		process.OSLauncher{},
		instancedirectory.LaunchLogs{},
		eventPublisher,
		telemetryService,
		service,
		lifecycle,
		operationManager,
		time.Now,
		newVersionID,
	)
	clearAccountsFromInstances = launchCoordinator.ClearAccountFromInstances
	// Do not probe with a write here: a temporarily locked native store must not
	// prevent the launcher from opening. Credential operations report failures
	// when the user signs in or launches a game.
	secretStore := credentials.NewStore(dataRoot)
	storedAccounts, err := store.ListAccounts(context.Background())
	if err != nil {
		closeStoreOnError(store)
		return nil, fmt.Errorf("list accounts for credential migration: %w", err)
	}
	accountIDs := make([]string, 0, len(storedAccounts))
	for _, account := range storedAccounts {
		accountIDs = append(accountIDs, account.ID)
	}
	if err := secretStore.ReconcilePending(context.Background(), accountIDs); err != nil {
		if !credentialStoreUnavailable(err) {
			closeStoreOnError(store)
			return nil, fmt.Errorf("reconcile interrupted credential commit: %w", err)
		}
		slog.Warn("bootstrap: credential store unavailable; interrupted credential recovery will retry later", "error", err)
	}
	if err := credentials.NewMigrator(dataRoot, secretStore).Run(context.Background(), accountIDs); err != nil {
		if !credentialStoreUnavailable(err) {
			closeStoreOnError(store)
			return nil, err
		}
		slog.Warn("bootstrap: credential store unavailable; legacy credential migration will retry later", "error", err)
	}
	slog.Info("bootstrap: credential recovery checks finished")
	settingsService := settingscore.NewService(store, settingsReader, telemetryService, telemetryService, downloadManager)
	dataRootService := settingscore.NewDataRootService(
		dataRootManager,
		mutationGate,
		launchCoordinator,
		lifecycle,
		eventPublisher,
		wailstransport.QuitAdapter{},
	)
	service.ConfigureTelemetry(telemetryService)
	service.ConfigureClientSettings(clientSettingsService)
	if err := launchCoordinator.ReconcileInjectedCredentials(context.Background()); err != nil {
		closeStoreOnError(store)
		return nil, err
	}
	service.ConfigureMods(modcatalog.NewClient(nil), modstorage.New(dataRoot))
	service.ConfigurePublicServerCatalog(servercatalog.NewClient(nil))
	packageService := instances.NewPackageService(
		store,
		instanceCreator,
		versionService,
		store,
		packageCatalogAdapter{service: service},
		packageDownloadedAdapter{service: service},
		service,
		service,
		instancepackage.Store{},
		mutationGate,
		eventPublisher,
		func(path string) error { return application.SafeRemoveAll(path, dataRoot, ".waxlight-instance") },
		dataRoot,
		time.Now,
		newVersionID,
	)
	updateHTTPClient := updater.NewHTTPClient()
	updateDownloader := downloader.NewManager(
		&downloader.HTTPDownloader{Client: updateHTTPClient},
		1,
	)
	updateService := application.NewLauncherUpdateService(
		updater.NewSource(updateHTTPClient),
		updateDownloader,
		updater.NewInstaller(),
		updater.NewSignatureVerifier(publishers.GetTrustedWindowsPublishers()),
		mutationGate,
		dataRoot,
		version.Version(),
	)
	updateService.ConfigureTelemetry(telemetryService)
	controllers := []any{
		presentation.NewAppController(),
		presentation.NewAccountController(accountService, lifecycle),
		presentation.NewGameVersionController(versionService, lifecycle),
		presentation.NewInstanceController(
			service,
			instanceCreator,
			instanceQueries,
			service.InstanceUpdater(),
			service.InstanceDeleter(),
			service.InstanceCloner(),
			sessionService,
			lifecycle,
		),
		presentation.NewServerController(service, lifecycle),
		presentation.NewModManagerController(service, lifecycle),
		presentation.NewModCatalogController(service, lifecycle),
		presentation.NewInstancePackageController(packageService, lifecycle),
		presentation.NewLaunchController(launchCoordinator, lifecycle),
		presentation.NewStatisticsController(sessionService, lifecycle),
		presentation.NewOperationController(operationManager, lifecycle),
		presentation.NewSnapshotController(service, lifecycle),
		presentation.NewLastKnownGoodController(service, lifecycle),
		presentation.NewLogController(service, versionService, lifecycle),
		presentation.NewSettingsController(
			settingsReader,
			settingsService,
			dataRootService,
			lifecycle,
			wailstransport.NewDialogAdapter(lifecycle),
			nativefs.Opener{},
		),
		presentation.NewLauncherUpdateController(updateService, lifecycle, eventPublisher),
	}

	return &Container{
		Service:        service,
		AccountService: accountService,
		Launching:      launchCoordinator,
		DataRoot:       dataRootManager,
		Lifecycle:      lifecycle,
		Events:         eventPublisher,
		Controllers:    controllers,
		telemetry:      telemetryService,
	}, nil
}

func newVersionID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(value)
}

func credentialStoreUnavailable(err error) bool {
	return errors.Is(err, accounts.ErrStoreLocked) ||
		errors.Is(err, accounts.ErrStoreUnavailable) ||
		errors.Is(err, accounts.ErrPermissionDenied)
}

// packageCatalogAdapter maps the application catalog method names to the
// instance package port until the mods feature owns them.
type packageCatalogAdapter struct {
	service *application.Service
}

func (adapter packageCatalogAdapter) Get(ctx context.Context, modID string) (domain.ModDetails, error) {
	return adapter.service.GetCatalogMod(ctx, modID)
}

// packageDownloadedAdapter maps the application download-store method names to
// the instance package port until the mods feature owns them.
type packageDownloadedAdapter struct {
	service *application.Service
}

func (adapter packageDownloadedAdapter) Get(ctx context.Context, modID, versionID string) (domain.DownloadedMod, error) {
	return adapter.service.GetDownloadedMod(ctx, modID, versionID)
}

func (container *Container) Startup(ctx context.Context) {
	container.Lifecycle.Startup(ctx)
	container.Service.SetEventPublisher(container.Events)
	// Push every new log line to the UI console as it is produced. The logging
	// package stays framework-free; the Wails binding lives here.
	logging.SetEmitter(func(entry logging.Entry) {
		container.Events.Publish("logs:append", entry.Line())
	})
	// Native keyring calls cannot be interrupted on every platform. Keep this
	// startup hygiene task cancelable, but do not let a blocked keyring prevent
	// the application from shutting down.
	go container.AccountService.ValidateStaleAccounts(container.Lifecycle.Context(), 24*time.Hour)
	container.telemetryHeartbeat()
}

func (container *Container) telemetryHeartbeat() {
	// The heartbeat is fully asynchronous and isolated: telemetry failures
	// never affect launcher startup or any other functionality.
	if container.telemetry != nil {
		container.telemetry.MaybeSendHeartbeat()
	}
}

func (container *Container) Shutdown(context.Context) {
	logging.SetEmitter(nil)
	container.Lifecycle.Shutdown()
	// A game still running past the startup window when the launcher shuts
	// down is evidence its configuration works; record it before the database
	// closes. waitForGame never observes this exit because the launcher is
	// already gone.
	container.Launching.RecordEstablishedOnShutdown()
	if err := container.Service.Close(); err != nil {
		slog.Warn("bootstrap: could not close the application service cleanly", "error", err)
	}
}

// closeStoreOnError best-effort closes the database after a bootstrap failure.
// The original error is already returned to the caller; a close failure is
// logged so a flushed-but-failed shutdown is not silently lost.
func closeStoreOnError(store *sqlite.SQLiteStore) {
	if err := store.Close(); err != nil {
		slog.Warn("bootstrap: could not close the database after a failure", "error", err)
	}
}
