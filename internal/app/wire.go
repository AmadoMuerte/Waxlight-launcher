package app

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

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/events"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/launching"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/mutations"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/news"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/operations"
	optimumfeature "github.com/AmadoMuerte/Waxlight-launcher/internal/optimum"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/credentials"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/dataroot"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/discord"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/dotnet"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/downloader"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/filesystem"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/gameversion"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/instancedirectory"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/instancepackage"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/logging"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/modcatalog"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/modstorage"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/nativefs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/newscache"
	platformoptimum "github.com/AmadoMuerte/Waxlight-launcher/internal/platform/optimum"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/process"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/securefs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/servercatalog"
	platformsnapshots "github.com/AmadoMuerte/Waxlight-launcher/internal/platform/snapshots"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/sqlite"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/updater"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/versionfs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/vintagestory"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/presence"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/publishers"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/recovery"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/servers"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/sessions"
	settingscore "github.com/AmadoMuerte/Waxlight-launcher/internal/settings"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/snapshots"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/statistics"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/supportreports"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/telemetry"
	wailstransport "github.com/AmadoMuerte/Waxlight-launcher/internal/transport/wails"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/updates"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/version"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/versions"
)

// Container is the explicit composition root of the launcher. Every
// dependency is constructed in New/NewWithHome and wired exactly once; the
// Wails entrypoint only starts the container and passes its parts to Wails.
type Container struct {
	AccountService *accounts.Service
	Launching      *launching.Coordinator
	DataRoot       *dataroot.Manager
	Lifecycle      *Lifecycle
	Events         events.Publisher
	Controllers    []any
	CoverHandler   *wailstransport.InstanceCoverHandler
	DeepLinks      *DeepLinks
	modsCatalog    *mods.CatalogService
	store          *sqlite.SQLiteStore
	telemetry      *telemetry.Service
	presence       *presence.Service
}

// New constructs the container at the OS configuration directory.
func New() (*Container, error) {
	return NewWithHome("")
}

