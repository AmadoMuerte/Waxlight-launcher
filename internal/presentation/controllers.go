package presentation

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/app"
	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/events"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/dataroot"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/downloader"
	"github.com/waxlight/waxlight-launcher/internal/version"
)

type AppController struct{}

func NewAppController() *AppController {
	return &AppController{}
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
	svc       *accounts.Service
	lifecycle *app.Lifecycle
}

func NewAccountController(service *accounts.Service, lifecycle *app.Lifecycle) *AccountController {
	return &AccountController{svc: service, lifecycle: lifecycle}
}

func (controller *AccountController) ListAccounts() ([]AccountDTO, error) {
	accounts, err := controller.svc.ListAccounts(controller.lifecycle.Context())
	result := make([]AccountDTO, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, accountDTO(account))
	}
	return result, err
}

func (controller *AccountController) Login(email, password string) (LoginResultDTO, error) {
	result, err := controller.svc.Login(controller.lifecycle.Context(), email, password)
	return loginResultDTO(result), err
}

func (controller *AccountController) CompleteTOTP(flowID, code string) (LoginResultDTO, error) {
	result, err := controller.svc.CompleteTOTP(controller.lifecycle.Context(), flowID, code)
	return loginResultDTO(result), err
}

func (controller *AccountController) CancelLogin(flowID string) error {
	return controller.svc.CancelLogin(flowID)
}

func (controller *AccountController) SetDefaultAccount(id string) error {
	return controller.svc.SelectAccount(controller.lifecycle.Context(), id)
}

func (controller *AccountController) RemoveAccount(id string) error {
	return controller.svc.RemoveAccount(controller.lifecycle.Context(), id)
}

func (controller *AccountController) ValidateAccount(id string) (AccountDTO, error) {
	account, err := controller.svc.ValidateAccount(controller.lifecycle.Context(), id)
	return accountDTO(account), err
}

func (controller *AccountController) ReauthenticateAccount(
	accountID string,
	email string,
	password string,
) (LoginResultDTO, error) {
	result, err := controller.svc.ReauthenticateAccount(
		controller.lifecycle.Context(),
		accountID,
		email,
		password,
	)
	return loginResultDTO(result), err
}

type GameVersionController struct {
	svc       *application.Service
	lifecycle *app.Lifecycle
}

func NewGameVersionController(service *application.Service, lifecycle *app.Lifecycle) *GameVersionController {
	return &GameVersionController{svc: service, lifecycle: lifecycle}
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
	versions, err := controller.svc.ListVersions(controller.lifecycle.Context())
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
	versions, err := controller.svc.ListAvailableVersions(controller.lifecycle.Context())
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
		controller.lifecycle.Context(),
		versionID,
	)
	return operationDTO(operation), err
}

