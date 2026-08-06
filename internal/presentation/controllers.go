package presentation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/dataroot"
	"github.com/waxlight/waxlight-launcher/internal/version"
)

type Base struct {
	svc *application.Service
	ctx context.Context
}

func NewBase(service *application.Service) *Base {
	return &Base{svc: service}
}

func (base *Base) Startup(ctx context.Context) {
	base.ctx = ctx
	base.svc.SetEventPublisher(&eventBus{ctx: ctx})
}

type eventBus struct {
	ctx context.Context
}

func (bus *eventBus) Publish(name string, payload any) {
	wruntime.EventsEmit(bus.ctx, name, payload)
}

type AppController struct {
	*Base
}

func NewAppController(base *Base) *AppController {
	return &AppController{Base: base}
}

func (controller *AppController) AppInfo() map[string]any {
	return map[string]any{
		"name":       "Waxlight Launcher",
		"shortName":  "Waxlight",
		"version":    version.Version(),
		"unofficial": true,
	}
}

type AccountController struct {
	svc *application.AccountService
}

func NewAccountController(service *application.AccountService) *AccountController {
	return &AccountController{svc: service}
}

func (controller *AccountController) ListAccounts() ([]AccountDTO, error) {
	accounts, err := controller.svc.ListAccounts(context.Background())
	result := make([]AccountDTO, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, accountDTO(account))
	}
	return result, err
}

func (controller *AccountController) Login(email, password string) (LoginResultDTO, error) {
	result, err := controller.svc.Login(context.Background(), email, password)
	return loginResultDTO(result), err
}

func (controller *AccountController) CompleteTOTP(flowID, code string) (LoginResultDTO, error) {
	result, err := controller.svc.CompleteTOTP(context.Background(), flowID, code)
	return loginResultDTO(result), err
}

func (controller *AccountController) CancelLogin(flowID string) error {
	return controller.svc.CancelLogin(flowID)
}

func (controller *AccountController) SetDefaultAccount(id string) error {
	return controller.svc.SelectAccount(context.Background(), id)
}

func (controller *AccountController) RemoveAccount(id string) error {
	return controller.svc.RemoveAccount(context.Background(), id)
}

func (controller *AccountController) ValidateAccount(id string) (AccountDTO, error) {
	account, err := controller.svc.ValidateAccount(context.Background(), id)
	return accountDTO(account), err
}

func (controller *AccountController) ReauthenticateAccount(
	accountID string,
	email string,
	password string,
) (LoginResultDTO, error) {
	result, err := controller.svc.ReauthenticateAccount(
		context.Background(),
		accountID,
		email,
		password,
	)
	return loginResultDTO(result), err
}

type GameVersionController struct {
	svc *application.Service
}

func NewGameVersionController(service *application.Service) *GameVersionController {
	return &GameVersionController{svc: service}
}

type InstallVersionRequest struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	SourcePath             string `json:"sourcePath"`
	ExecutableRelativePath string `json:"executableRelativePath"`
	ExpectedSHA256         string `json:"expectedSha256"`
}

func (controller *GameVersionController) ListInstalledVersions() (
	[]GameVersionDTO,
	error,
) {
	versions, err := controller.svc.ListVersions(context.Background())
	result := make([]GameVersionDTO, 0, len(versions))
	for _, version := range versions {
		result = append(result, versionDTO(version))
	}
	return result, err
}

func (controller *GameVersionController) ListAvailableVersions() (
	[]AvailableGameVersionDTO,
	error,
) {
	versions, err := controller.svc.ListAvailableVersions(context.Background())
	result := make([]AvailableGameVersionDTO, 0, len(versions))
	for _, version := range versions {
		result = append(result, availableVersionDTO(version))
	}
	return result, err
}

func (controller *GameVersionController) InstallVersion(
	versionID string,
) (OperationDTO, error) {
	operation, err := controller.svc.InstallAvailableVersion(
		context.Background(),
		versionID,
	)
	return operationDTO(operation), err
}

func (controller *GameVersionController) InstallLocalVersion(
	request InstallVersionRequest,
) (OperationDTO, error) {
	operation, err := controller.svc.InstallVersion(
		context.Background(),
		request.ID,
		request.Name,
		request.SourcePath,
		request.ExecutableRelativePath,
		request.ExpectedSHA256,
	)
	return operationDTO(operation), err
}

