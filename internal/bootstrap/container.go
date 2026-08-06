package bootstrap

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/auth"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/credentials"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/database"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/dataroot"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/downloader"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/filesystem"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/gameversion"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/modcatalog"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/modstorage"
	processinfra "github.com/waxlight/waxlight-launcher/internal/infrastructure/process"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/securefs"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/updater"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/vintagestory"
	"github.com/waxlight/waxlight-launcher/internal/presentation"
	"github.com/waxlight/waxlight-launcher/internal/publishers"
	"github.com/waxlight/waxlight-launcher/internal/version"
)

type Container struct {
	Service        *application.Service
	AccountService *application.AccountService
	DataRoot       *dataroot.Manager
	Base           *presentation.Base
	Controllers    []any
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

	if err := application.PurgeStaleUpdateSessions(dataRoot); err != nil {
		return nil, fmt.Errorf("purge stale launcher update sessions: %w", err)
	}

	store, err := database.Open(dataroot.DatabasePath(dataRoot))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := dataRootManager.FinalizePrevious(func(oldRoot, newRoot string) error {
		return store.RelocatePaths(context.Background(), oldRoot, newRoot)
	}); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("finish data folder relocation: %w", err)
	}

	_ = store.RecoverOpenSessions(context.Background(), time.Now().UTC())
	service := application.NewService(
		store,
		filesystem.ArchiveInstaller{},
		filesystem.ModFileManager{},
		processinfra.Launcher{},
		dataRoot,
	)
	secretStore := credentials.NewStore(dataRoot)
	if err := secretStore.Probe(context.Background()); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("native credential store is unavailable or locked; unlock it and retry: %w", err)
	}
	accounts, err := store.ListAccounts(context.Background())
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("list accounts for credential migration: %w", err)
	}
	accountIDs := make([]string, 0, len(accounts))
	for _, account := range accounts {
		accountIDs = append(accountIDs, account.ID)
	}
	if err := secretStore.ReconcilePending(context.Background(), accountIDs); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("reconcile interrupted credential commit: %w", err)
	}
	if err := credentials.NewMigrator(dataRoot, secretStore).Run(context.Background(), accountIDs); err != nil {
		_ = store.Close()
		return nil, err
	}
	accountService := application.NewAccountService(
		store,
		auth.NewClient(nil),
		secretStore,
	)
	service.ConfigureAuthentication(
		accountService,
		filesystem.ClientSettingsService{},
	)
	if err := service.ReconcileInjectedCredentials(context.Background()); err != nil {
		_ = store.Close()
		return nil, err
	}
	downloadManager := downloader.NewManager(downloader.NewHTTPDownloader(), 3)
	service.ConfigureVersionDownloads(
		vintagestory.NewVersionCatalog(nil),
		downloadManager,
		gameversion.NewInstaller(),
	)
	service.ConfigureMods(modcatalog.NewClient(nil), modstorage.New(dataRoot))
	service.ConfigureDiskSpaceChecker(filesystem.DiskSpace{})
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
		dataRoot,
		version.Version(),
	)
	base := presentation.NewBase(service)
	controllers := []any{
		presentation.NewAppController(base),
		presentation.NewAccountController(accountService),
		presentation.NewGameVersionController(service),
		presentation.NewInstanceController(service),
		presentation.NewModManagerController(service),
		presentation.NewModCatalogController(service),
		presentation.NewLaunchController(service),
		presentation.NewStatisticsController(service),
		presentation.NewOperationController(service),
		presentation.NewSettingsController(service, base, dataRootManager),
		presentation.NewLauncherUpdateController(updateService, base),
	}

	return &Container{
		Service:        service,
		AccountService: accountService,
		DataRoot:       dataRootManager,
		Base:           base,
		Controllers:    controllers,
	}, nil
}

func (container *Container) Startup(ctx context.Context) {
	container.Base.Startup(ctx)
	go container.AccountService.ValidateStaleAccounts(ctx, 24*time.Hour)
}

func (container *Container) Shutdown(context.Context) {
	_ = container.Service.Close()
}