func (controller *GameVersionController) InstallLocalVersion(
	request InstallVersionRequest,
) (OperationDTO, error) {
	operation, err := controller.svc.InstallVersion(
		controller.lifecycle.Context(),
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
	return controller.svc.DeleteVersion(controller.lifecycle.Context(), id, deleteFiles)
}

type InstanceController struct {
	svc       *application.Service
	lifecycle *app.Lifecycle
}

func NewInstanceController(service *application.Service, lifecycle *app.Lifecycle) *InstanceController {
	return &InstanceController{svc: service, lifecycle: lifecycle}
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

type CloneInstanceRequest struct {
	SourceID string `json:"sourceId"`
	Name     string `json:"name"`
}

func (controller *InstanceController) ListInstances() ([]InstanceDTO, error) {
	ctx := controller.lifecycle.Context()
	instances, err := controller.svc.ListInstances(ctx)
	result := make([]InstanceDTO, 0, len(instances))

	for _, instance := range instances {
		dto := instanceDTO(instance)
		mods, modsErr := controller.svc.ListMods(ctx, instance.ID)
		if modsErr != nil {
			slog.Warn("could not count mods for the instance list", "instance", instance.ID, "error", modsErr)
		}
		for _, mod := range mods {
			dto.TotalModCount++
			if mod.Enabled {
				dto.EnabledModCount++
			}
		}
		playtime, playtimeErr := controller.svc.GetInstancePlaytime(
			ctx,
			instance.ID,
		)
		if playtimeErr != nil {
			slog.Warn("could not read the playtime for the instance list", "instance", instance.ID, "error", playtimeErr)
		}
		dto.PlaytimeSeconds = playtime
		result = append(result, dto)
	}

	return result, err
}

func (controller *InstanceController) GetInstance(id string) (InstanceDTO, error) {
	instance, err := controller.svc.GetInstance(controller.lifecycle.Context(), id)
	return instanceDTO(instance), err
}

func (controller *InstanceController) CreateInstance(
	request CreateInstanceRequest,
) (InstanceDTO, error) {
	instance, err := controller.svc.CreateInstance(
		controller.lifecycle.Context(),
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
	ctx := controller.lifecycle.Context()
	instance, err := controller.svc.GetInstance(ctx, request.ID)
	if err != nil {
		return InstanceDTO{}, err
	}

	instance.Name = request.Name
	instance.Description = request.Description
	instance.GameVersionID = request.GameVersionID
	instance.DefaultAccountID = request.DefaultAccountID
	instance.LaunchArguments = request.LaunchArguments

	updated, err := controller.svc.UpdateInstance(ctx, instance)
	return instanceDTO(updated), err
}

func (controller *InstanceController) DeleteInstance(
	id string,
	deleteFiles bool,
) error {
	return controller.svc.DeleteInstance(controller.lifecycle.Context(), id, deleteFiles)
}

func (controller *InstanceController) CloneInstance(
	request CloneInstanceRequest,
) (InstanceDTO, error) {
	instance, err := controller.svc.CloneInstance(
		controller.lifecycle.Context(),
		request.SourceID,
		request.Name,
	)
	return instanceDTO(instance), err
}

type ServerController struct {
	svc       *application.Service
	lifecycle *app.Lifecycle
}

func NewServerController(service *application.Service, lifecycle *app.Lifecycle) *ServerController {
	return &ServerController{svc: service, lifecycle: lifecycle}
}

type SaveFavoriteServerRequest struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Address    string  `json:"address"`
	InstanceID *string `json:"instanceId,omitempty"`
}

func (controller *ServerController) ListFavoriteServers() ([]FavoriteServerDTO, error) {
	servers, err := controller.svc.ListFavoriteServers(controller.lifecycle.Context())
	if err != nil {
		return nil, err
	}
	result := make([]FavoriteServerDTO, 0, len(servers))
	for _, server := range servers {
		result = append(result, favoriteServerDTO(server))
	}
	return result, nil
}

func (controller *ServerController) ListPublicServers() ([]PublicServerDTO, error) {
	servers, err := controller.svc.ListPublicServers(controller.lifecycle.Context())
	if err != nil {
		return nil, err
	}
	result := make([]PublicServerDTO, 0, len(servers))
	for _, server := range servers {
		result = append(result, publicServerDTO(server))
	}
	return result, nil
}

func (controller *ServerController) SaveFavoriteServer(request SaveFavoriteServerRequest) (FavoriteServerDTO, error) {
	server, err := controller.svc.SaveFavoriteServer(controller.lifecycle.Context(), application.SaveFavoriteServerInput{
		ID: request.ID, Name: request.Name, Address: request.Address, InstanceID: request.InstanceID,
	})
	return favoriteServerDTO(server), err
}

func (controller *ServerController) DeleteFavoriteServer(id string) error {
	return controller.svc.DeleteFavoriteServer(controller.lifecycle.Context(), id)
}

type ModManagerController struct {
	svc       *application.Service
	lifecycle *app.Lifecycle
}

func NewModManagerController(service *application.Service, lifecycle *app.Lifecycle) *ModManagerController {
	return &ModManagerController{svc: service, lifecycle: lifecycle}
}

type InstallModFileRequest struct {
	InstanceID string `json:"instanceId"`
	SourcePath string `json:"sourcePath"`
	Name       string `json:"name"`
	Version    string `json:"version"`
}

type InstallModFilesRequest struct {
	InstanceID  string   `json:"instanceId"`
	SourcePaths []string `json:"sourcePaths"`
}

type InstallModFilesResultDTO struct {
	Installed []string            `json:"installed"`
	Skipped   []string            `json:"skipped"`
	Failed    []ModFileFailureDTO `json:"failed"`
}

type ModFileFailureDTO struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

func (controller *ModManagerController) ListInstalledMods(
	instanceID string,
) ([]InstalledModDTO, error) {
	mods, err := controller.svc.ListMods(controller.lifecycle.Context(), instanceID)
	result := make([]InstalledModDTO, 0, len(mods))
	for _, mod := range mods {
		result = append(result, modDTO(mod))
	}
	return result, err
}

func (controller *ModManagerController) LinkLocalMods(
	instanceID string,
) (LinkLocalModsResultDTO, error) {
	result, err := controller.svc.LinkLocalMods(controller.lifecycle.Context(), instanceID)
	return linkLocalModsResultDTO(result), err
}

func (controller *ModManagerController) CheckInstanceModUpdates(
	instanceID string,
) (InstanceModUpdateReportDTO, error) {
	report, err := controller.svc.CheckInstanceModUpdates(
		controller.lifecycle.Context(),
		instanceID,
	)
	return instanceModUpdateReportDTO(report), err
}

type UpdateInstanceModsRequest struct {
	InstanceID        string               `json:"instanceId"`
	Mods              []ModUpdateTargetDTO `json:"mods"`
	AllowIncompatible bool                 `json:"allowIncompatible"`
}

type ModUpdateTargetDTO struct {
	ModID     string `json:"modId"`
	VersionID string `json:"versionId"`
}

type ModUpdateResultDTO struct {
	Updated int `json:"updated"`
}

// UpdateInstanceMods updates several installed mods of one instance in a
// single coordinated operation; the backend creates exactly one automatic
// safety snapshot before the first update is applied.
func (controller *ModManagerController) UpdateInstanceMods(
	request UpdateInstanceModsRequest,
) (ModUpdateResultDTO, error) {
	targets := make([]application.ModUpdateTarget, 0, len(request.Mods))
	for _, mod := range request.Mods {
		targets = append(targets, application.ModUpdateTarget{
			ModID:     mod.ModID,
			VersionID: mod.VersionID,
		})
	}
	result, err := controller.svc.UpdateInstanceMods(
		controller.lifecycle.Context(),
		request.InstanceID,
		targets,
		request.AllowIncompatible,
	)
	if err != nil {
		slog.Warn("instance mod update failed", "instanceId", request.InstanceID, "error", err)
		return ModUpdateResultDTO{}, err
	}
	return ModUpdateResultDTO{Updated: result.Updated}, nil
}

func (controller *ModManagerController) InstallModFile(
	request InstallModFileRequest,
) (OperationDTO, error) {
	operation, err := controller.svc.InstallModFile(
		controller.lifecycle.Context(),
		request.InstanceID,
		request.SourcePath,
		request.Name,
		request.Version,
	)
	return operationDTO(operation), err
}

func (controller *ModManagerController) InstallModFiles(
	request InstallModFilesRequest,
) (InstallModFilesResultDTO, error) {
	result, err := controller.svc.InstallModFiles(
		controller.lifecycle.Context(),
		request.InstanceID,
		request.SourcePaths,
	)
	dto := InstallModFilesResultDTO{
		Installed: result.Installed,
		Skipped:   result.Skipped,
	}
	for _, failure := range result.Failed {
		dto.Failed = append(dto.Failed, ModFileFailureDTO{
			Path:  failure.Path,
			Error: failure.Error,
		})
	}
	return dto, err
}

func (controller *ModManagerController) SetModEnabled(
	id string,
	enabled bool,
) (InstalledModDTO, error) {
	mod, err := controller.svc.SetModEnabled(
		controller.lifecycle.Context(),
		id,
		enabled,
	)
	return modDTO(mod), err
}

func (controller *ModManagerController) RemoveMod(id string, deleteDependencies bool) error {
	return controller.svc.DeleteMod(controller.lifecycle.Context(), id, deleteDependencies)
}

func (controller *ModManagerController) GetModDeletePreview(id string) (ModDeletePreviewDTO, error) {
	preview, err := controller.svc.ModDeletePreview(controller.lifecycle.Context(), id)
	if err != nil {
		return ModDeletePreviewDTO{}, err
	}
	dto := ModDeletePreviewDTO{ModID: preview.ModID, ModName: preview.ModName, Dependencies: []InstalledModDTO{}}
	for _, dependency := range preview.Dependencies {
		dto.Dependencies = append(dto.Dependencies, modDTO(dependency))
	}
	return dto, nil
}

type LaunchController struct {
	svc       *application.Service
	lifecycle *app.Lifecycle
}

func NewLaunchController(service *application.Service, lifecycle *app.Lifecycle) *LaunchController {
	return &LaunchController{svc: service, lifecycle: lifecycle}
}

type LaunchRequest struct {
	InstanceID string  `json:"instanceId"`
	AccountID  *string `json:"accountId,omitempty"`
}

type ServerLaunchRequest struct {
	InstanceID string  `json:"instanceId"`
	AccountID  *string `json:"accountId,omitempty"`
	Address    string  `json:"address"`
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
		controller.lifecycle.Context(),
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
		controller.lifecycle.Context(),
		request.InstanceID,
		request.AccountID,
	)
	if err != nil {
		slog.Warn("launch request failed", "error", err)
		return PlaySessionDTO{}, err
	}
	return sessionDTO(session), nil
}

func (controller *LaunchController) LaunchServer(
	request ServerLaunchRequest,
) (PlaySessionDTO, error) {
	session, err := controller.svc.LaunchServer(
		controller.lifecycle.Context(),
		request.InstanceID,
		request.AccountID,
		request.Address,
	)
	if err != nil {
		slog.Warn("server launch request failed", "error", err)
		return PlaySessionDTO{}, err
	}
	return sessionDTO(session), nil
}

func (controller *LaunchController) StopInstance(id string) error {
	err := controller.svc.Stop(controller.lifecycle.Context(), id, false)
	if err != nil {
		slog.Warn("stop request failed", "error", err)
	}
	return err
}

func (controller *LaunchController) ForceStopInstance(id string) error {
	err := controller.svc.Stop(controller.lifecycle.Context(), id, true)
	if err != nil {
		slog.Warn("force stop request failed", "error", err)
	}
	return err
}

func (controller *LaunchController) GetRunningInstances() []string {
	return controller.svc.RunningInstanceIDs()
}

type StatisticsController struct {
	svc       *application.Service
	lifecycle *app.Lifecycle
}

func NewStatisticsController(service *application.Service, lifecycle *app.Lifecycle) *StatisticsController {
	return &StatisticsController{svc: service, lifecycle: lifecycle}
}

func (controller *StatisticsController) GetOverviewStatistics() (
	StatisticsDTO,
	error,
) {
	statistics, err := controller.svc.GetStatistics(controller.lifecycle.Context())
	return statisticsDTO(statistics), err
}

type OperationController struct {
	svc       *application.Service
	lifecycle *app.Lifecycle
}

func NewOperationController(service *application.Service, lifecycle *app.Lifecycle) *OperationController {
	return &OperationController{svc: service, lifecycle: lifecycle}
}

func (controller *OperationController) ListOperations() ([]OperationDTO, error) {
	operations, err := controller.svc.ListOperations(controller.lifecycle.Context())
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
	return controller.svc.DeleteFinishedOperation(controller.lifecycle.Context(), id)
}

func (controller *OperationController) ClearOperationHistory() (int64, error) {
	return controller.svc.ClearFinishedOperations(controller.lifecycle.Context())
}

type SettingsController struct {
	svc       *application.Service
	lifecycle *app.Lifecycle
	events    events.Publisher
	dataRoot  *dataroot.Manager
	downloads *downloader.Manager
}

func NewSettingsController(
	service *application.Service,
	lifecycle *app.Lifecycle,
	eventPublisher events.Publisher,
	dataRoot *dataroot.Manager,
	downloads *downloader.Manager,
) *SettingsController {
	return &SettingsController{svc: service, lifecycle: lifecycle, events: eventPublisher, dataRoot: dataRoot, downloads: downloads}
}

func (controller *SettingsController) GetSettings() (SettingsDTO, error) {
	settings, err := controller.svc.GetSettings(controller.lifecycle.Context())
	return settingsDTO(settings), err
}

func (controller *SettingsController) UpdateSettings(
	request SettingsDTO,
) (SettingsDTO, error) {
	settings, err := controller.svc.SaveSettings(
		controller.lifecycle.Context(),
		domain.Settings{
			Language:                 request.Language,
			DownloadsParallel:        request.DownloadsParallel,
			ConfirmDeletion:          request.ConfirmDeletion,
			GlobalLaunchArguments:    request.GlobalLaunchArguments,
			CheckForUpdates:          request.CheckForUpdates,
			UpdateChannel:            request.UpdateChannel,
			SkippedUpdateVersion:     request.SkippedUpdateVersion,
			TelemetryEnabled:         request.TelemetryEnabled,
			AutomaticSafetySnapshots: request.AutomaticSafetySnapshots,
		},
	)
	if err != nil {
		return settingsDTO(settings), err
	}
	controller.downloads.SetLimit(request.DownloadsParallel)
	return settingsDTO(settings), nil
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
	ctx := controller.lifecycle.Context()
	return wruntime.OpenDirectoryDialog(
		ctx,
		wruntime.OpenDialogOptions{Title: "Select the launcher data folder"},
	)
}

// MoveDataFolder starts a background relocation of the launcher data folder to
// target. Progress is emitted through the "data-folder:progress" event; the
// application relaunches itself when the copy finishes.
func (controller *SettingsController) MoveDataFolder(target string) error {
	if err := controller.svc.CanRelocateDataFolder(); err != nil {
		return err
	}
	controller.svc.SetDataFolderRelocating(true)
	controller.events.Publish("data-folder:progress", DataFolderProgressDTO{Phase: "preparing"})
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
			controller.events.Publish("data-folder:progress", DataFolderProgressDTO{
				CopiedBytes: copied,
				TotalBytes:  total,
				Progress:    progress,
				Phase:       "moving",
			})
		},
		func(err error) {
			controller.svc.SetDataFolderRelocating(false)
			if err != nil {
				controller.events.Publish("data-folder:error", map[string]string{"message": err.Error()})
				return
			}
			controller.events.Publish("data-folder:progress", DataFolderProgressDTO{
				Progress: 1,
				Phase:    "relaunching",
			})
			controller.lifecycle.Go(func(workerCtx context.Context) {
				select {
				case <-time.After(500 * time.Millisecond):
					wruntime.Quit(workerCtx)
				case <-workerCtx.Done():
				}
			})
		},
	)
	if err != nil {
		controller.svc.SetDataFolderRelocating(false)
		return err
	}
	return nil
}