func (controller *GameVersionController) RemoveVersion(
	id string,
	deleteFiles bool,
) error {
	return controller.svc.DeleteVersion(context.Background(), id, deleteFiles)
}

type InstanceController struct {
	svc *application.Service
}

func NewInstanceController(service *application.Service) *InstanceController {
	return &InstanceController{svc: service}
}

type CreateInstanceRequest struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	GameVersionID    string   `json:"gameVersionId"`
	DefaultAccountID *string  `json:"defaultAccountId,omitempty"`
	Directory        string   `json:"directory"`
	LaunchArguments  []string `json:"launchArguments"`
}

type UpdateInstanceRequest struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	GameVersionID    string   `json:"gameVersionId"`
	DefaultAccountID *string  `json:"defaultAccountId,omitempty"`
	LaunchArguments  []string `json:"launchArguments"`
}

func (controller *InstanceController) ListInstances() ([]InstanceDTO, error) {
	instances, err := controller.svc.ListInstances(context.Background())
	result := make([]InstanceDTO, 0, len(instances))

	for _, instance := range instances {
		dto := instanceDTO(instance)
		mods, _ := controller.svc.ListMods(context.Background(), instance.ID)
		for _, mod := range mods {
			dto.TotalModCount++
			if mod.Enabled {
				dto.EnabledModCount++
			}
		}
		dto.PlaytimeSeconds, _ = controller.svc.GetInstancePlaytime(
			context.Background(),
			instance.ID,
		)
		result = append(result, dto)
	}

	return result, err
}

func (controller *InstanceController) GetInstance(id string) (InstanceDTO, error) {
	instance, err := controller.svc.GetInstance(context.Background(), id)
	return instanceDTO(instance), err
}

func (controller *InstanceController) CreateInstance(
	request CreateInstanceRequest,
) (InstanceDTO, error) {
	instance, err := controller.svc.CreateInstance(
		context.Background(),
		application.CreateInstanceInput{
			Name:             request.Name,
			Description:      request.Description,
			GameVersionID:    request.GameVersionID,
			DefaultAccountID: request.DefaultAccountID,
			Directory:        request.Directory,
			LaunchArguments:  request.LaunchArguments,
		},
	)
	return instanceDTO(instance), err
}

func (controller *InstanceController) UpdateInstance(
	request UpdateInstanceRequest,
) (InstanceDTO, error) {
	instance, err := controller.svc.GetInstance(context.Background(), request.ID)
	if err != nil {
		return InstanceDTO{}, err
	}

	instance.Name = request.Name
	instance.Description = request.Description
	instance.GameVersionID = request.GameVersionID
	instance.DefaultAccountID = request.DefaultAccountID
	instance.LaunchArguments = request.LaunchArguments

	updated, err := controller.svc.UpdateInstance(context.Background(), instance)
	return instanceDTO(updated), err
}

func (controller *InstanceController) DeleteInstance(
	id string,
	deleteFiles bool,
) error {
	return controller.svc.DeleteInstance(context.Background(), id, deleteFiles)
}

type ModManagerController struct {
	svc *application.Service
}

func NewModManagerController(service *application.Service) *ModManagerController {
	return &ModManagerController{svc: service}
}

type InstallModFileRequest struct {
	InstanceID string `json:"instanceId"`
	SourcePath string `json:"sourcePath"`
	Name       string `json:"name"`
	Version    string `json:"version"`
}

func (controller *ModManagerController) ListInstalledMods(
	instanceID string,
) ([]InstalledModDTO, error) {
	mods, err := controller.svc.ListMods(context.Background(), instanceID)
	result := make([]InstalledModDTO, 0, len(mods))
	for _, mod := range mods {
		result = append(result, modDTO(mod))
	}
	return result, err
}

func (controller *ModManagerController) CheckInstanceModUpdates(
	instanceID string,
) (InstanceModUpdateReportDTO, error) {
	report, err := controller.svc.CheckInstanceModUpdates(
		context.Background(),
		instanceID,
	)
	return instanceModUpdateReportDTO(report), err
}

func (controller *ModManagerController) InstallModFile(
	request InstallModFileRequest,
) (OperationDTO, error) {
	operation, err := controller.svc.InstallModFile(
		context.Background(),
		request.InstanceID,
		request.SourcePath,
		request.Name,
		request.Version,
	)
	return operationDTO(operation), err
}