// NewWithHome constructs the container at an explicit home directory. An
// empty home falls back to the OS configuration directory. Tests use this
// entrypoint to isolate the launcher state.
func NewWithHome(home string) (*Container, error) {
	dataRootManager, err := newDataRootManager(home)
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
	slog.Info("wire: data directory ready", "root", dataRoot)
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

	if err := updates.PurgeStaleUpdateSessions(dataRoot); err != nil {
		return nil, fmt.Errorf("purge stale launcher update sessions: %w", err)
	}

	store, err := sqlite.Open(dataroot.DatabasePath(dataRoot))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	slog.Info("wire: database opened")
	if err := dataRootManager.FinalizePrevious(func(oldRoot, newRoot string) error {
		if err := store.RelocatePaths(context.Background(), oldRoot, newRoot); err != nil {
			return err
		}
		// The downloaded-mod cache stores file paths too; heal stale absolute
		// paths from the previous root. Cache healing is best-effort: a failure
		// here only means a re-download later, never broken instance data.
		if err := modstorage.New(newRoot).RelocateOldRoot(context.Background(), oldRoot); err != nil {
			slog.Warn("wire: could not rewrite cached mod paths after the data folder move", "error", err)
		}
		return nil
	}); err != nil {
		closeStoreOnError(store)
		return nil, fmt.Errorf("finish data folder relocation: %w", err)
	}
	if err := applyInstallerTelemetryConsent(context.Background(), store, dataRootManager.Home()); err != nil {
		// Consent handling fails closed: an unreadable or malformed installer
		// marker must never enable telemetry or prevent the launcher from starting.
		slog.Warn("wire: could not apply installer telemetry choice", "error", err)
	}

	sessionService := sessions.NewService(store, time.Now)
	if err := sessionService.RecoverOpen(context.Background()); err != nil {
		slog.Warn("wire: could not recover interrupted game sessions", "error", err)
	}
	statisticsService := statistics.NewService(sessionService)
	lifecycle := NewLifecycle()
	eventPublisher := wailstransport.NewEventAdapter(lifecycle)
	deepLinks := NewDeepLinks(eventPublisher)
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
	telemetryClient := telemetry.NewClient(telemetry.ProductionEndpoint())
	telemetryService := telemetry.NewService(
		telemetryClient,
		settingsReader,
		store,
		store,
		lifecycle,
	)
	presenceService := presence.NewService(settingsReader, func(appID string) presence.Client {
		client := discord.Dial(appID)
		if client == nil {
			return nil
		}
		return discordPresenceClient{client: client}
	})
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
	clientSettingsService := filesystem.ClientSettingsService{}
	clearClientSettings := func(path string) error { return clientSettingsService.Clear(path) }
	safeRemoveInstanceDir := func(path string) error {
		return instances.SafeRemoveAll(path, dataRoot, ".waxlight-instance")
	}
	stageRemoveInstanceDir := func(path string) (func() error, func() error, error) {
		return instances.StageDirectoryRemoval(path, dataRoot, ".waxlight-instance")
	}
	modsRepository := modsStoreAdapter{store: store}
	modCatalog := modcatalog.NewClient(nil)
	modDownloads := modstorage.New(dataRoot)
	emit := func(name string, payload any) { eventPublisher.Publish(name, payload) }
	reportEvent := func(ctx context.Context, name string) { telemetryService.Event(ctx, name) }

	var recoveryService *recovery.Service
	var modsService *mods.Service
	var modsCatalogService *mods.CatalogService
	snapshotService := snapshots.NewService(
		platformsnapshots.New(dataRoot),
		snapshotInstanceAdapter{store: store},
		versionService,
		snapshotModStoreAdapter{store: store, mods: func() *mods.Service { return modsService }},
		snapshotCatalogAdapter{catalog: func() *mods.CatalogService { return modsCatalogService }},
		snapshotArchiveInfoAdapter{},
		settingsReader,
		operationManager,
		mutationGate,
		instanceSlot,
		launchRegistry,
		filesystem.DiskSpace{},
		dataroot.TotalSizeContext,
		filesystem.SanitizeClientSettings,
		instancedirectory.HardenLogs,
		clearClientSettings,
		safeRemoveInstanceDir,
		snapshotLKGReferenceAdapter{recovery: func() *recovery.Service { return recoveryService }},
		dataRoot,
		time.Now,
		newVersionID,
	)
	recoveryService = recovery.NewService(
		store,
		snapshotService,
		snapshotService,
		store,
		mutationGate,
		eventPublisher,
		time.Now,
	)
	snapshotter := snapshots.SafetySnapshotterFunc(func(ctx context.Context, instanceID string, reason snapshots.Reason, snapshotContext map[string]string) error {
		_, err := snapshotService.CreateSafety(ctx, instanceID, reason, snapshotContext)
		return err
	})
	modsService = mods.NewService(
		modsRepository,
		filesystem.ModFileManager{},
		modCatalog,
		modDownloads,
		operationManager,
		mutationGate,
		launchRegistry,
		snapshotter,
		mods.PublishFunc(emit),
		telemetryService,
		time.Now,
		newVersionID,
	)
	modsCatalogService = mods.NewCatalogService(
		modsRepository,
		filesystem.ModFileManager{},
		modCatalog,
		modDownloads,
		downloadManager,
		versionService,
		modsService,
		mutationGate,
		launchRegistry,
		snapshotter,
		mods.PublishFunc(emit),
		telemetryService,
		mods.NewModTaskManager(mods.PublishFunc(emit)),
		time.Now,
		newVersionID,
	)
	instanceUpdater := instances.NewUpdateService(
		store,
		versionService,
		mutationGate,
		launchRegistry,
		snapshotter,
		clearClientSettings,
		instances.PublishFunc(emit),
		time.Now,
	)
	instanceDeleter := instances.NewDeleteService(
		store,
		mutationGate,
		launchRegistry,
		stageRemoveInstanceDir,
		clearClientSettings,
		store.DeleteLastKnownGood,
		instances.PublishFunc(emit),
		reportEvent,
	)
	instanceCloner := instances.NewCloneService(
		store,
		store,
		instanceCreator,
		mutationGate,
		launchRegistry,
		instancedirectory.NewCloneStorage(filesystem.SanitizeClientSettings),
		safeRemoveInstanceDir,
		time.Now,
		newVersionID,
	)
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
	optimumService := optimumfeature.NewService(platformoptimum.NewLocator())
	launchCoordinator := launching.NewCoordinator(
		launchRegistry,
		mutationGate,
		store,
		versionService,
		accountService,
		clientSettingsService,
		settingsReader,
		filesystem.ModFileManager{},
		optimumLaunchAdapter{service: optimumService},
		enabledModAdapter{files: filesystem.ModFileManager{}},
		sessionService,
		process.OSLauncher{},
		dotnet.NewDetector(runtime.GOOS),
		instancedirectory.LaunchLogs{},
		eventPublisher,
		telemetryService,
		recoveryService,
		lifecycle,
		operationManager,
		presenceService,
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
		slog.Warn("wire: credential store unavailable; interrupted credential recovery will retry later", "error", err)
	}
	if err := credentials.NewMigrator(dataRoot, secretStore).Run(context.Background(), accountIDs); err != nil {
		if !credentialStoreUnavailable(err) {
			closeStoreOnError(store)
			return nil, err
		}
		slog.Warn("wire: credential store unavailable; legacy credential migration will retry later", "error", err)
	}
	slog.Info("wire: credential recovery checks finished")
	settingsService := settingscore.NewService(store, settingsReader, telemetryService, telemetryService, downloadManager)
	dataRootService := settingscore.NewDataRootService(
		dataRootManager,
		mutationGate,
		launchCoordinator,
		lifecycle,
		eventPublisher,
		wailstransport.QuitAdapter{},
	)
	if err := launchCoordinator.ReconcileInjectedCredentials(context.Background()); err != nil {
		closeStoreOnError(store)
		return nil, err
	}
	serverService := servers.NewService(store, store, mutationGate, eventPublisher, time.Now, newVersionID)
	serverCatalogService := servers.NewCatalogService(servercatalog.NewClient(nil))
	newsService := news.NewService(
		vintagestory.NewNewsSource(nil, fmt.Sprintf("Waxlight/%s (+https://github.com/AmadoMuerte/Waxlight-launcher)", version.Version())),
		newscache.New(dataRoot),
		store,
		time.Hour,
		time.Now,
	)
	packageService := instances.NewPackageService(
		store,
		instanceCreator,
		versionService,
		store,
		packageCatalogAdapter{service: modsCatalogService},
		packageDownloadedAdapter{service: modsCatalogService},
		packageCatalogModInstaller{installed: modsService, catalog: modsCatalogService},
		mods.Identity{},
		instancepackage.Store{},
		mutationGate,
		operationManager,
		eventPublisher,
		func(path string) error { return instances.SafeRemoveAll(path, dataRoot, ".waxlight-instance") },
		dataRoot,
		time.Now,
		newVersionID,
	)
	migrationService := instances.NewMigrationService(
		instancedirectory.NewMigrationStorage(filesystem.SanitizeClientSettings),
		filesystem.ImportDiskSpace{},
		instanceCreator,
		operationManager,
		func(ctx context.Context, instanceID string) []string {
			if _, err := modsService.ListMods(ctx, instanceID); err != nil {
				return []string{"Imported mods remain local because they could not be scanned"}
			}
			result, err := modsCatalogService.LinkLocalMods(ctx, instanceID)
			if err != nil {
				return []string{"Imported mods remain local because catalog linking failed"}
			}
			warnings := make([]string, 0, len(result.Failed)+len(result.NotMatched))
			for range result.Failed {
				warnings = append(warnings, "An imported mod could not be linked to the catalog")
			}
			for range result.NotMatched {
				warnings = append(warnings, "An imported mod was not found in the catalog and remains local")
			}
			return warnings
		},
		dataRoot,
		time.Now,
		newVersionID,
	)
	updateHTTPClient := updater.NewHTTPClient()
	updateDownloader := downloader.NewManager(
		&downloader.HTTPDownloader{Client: updateHTTPClient},
		1,
	)
	updateService := updates.NewService(
		updater.NewSource(updateHTTPClient),
		updateDownloader,
		updater.NewInstaller(),
		updater.NewSignatureVerifier(publishers.GetTrustedWindowsPublishers()),
		mutationGate,
		dataRoot,
		version.Version(),
		telemetryService,
	)
	dialogs := wailstransport.NewDialogAdapter(lifecycle)
	supportReportService := supportreports.NewService(
		store,
		operationManager,
		sessionService,
		supportRecoveryAdapter{recovery: recoveryService, snapshots: snapshotService},
		supportLogAdapter{},
		telemetryService,
		supportSenderAdapter{client: telemetryClient},
	)
	instanceController := wailstransport.NewInstanceController(
		instanceCreator,
		instanceQueries,
		instanceUpdater,
		instanceDeleter,
		instanceCloner,
		statisticsService,
		modsService,
		migrationService,
		lifecycle,
		dialogs,
	)
	controllers := []any{
		wailstransport.NewAppController(),
		wailstransport.NewDeepLinkController(deepLinks),
		wailstransport.NewAccountController(accountService, lifecycle),
		wailstransport.NewGameVersionController(versionService, lifecycle),
		instanceController,
		wailstransport.NewServerController(serverService, serverCatalogService, lifecycle),
		wailstransport.NewNewsController(newsService, lifecycle),
		wailstransport.NewModManagerController(modsService, modsCatalogService, lifecycle),
		wailstransport.NewModCatalogController(modsCatalogService, lifecycle),
		wailstransport.NewInstancePackageController(packageService, lifecycle),
		wailstransport.NewLaunchController(launchCoordinator, lifecycle),
		wailstransport.NewStatisticsController(statisticsService, lifecycle),
		wailstransport.NewOperationController(operationManager, lifecycle),
		wailstransport.NewSnapshotController(snapshotService, lifecycle),
		wailstransport.NewLastKnownGoodController(recoveryService, lifecycle),
		wailstransport.NewLogController(instanceQueries, modsService, versionService, lifecycle),
		wailstransport.NewSupportReportController(supportReportService, lifecycle),
		wailstransport.NewSettingsController(
			settingsReader,
			settingsService,
			dataRootService,
			optimumService,
			lifecycle,
			dialogs,
			nativefs.Opener{},
		),
		wailstransport.NewLauncherUpdateController(updateService, lifecycle, eventPublisher),
		wailstransport.NewPresenceController(presenceService, lifecycle),
	}

	return &Container{
		AccountService: accountService,
		Launching:      launchCoordinator,
		DataRoot:       dataRootManager,
		Lifecycle:      lifecycle,
		Events:         eventPublisher,
		Controllers:    controllers,
		CoverHandler:   wailstransport.NewInstanceCoverHandler(instanceQueries),
		DeepLinks:      deepLinks,
		modsCatalog:    modsCatalogService,
		store:          store,
		telemetry:      telemetryService,
		presence:       presenceService,
	}, nil
}

