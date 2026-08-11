package wails

import (
	"log/slog"

	"github.com/waxlight/waxlight-launcher/internal/mods"
)

// ModManagerController exposes installed-mod management to the frontend. It
// stays limited to DTO conversion and feature invocation.
type ModManagerController struct {
	svc       *mods.Service
	catalog   *mods.CatalogService
	lifecycle lifecycle
}

func NewModManagerController(service *mods.Service, catalog *mods.CatalogService, lifecycle lifecycle) *ModManagerController {
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
