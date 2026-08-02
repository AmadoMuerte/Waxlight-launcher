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
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/filesystem"
	processinfra "github.com/waxlight/waxlight-launcher/internal/infrastructure/process"
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
	if err := os.MkdirAll(dataRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
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
	accountService := application.NewAccountService(
		store,
		auth.NewClient(nil),
		credentials.NewFileStore(credentials.DefaultPath(dataRoot)),
	)
	service.ConfigureAuthentication(
		accountService,
		filesystem.ClientSettingsService{},
	)
	base := presentation.NewBase(service)
	controllers := []any{
		presentation.NewAppController(base),
		presentation.NewAccountController(accountService),
		presentation.NewGameVersionController(service),
		presentation.NewInstanceController(service),
		presentation.NewModManagerController(service),
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