func (controller *ModManagerController) SetModEnabled(
	id string,
	enabled bool,
) (InstalledModDTO, error) {
	mod, err := controller.svc.SetModEnabled(
		context.Background(),
		id,
		enabled,
	)
	return modDTO(mod), err
}

func (controller *ModManagerController) RemoveMod(id string) error {
	return controller.svc.DeleteMod(context.Background(), id)
}

type LaunchController struct {
	svc *application.Service
}

func NewLaunchController(service *application.Service) *LaunchController {
	return &LaunchController{svc: service}
}

type LaunchRequest struct {
	InstanceID string  `json:"instanceId"`
	AccountID  *string `json:"accountId,omitempty"`
}

type LaunchValidationDTO struct {
	Valid    bool     `json:"valid"`
	Issues   []string `json:"issues"`
	Warnings []string `json:"warnings"`
}

func (controller *LaunchController) ValidateLaunch(
	request LaunchRequest,
) (LaunchValidationDTO, error) {
	validation, err := controller.svc.ValidateLaunch(
		context.Background(),
		request.InstanceID,
		request.AccountID,
	)
	return LaunchValidationDTO{
		Valid:    validation.Valid,
		Issues:   nonNilStrings(validation.Issues),
		Warnings: nonNilStrings(validation.Warnings),
	}, err
}

func (controller *LaunchController) LaunchInstance(
	request LaunchRequest,
) (PlaySessionDTO, error) {
	session, err := controller.svc.Launch(
		context.Background(),
		request.InstanceID,
		request.AccountID,
	)
	return sessionDTO(session), err
}

func (controller *LaunchController) StopInstance(id string) error {
	return controller.svc.Stop(context.Background(), id, false)
}

func (controller *LaunchController) ForceStopInstance(id string) error {
	return controller.svc.Stop(context.Background(), id, true)
}

func (controller *LaunchController) GetRunningInstances() []string {
	return controller.svc.RunningInstanceIDs()
}

type StatisticsController struct {
	svc *application.Service
}

func NewStatisticsController(service *application.Service) *StatisticsController {
	return &StatisticsController{svc: service}
}

func (controller *StatisticsController) GetOverviewStatistics() (
	StatisticsDTO,
	error,
) {
	statistics, err := controller.svc.GetStatistics(context.Background())
	return statisticsDTO(statistics), err
}

type OperationController struct {
	svc *application.Service
}

func NewOperationController(service *application.Service) *OperationController {
	return &OperationController{svc: service}
}

func (controller *OperationController) ListOperations() ([]OperationDTO, error) {
	operations, err := controller.svc.ListOperations(context.Background())
	result := make([]OperationDTO, 0, len(operations))
	for _, operation := range operations {
		result = append(result, operationDTO(operation))
	}
	return result, err
}

func (controller *OperationController) CancelOperation(id string) error {
	return controller.svc.CancelOperation(id)
}

func (controller *OperationController) DeleteOperation(id string) error {
	return controller.svc.DeleteFinishedOperation(context.Background(), id)
}

func (controller *OperationController) ClearOperationHistory() (int64, error) {
	return controller.svc.ClearFinishedOperations(context.Background())
}

type SettingsController struct {
	svc      *application.Service
	base     *Base
	dataRoot *dataroot.Manager
}

func NewSettingsController(
	service *application.Service,
	base *Base,
	dataRoot *dataroot.Manager,
) *SettingsController {
	return &SettingsController{svc: service, base: base, dataRoot: dataRoot}
}

func (controller *SettingsController) GetSettings() (SettingsDTO, error) {
	settings, err := controller.svc.GetSettings(context.Background())
	return settingsDTO(settings), err
}

func (controller *SettingsController) UpdateSettings(
	request SettingsDTO,
) (SettingsDTO, error) {
	settings, err := controller.svc.SaveSettings(
		context.Background(),
		domain.Settings{
			Theme:                 request.Theme,
			Language:              request.Language,
			DownloadsParallel:     request.DownloadsParallel,
			ConfirmDeletion:       request.ConfirmDeletion,
			MinSessionDurationSec: request.MinSessionDurationSec,
			GlobalLaunchArguments: request.GlobalLaunchArguments,
			CheckForUpdates:       request.CheckForUpdates,
			UpdateChannel:         request.UpdateChannel,
			SkippedUpdateVersion:  request.SkippedUpdateVersion,
		},
	)
	return settingsDTO(settings), err
}