func newDataRootManager(home string) (*dataroot.Manager, error) {
	if home != "" {
		return dataroot.NewWithHome(home), nil
	}
	return dataroot.New()
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

// packageCatalogAdapter maps the catalog service method names to the instance
// package port.
type packageCatalogAdapter struct {
	service *mods.CatalogService
}

func (adapter packageCatalogAdapter) Get(ctx context.Context, modID string) (mods.ModDetails, error) {
	return adapter.service.GetCatalogMod(ctx, modID)
}

// packageDownloadedAdapter maps the catalog download-store method names to the
// instance package port.
type packageDownloadedAdapter struct {
	service *mods.CatalogService
}

func (adapter packageDownloadedAdapter) Get(ctx context.Context, modID, versionID string) (mods.DownloadedMod, error) {
	return adapter.service.GetDownloadedMod(ctx, modID, versionID)
}

// packageCatalogModInstaller combines the installed-mod and catalog services
// for the instance package port.
type packageCatalogModInstaller struct {
	installed *mods.Service
	catalog   *mods.CatalogService
}

func (adapter packageCatalogModInstaller) DownloadCatalogMod(ctx context.Context, request mods.DownloadModRequest) (mods.ModInstallResult, error) {
	return adapter.catalog.DownloadCatalogMod(ctx, request)
}

func (adapter packageCatalogModInstaller) RemoveDownloadedModsIfUnused(ctx context.Context, downloaded []mods.DownloadedMod) error {
	return adapter.catalog.RemoveDownloadedModsIfUnusedLocked(ctx, downloaded)
}

func (adapter packageCatalogModInstaller) SetModEnabled(ctx context.Context, id string, enabled bool) (mods.InstalledMod, error) {
	return adapter.installed.SetModEnabled(ctx, id, enabled)
}

// Startup runs the lifecycle-owned startup steps Wails invokes after the
// application context is available.
func (container *Container) Startup(ctx context.Context) {
	container.Lifecycle.Startup(ctx)
	// Push every new log line to the UI console as it is produced. The logging
	// package stays framework-free; the Wails binding lives here.
	logging.SetEmitter(func(entry logging.Entry) {
		container.Events.Publish("logs:append", entry.Line())
	})
	// Native keyring calls cannot be interrupted on every platform. Keep this
	// startup hygiene task cancelable, but do not let a blocked keyring prevent
	// the application from shutting down.
	go container.AccountService.ValidateStaleAccounts(container.Lifecycle.Context(), 24*time.Hour)
	// Older cache entries may lack catalog tags; persist them once here so the
	// read paths stay read-only.
	container.Lifecycle.Go(func(ctx context.Context) { container.modsCatalog.BackfillDownloadedModTags(ctx) })
	container.presence.Connect(container.Lifecycle.Context())
	container.telemetryHeartbeat()
}

func (container *Container) telemetryHeartbeat() {
	// The heartbeat is fully asynchronous and isolated: telemetry failures
	// never affect launcher startup or any other functionality.
	if container.telemetry != nil {
		container.telemetry.MaybeSendHeartbeat()
	}
}

// Shutdown stops the lifecycle workers, records the established launches, and
// closes the shared store deterministically.
func (container *Container) Shutdown(context.Context) {
	logging.SetEmitter(nil)
	container.presence.Close()
	container.Lifecycle.Shutdown()
	// A game still running past the startup window when the launcher shuts
	// down is evidence its configuration works; record it before the database
	// closes. waitForGame never observes this exit because the launcher is
	// already gone.
	container.Launching.RecordEstablishedOnShutdown()
	if err := container.store.Close(); err != nil {
		slog.Warn("wire: could not close the database cleanly", "error", err)
	}
}

// closeStoreOnError best-effort closes the database after a construction
// failure. The original error is already returned to the caller; a close
// failure is logged so a flushed-but-failed shutdown is not silently lost.
func closeStoreOnError(store *sqlite.SQLiteStore) {
	if err := store.Close(); err != nil {
		slog.Warn("wire: could not close the database after a failure", "error", err)
	}
}
