package wails

import (
	"log/slog"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
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

// InstallModFileRequest selects one local mod archive and target instance.
type InstallModFileRequest struct {
	InstanceID string `json:"instanceId"`
	SourcePath string `json:"sourcePath"`
	Name       string `json:"name"`
	Version    string `json:"version"`
}

// InstallModFilesRequest selects local mod archives and their target instance.
type InstallModFilesRequest struct {
	InstanceID  string   `json:"instanceId"`
	SourcePaths []string `json:"sourcePaths"`
}

// InstallModFilesResultDTO separates installed, skipped, and failed local mod imports.
type InstallModFilesResultDTO struct {
	Installed []string            `json:"installed"`
	Skipped   []string            `json:"skipped"`
	Failed    []ModFileFailureDTO `json:"failed"`
}

// ModFileFailureDTO identifies a local mod archive that could not be installed.
type ModFileFailureDTO struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// ListInstalledMods returns mods installed in an instance and their enabled state.
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

// LinkLocalMods links compatible cached mod files into an instance.
func (controller *ModManagerController) LinkLocalMods(
	instanceID string,
) (LinkLocalModsResultDTO, error) {
	result, err := controller.catalog.LinkLocalMods(controller.lifecycle.Context(), instanceID)
	return linkLocalModsResultDTO(result), err
}

// CheckInstanceModUpdates finds compatible catalog updates for managed mods in an instance.
func (controller *ModManagerController) CheckInstanceModUpdates(
	instanceID string,
) (InstanceModUpdateReportDTO, error) {
	report, err := controller.catalog.CheckInstanceModUpdates(
		controller.lifecycle.Context(),
		instanceID,
	)
	return instanceModUpdateReportDTO(report), err
}

// UpdateInstanceModsRequest selects catalog versions to apply to an instance.
type UpdateInstanceModsRequest struct {
	InstanceID        string               `json:"instanceId"`
	Mods              []ModUpdateTargetDTO `json:"mods"`
	AllowIncompatible bool                 `json:"allowIncompatible"`
}

// ModUpdateTargetDTO is the frontend-safe representation of mod update target.
type ModUpdateTargetDTO struct {
	ModID     string `json:"modId"`
	VersionID string `json:"versionId"`
}

// ModUpdateResultDTO reports how many selected mods were updated or skipped.
type ModUpdateResultDTO struct {
	Updated         int `json:"updated"`
	SkippedByPolicy int `json:"skippedByPolicy"`
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
	return ModUpdateResultDTO{Updated: result.Updated, SkippedByPolicy: result.SkippedByPolicy}, nil
}

// SetModUpdatePolicy changes how a managed mod participates in automatic updates.
func (controller *ModManagerController) SetModUpdatePolicy(id, policy string) (InstalledModDTO, error) {
	mod, err := controller.svc.SetUpdatePolicy(controller.lifecycle.Context(), id, mods.UpdatePolicy(policy))
	return modDTO(mod), err
}

// InstallModFile installs one local mod archive into an instance.
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

// InstallModFiles installs several local mod archives and reports individual failures.
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

// SetModEnabled enables or disables an installed mod for its owning instance.
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

// RemoveMod removes an installed mod and optionally its dependent mods.
func (controller *ModManagerController) RemoveMod(id string, deleteDependencies bool) error {
	return controller.svc.DeleteMod(controller.lifecycle.Context(), id, deleteDependencies)
}

// GetModDeletePreview reports dependent mods that would be affected by removal.
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