// GetDataFolder returns the current data folder, the default folder, and any
// error left behind by a failed relocation from a previous run.
func (controller *SettingsController) GetDataFolder() (DataFolderDTO, error) {
	current, err := controller.dataRoot.Current()
	if err != nil {
		return DataFolderDTO{}, err
	}
	lastError, err := controller.dataRoot.ReadError()
	if err != nil {
		return DataFolderDTO{}, err
	}
	return DataFolderDTO{
		CurrentPath: current,
		DefaultPath: controller.dataRoot.Home(),
		LastError:   lastError,
	}, nil
}

// SelectDataFolder opens a native directory picker for the data folder target.
func (controller *SettingsController) SelectDataFolder() (string, error) {
	if controller.base.ctx == nil {
		return "", nil
	}
	return wruntime.OpenDirectoryDialog(
		controller.base.ctx,
		wruntime.OpenDialogOptions{Title: "Select the launcher data folder"},
	)
}

// MoveDataFolder starts a background relocation of the launcher data folder to
// target. Progress is emitted through the "data-folder:progress" event; the
// application relaunches itself when the copy finishes.
func (controller *SettingsController) MoveDataFolder(target string) error {
	ctx := controller.base.ctx
	if ctx == nil {
		return errors.New("application runtime is unavailable")
	}
	if err := controller.svc.CanRelocateDataFolder(); err != nil {
		return err
	}
	controller.svc.SetDataFolderRelocating(true)
	wruntime.EventsEmit(ctx, "data-folder:progress", DataFolderProgressDTO{Phase: "preparing"})
	lastProgressEmit := time.Time{}
	err := controller.dataRoot.StartRelocation(
		target,
		func(copied, total int64) {
			progress := 0.0
			if total > 0 {
				progress = float64(copied) / float64(total)
			}
			now := time.Now()
			if now.Sub(lastProgressEmit) < 150*time.Millisecond && progress < 1 {
				return
			}
			lastProgressEmit = now
			wruntime.EventsEmit(ctx, "data-folder:progress", DataFolderProgressDTO{
				CopiedBytes: copied,
				TotalBytes:  total,
				Progress:    progress,
				Phase:       "moving",
			})
		},
		func(err error) {
			controller.svc.SetDataFolderRelocating(false)
			if err != nil {
				wruntime.EventsEmit(ctx, "data-folder:error", map[string]string{"message": err.Error()})
				return
			}
			wruntime.EventsEmit(ctx, "data-folder:progress", DataFolderProgressDTO{
				Progress: 1,
				Phase:    "relaunching",
			})
			go func() {
				time.Sleep(500 * time.Millisecond)
				wruntime.Quit(ctx)
			}()
		},
	)
	if err != nil {
		controller.svc.SetDataFolderRelocating(false)
		return err
	}
	return nil
}

func (controller *SettingsController) SelectGameArchive() (string, error) {
	if controller.base.ctx == nil {
		return "", nil
	}

	return wruntime.OpenFileDialog(
		controller.base.ctx,
		wruntime.OpenDialogOptions{
			Title: "Select a Vintage Story archive",
			Filters: []wruntime.FileFilter{
				{
					DisplayName: "Game archives (*.zip, *.tar.gz, *.tgz)",
					Pattern:     "*.zip;*.tar.gz;*.tgz",
				},
			},
		},
	)
}

func (controller *SettingsController) SelectGameDirectory() (string, error) {
	if controller.base.ctx == nil {
		return "", nil
	}

	return wruntime.OpenDirectoryDialog(
		controller.base.ctx,
		wruntime.OpenDialogOptions{Title: "Select a Vintage Story directory"},
	)
}

func (controller *SettingsController) SelectModFile() (string, error) {
	if controller.base.ctx == nil {
		return "", nil
	}

	return wruntime.OpenFileDialog(
		controller.base.ctx,
		wruntime.OpenDialogOptions{
			Title: "Select a mod file",
			Filters: []wruntime.FileFilter{
				{
					DisplayName: "Vintage Story mods",
					Pattern:     "*.zip;*.cs;*.dll",
				},
			},
		},
	)
}

func (controller *SettingsController) OpenDirectory(path string) error {
	if controller.base.ctx == nil {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return domain.NewError(domain.ErrValidation, "Directory not found")
	}

	wruntime.BrowserOpenURL(
		controller.base.ctx,
		"file://"+filepath.ToSlash(path),
	)
	return nil
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
