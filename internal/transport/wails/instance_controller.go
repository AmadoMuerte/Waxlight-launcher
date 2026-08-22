package wails

import (
	"context"
	"log/slog"

	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/mods"
	"github.com/waxlight/waxlight-launcher/internal/operations"
)

type instanceCreator interface {
	Create(context.Context, instances.CreateInput) (instances.Instance, error)
}

type instanceQueries interface {
	List(context.Context) ([]instances.Instance, error)
	Get(context.Context, string) (instances.Instance, error)
}

type instanceUpdater interface {
	UpdateWithCover(context.Context, instances.Instance, *string) (instances.Instance, error)
	SetPinned(context.Context, string, bool) (instances.Instance, error)
}

type instanceCoverDialog interface {
	SelectInstanceCover() (string, error)
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

type instanceDataMigration interface {
	Detect(context.Context) ([]instances.MigrationCandidate, error)
	Inspect(string) (instances.MigrationCandidate, error)
	Start(context.Context, instances.MigrationImportRequest) (operations.Operation, error)
}

// InstanceController exposes instance CRUD to the frontend. It stays limited
// to DTO conversion and feature invocation.
type InstanceController struct {
	creator    instanceCreator
	queries    instanceQueries
	updater    instanceUpdater
	deleter    instanceDeleter
	cloner     instanceCloner
	sessions   instancePlaytime
	modCounter instanceModCounter
	dialogs    instanceCoverDialog
	lifecycle  lifecycle
	migration  instanceDataMigration
}

func NewInstanceController(
	creator instanceCreator,
	queries instanceQueries,
	updater instanceUpdater,
	deleter instanceDeleter,
	cloner instanceCloner,
	sessionQueries instancePlaytime,
	modCounter instanceModCounter,
	migration instanceDataMigration,
	lifecycle lifecycle,
	dialogs instanceCoverDialog,
) *InstanceController {
	return &InstanceController{
		creator: creator, queries: queries, updater: updater,
		deleter: deleter, cloner: cloner, sessions: sessionQueries, lifecycle: lifecycle,
		modCounter: modCounter, migration: migration,
		dialogs: dialogs,
	}
}

type MigrationCandidateDTO struct {
	Path                string   `json:"path"`
	WorldCount          int      `json:"worldCount"`
	ModCount            int      `json:"modCount"`
	TotalBytes          int64    `json:"totalBytes"`
	TotalFiles          int64    `json:"totalFiles"`
	HasClientSettings   bool     `json:"hasClientSettings"`
	HasModConfig        bool     `json:"hasModConfig"`
	DetectedGameVersion string   `json:"detectedGameVersion"`
	VersionConfidence   string   `json:"versionConfidence"`
	Warnings            []string `json:"warnings"`
}

type MigrationImportRequest struct {
	SourcePath    string `json:"sourcePath"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	GameVersionID string `json:"gameVersionId"`
}

func migrationCandidateDTO(candidate instances.MigrationCandidate) MigrationCandidateDTO {
	return MigrationCandidateDTO{Path: candidate.Path, WorldCount: candidate.WorldCount, ModCount: candidate.ModCount,
		TotalBytes: candidate.TotalBytes, TotalFiles: candidate.TotalFiles, HasClientSettings: candidate.HasClientSettings,
		HasModConfig: candidate.HasModConfig, DetectedGameVersion: candidate.DetectedGameVersion,
		VersionConfidence: candidate.VersionConfidence, Warnings: nonNilStrings(candidate.Warnings)}
}

func (controller *InstanceController) DetectExistingVintageStoryData() ([]MigrationCandidateDTO, error) {
	candidates, err := controller.migration.Detect(controller.lifecycle.Context())
	result := make([]MigrationCandidateDTO, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, migrationCandidateDTO(candidate))
	}
	return result, err
}

func (controller *InstanceController) InspectExistingVintageStoryData(path string) (MigrationCandidateDTO, error) {
	candidate, err := controller.migration.Inspect(path)
	return migrationCandidateDTO(candidate), err
}

func (controller *InstanceController) StartExistingDataImport(request MigrationImportRequest) (OperationDTO, error) {
	operation, err := controller.migration.Start(controller.lifecycle.Context(), instances.MigrationImportRequest{
		SourcePath: request.SourcePath, Name: request.Name, Description: request.Description,
		GameVersionID: request.GameVersionID,
	})
	return operationDTO(operation), err
}

func (controller *InstanceController) SelectInstanceCover() (string, error) {
	return controller.dialogs.SelectInstanceCover()
}

type CreateInstanceRequest struct {
	Name                 string            `json:"name"`
	Description          string            `json:"description"`
	GameVersionID        string            `json:"gameVersionId"`
	GameClient           string            `json:"gameClient"`
	DefaultAccountID     *string           `json:"defaultAccountId,omitempty"`
	Directory            string            `json:"directory"`
	LaunchArguments      []string          `json:"launchArguments"`
	EnvironmentVariables map[string]string `json:"environmentVariables"`
}

type UpdateInstanceRequest struct {
	ID                   string             `json:"id"`
	Name                 string             `json:"name"`
	Description          string             `json:"description"`
	GameVersionID        string             `json:"gameVersionId"`
	GameClient           *string            `json:"gameClient,omitempty"`
	DefaultAccountID     *string            `json:"defaultAccountId,omitempty"`
	LaunchArguments      []string           `json:"launchArguments"`
	EnvironmentVariables *map[string]string `json:"environmentVariables,omitempty"`
	CoverSourcePath      *string            `json:"coverSourcePath,omitempty"`
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
		playtime, playtimeErr := controller.sessions.InstancePlaytime(
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
			Name:                 request.Name,
			Description:          request.Description,
			GameVersionID:        request.GameVersionID,
			GameClient:           instances.GameClient(request.GameClient),
			DefaultAccountID:     request.DefaultAccountID,
			Directory:            request.Directory,
			LaunchArguments:      request.LaunchArguments,
			EnvironmentVariables: request.EnvironmentVariables,
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
	if request.GameClient != nil {
		instance.GameClient = instances.GameClient(*request.GameClient)
	}
	instance.DefaultAccountID = request.DefaultAccountID
	instance.LaunchArguments = request.LaunchArguments
	if request.EnvironmentVariables != nil {
		instance.EnvironmentVariables = *request.EnvironmentVariables
	}
	updated, err := controller.updater.UpdateWithCover(ctx, instance, request.CoverSourcePath)
	return instanceDTO(updated), err
}

func (controller *InstanceController) SetInstancePinned(id string, pinned bool) (InstanceDTO, error) {
	instance, err := controller.updater.SetPinned(controller.lifecycle.Context(), id, pinned)
	return instanceDTO(instance), err
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