func (controller *SettingsController) SelectGameArchive() (string, error) {
	return wruntime.OpenFileDialog(
		controller.lifecycle.Context(),
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
	return wruntime.OpenDirectoryDialog(
		controller.lifecycle.Context(),
		wruntime.OpenDialogOptions{Title: "Select a Vintage Story directory"},
	)
}

func (controller *SettingsController) SelectModFile() (string, error) {
	return wruntime.OpenFileDialog(
		controller.lifecycle.Context(),
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

func (controller *SettingsController) SelectModFiles() ([]string, error) {
	return wruntime.OpenMultipleFilesDialog(
		controller.lifecycle.Context(),
		wruntime.OpenDialogOptions{
			Title: "Select mod files",
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
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return domain.NewError(domain.ErrValidation, "Directory not found")
	}

	// Wails rejects file:// URLs on Linux ("scheme not allowed"), so open the
	// directory through the platform file manager directly.
	if err := openDirectoryNative(path); err != nil {
		return &domain.AppError{
			Code:    domain.ErrFilePermission,
			Message: "Could not open the directory",
			Cause:   err,
		}
	}
	return nil
}

func openDirectoryNative(path string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "windows":
		command = "explorer.exe"
		args = []string{path}
	case "darwin":
		command = "open"
		args = []string{path}
	default:
		command = "xdg-open"
		args = []string{path}
	}
	return exec.Command(command, args...).Start()
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
