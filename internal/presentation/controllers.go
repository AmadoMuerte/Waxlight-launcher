package presentation

import (
	"context"
	"log/slog"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/app"
	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/launching"
	"github.com/waxlight/waxlight-launcher/internal/mods"
	"github.com/waxlight/waxlight-launcher/internal/operations"
	"github.com/waxlight/waxlight-launcher/internal/statistics"
	"github.com/waxlight/waxlight-launcher/internal/version"
	"github.com/waxlight/waxlight-launcher/internal/versions"
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
	svc       gameVersionCapabilities
	lifecycle *app.Lifecycle
}

type gameVersionCapabilities interface {
	List(context.Context) ([]versions.GameVersion, error)
	ListAvailable(context.Context) ([]versions.AvailableGameVersion, error)
	InstallCatalog(context.Context, string) (versions.Install, error)
	InstallLocal(context.Context, string, string, string, string, string) (operations.Operation, error)
	Remove(context.Context, string, bool) error
}

func NewGameVersionController(service gameVersionCapabilities, lifecycle *app.Lifecycle) *GameVersionController {
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
	versions, err := controller.svc.List(controller.lifecycle.Context())
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
	versions, err := controller.svc.ListAvailable(controller.lifecycle.Context())
	result := make([]AvailableGameVersionDTO, 0, len(versions))
	for _, version := range versions {
		result = append(result, availableVersionDTO(version))
	}
	return result, err
}

func (controller *GameVersionController) InstallVersion(
	versionID string,
) (OperationDTO, error) {
	install, err := controller.svc.InstallCatalog(
		controller.lifecycle.Context(),
		versionID,
	)
	return operationDTO(install.Operation), err
}

