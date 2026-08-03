package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/auth"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/credentials"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/database"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/downloader"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/filesystem"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/gameversion"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/modcatalog"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/modstorage"
	processinfra "github.com/waxlight/waxlight-launcher/internal/infrastructure/process"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/securefs"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/vintagestory"
	"github.com/waxlight/waxlight-launcher/internal/presentation"
)

type Container struct {
	Service        *application.Service
	AccountService *application.AccountService
	Base           *presentation.Base
	Controllers    []any
}

func New() (*Container, error) {
	configDirectory, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	dataRoot := filepath.Join(configDirectory, "waxlight")
	if err := os.MkdirAll(dataRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	if err := securefs.Apply(dataRoot, 0o700, true); err != nil {
		return nil, fmt.Errorf("secure data directory: %w", err)
	}

	store, err := database.Open(filepath.Join(dataRoot, "waxlight.db"))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
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
		presentation.NewSettingsController(service, base),
	}

	return &Container{
		Service:        service,
		AccountService: accountService,
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