func (controller *GameVersionController) InstallLocalVersion(
	request InstallVersionRequest,
) (OperationDTO, error) {
	operation, err := controller.svc.InstallLocal(
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
	return controller.svc.Remove(controller.lifecycle.Context(), id, deleteFiles)
}

type InstanceController struct {
	svc        *application.Service
	creator    instanceCreator
	queries    instanceQueries
	updater    instanceUpdater
	deleter    instanceDeleter
	cloner     instanceCloner
	statistics instancePlaytime
	modCounter instanceModCounter
	lifecycle  *app.Lifecycle
}

type instanceCreator interface {
	Create(context.Context, instances.CreateInput) (instances.Instance, error)
}

type instanceQueries interface {
	List(context.Context) ([]instances.Instance, error)
	Get(context.Context, string) (instances.Instance, error)
}

type instanceUpdater interface {
	Update(context.Context, instances.Instance) (instances.Instance, error)
}

type instanceDeleter interface {
	Delete(context.Context, string, bool) error
}

type instanceCloner interface {
	Clone(context.Context, string, string) (instances.Instance, error)
}

type instancePlaytime interface {
	InstancePlaytime(context.Context, string) (int64, error)
}

type instanceModCounter interface {
	ListMods(context.Context, string) ([]mods.InstalledMod, error)
}

func NewInstanceController(
	service *application.Service,
	creator instanceCreator,
	queries instanceQueries,
	updater instanceUpdater,
	deleter instanceDeleter,
	cloner instanceCloner,
	playtime instancePlaytime,
	modCounter instanceModCounter,
	lifecycle *app.Lifecycle,
) *InstanceController {
	return &InstanceController{
		svc: service, creator: creator, queries: queries, updater: updater,
		deleter: deleter, cloner: cloner, statistics: playtime, lifecycle: lifecycle,
		modCounter: modCounter,
	}
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
	storedInstances, err := controller.queries.List(ctx)
	result := make([]InstanceDTO, 0, len(storedInstances))

	for _, instance := range storedInstances {
		dto := instanceDTO(instance)
		mods, modsErr := controller.modCounter.ListMods(ctx, instance.ID)
		if modsErr != nil {
			slog.Warn("could not count mods for the instance list", "instance", instance.ID, "error", modsErr)
		}
		for _, mod := range mods {
			dto.TotalModCount++
			if mod.Enabled {
				dto.EnabledModCount++
			}
		}
		playtime, playtimeErr := controller.statistics.InstancePlaytime(
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
	instance, err := controller.queries.Get(controller.lifecycle.Context(), id)
	return instanceDTO(instance), err
}

func (controller *InstanceController) CreateInstance(
	request CreateInstanceRequest,
) (InstanceDTO, error) {
	instance, err := controller.creator.Create(
		controller.lifecycle.Context(),
		instances.CreateInput{
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
	instance, err := controller.queries.Get(ctx, request.ID)
	if err != nil {
		return InstanceDTO{}, err
	}

	instance.Name = request.Name
	instance.Description = request.Description
	instance.GameVersionID = request.GameVersionID
	instance.DefaultAccountID = request.DefaultAccountID
	instance.LaunchArguments = request.LaunchArguments

	updated, err := controller.updater.Update(ctx, instance)
	return instanceDTO(updated), err
}

func (controller *InstanceController) DeleteInstance(
	id string,
	deleteFiles bool,
) error {
	return controller.deleter.Delete(controller.lifecycle.Context(), id, deleteFiles)
}

func (controller *InstanceController) CloneInstance(
	request CloneInstanceRequest,
) (InstanceDTO, error) {
	instance, err := controller.cloner.Clone(
		controller.lifecycle.Context(),
		request.SourceID,
		request.Name,
	)
	return instanceDTO(instance), err
}

type ModManagerController struct {
	svc       *mods.Service
	catalog   *mods.CatalogService
	lifecycle *app.Lifecycle
}

func NewModManagerController(service *mods.Service, catalog *mods.CatalogService, lifecycle *app.Lifecycle) *ModManagerController {
	return &ModManagerController{svc: service, catalog: catalog, lifecycle: lifecycle}
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
	result, err := controller.catalog.LinkLocalMods(controller.lifecycle.Context(), instanceID)
	return linkLocalModsResultDTO(result), err
}

func (controller *ModManagerController) CheckInstanceModUpdates(
	instanceID string,
) (InstanceModUpdateReportDTO, error) {
	report, err := controller.catalog.CheckInstanceModUpdates(
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
	targets := make([]mods.ModUpdateTarget, 0, len(request.Mods))
	for _, mod := range request.Mods {
		targets = append(targets, mods.ModUpdateTarget{
			ModID:     mod.ModID,
			VersionID: mod.VersionID,
		})
	}
	result, err := controller.catalog.UpdateInstanceMods(
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
	svc       *launching.Coordinator
	lifecycle *app.Lifecycle
}

func NewLaunchController(service *launching.Coordinator, lifecycle *app.Lifecycle) *LaunchController {
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
	svc       statisticsQueries
	lifecycle *app.Lifecycle
}

type statisticsQueries interface {
	Overview(context.Context) (statistics.Statistics, error)
}

func NewStatisticsController(service statisticsQueries, lifecycle *app.Lifecycle) *StatisticsController {
	return &StatisticsController{svc: service, lifecycle: lifecycle}
}

func (controller *StatisticsController) GetOverviewStatistics() (
	StatisticsDTO,
	error,
) {
	statistics, err := controller.svc.Overview(controller.lifecycle.Context())
	return statisticsDTO(statistics), err
}

type OperationController struct {
	operations *operations.Manager
	lifecycle  *app.Lifecycle
}

func NewOperationController(manager *operations.Manager, lifecycle *app.Lifecycle) *OperationController {
	return &OperationController{operations: manager, lifecycle: lifecycle}
}

func (controller *OperationController) ListOperations() ([]OperationDTO, error) {
	tracked, err := controller.operations.List(controller.lifecycle.Context())
	result := make([]OperationDTO, 0, len(tracked))
	for _, operation := range tracked {
		result = append(result, operationDTO(operation))
	}
	return result, err
}

func (controller *OperationController) CancelOperation(id string) error {
	return controller.operations.Cancel(id)
}

func (controller *OperationController) DeleteOperation(id string) error {
	return controller.operations.Delete(controller.lifecycle.Context(), id)
}

func (controller *OperationController) ClearOperationHistory() (int64, error) {
	return controller.operations.Clear(controller.lifecycle.Context())
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
