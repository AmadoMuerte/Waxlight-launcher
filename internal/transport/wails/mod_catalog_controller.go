package wails

import (
	"time"

	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/mods"
)

// ModCatalogController exposes ModDB browsing, catalog downloads, and
// downloaded-mod management to the frontend. It stays limited to DTO
// conversion and feature invocation.
type ModCatalogController struct {
	svc       *mods.CatalogService
	lifecycle lifecycle
}

func NewModCatalogController(service *mods.CatalogService, lifecycle lifecycle) *ModCatalogController {
	return &ModCatalogController{svc: service, lifecycle: lifecycle}
}

type ModSearchRequest struct {
	Text           string   `json:"text"`
	GameVersion    string   `json:"gameVersion"`
	Side           string   `json:"side"`
	UpdatedAfter   *string  `json:"updatedAfter,omitempty"`
	Tags           []string `json:"tags"`
	CompatibleOnly bool     `json:"compatibleOnly"`
	InstanceID     string   `json:"instanceId"`
	Sort           string   `json:"sort"`
	Page           int      `json:"page"`
	PageSize       int      `json:"pageSize"`
}

func (controller *ModCatalogController) SearchMods(
	request ModSearchRequest,
) (ModSearchResultDTO, error) {
	query := mods.ModSearchQuery{
		Text: request.Text, GameVersion: request.GameVersion,
		Side: mods.ModSide(request.Side), Tags: request.Tags,
		CompatibleOnly: request.CompatibleOnly, InstanceID: request.InstanceID,
		Sort: request.Sort, Page: request.Page, PageSize: request.PageSize,
	}
	if request.UpdatedAfter != nil && *request.UpdatedAfter != "" {
		value, err := time.Parse(time.RFC3339, *request.UpdatedAfter)
		if err != nil {
			return ModSearchResultDTO{}, errs.NewError(errs.ErrValidation, "Invalid updated date")
		}
		query.UpdatedAfter = &value
	}
	result, err := controller.svc.SearchMods(controller.lifecycle.Context(), query)
	dto := ModSearchResultDTO{
		Items: []ModSummaryDTO{}, Page: result.Page, PageSize: result.PageSize,
		TotalItems: result.TotalItems, TotalPages: result.TotalPages,
		HasNext: result.HasNext,
	}
	for _, item := range result.Items {
		dto.Items = append(dto.Items, modSummaryDTO(item))
	}
	return dto, err
}

func (controller *ModCatalogController) GetMod(modID string) (ModDetailsDTO, error) {
	mod, err := controller.svc.GetCatalogMod(controller.lifecycle.Context(), modID)
	return modDetailsDTO(mod), err
}

func (controller *ModCatalogController) ListModTags() ([]ModTagDTO, error) {
	tags, err := controller.svc.ListModTags(controller.lifecycle.Context())
	dtos := make([]ModTagDTO, 0, len(tags))
	for _, tag := range tags {
		dtos = append(dtos, ModTagDTO{Name: tag.Name, Count: tag.Count})
	}
	return dtos, err
}

func (controller *ModCatalogController) ListDownloadedMods() ([]DownloadedModDTO, error) {
	mods, err := controller.svc.ListDownloadedMods(controller.lifecycle.Context())
	dto := make([]DownloadedModDTO, 0, len(mods))
	for _, mod := range mods {
		dto = append(dto, downloadedModDTO(mod))
	}
	return dto, err
}

type DownloadCatalogModRequest struct {
	ModID             string   `json:"modId"`
	VersionID         string   `json:"versionId"`
	InstanceIDs       []string `json:"instanceIds"`
	DownloadOnly      bool     `json:"downloadOnly"`
	AllowIncompatible bool     `json:"allowIncompatible"`
}

func (controller *ModCatalogController) DownloadMod(
	request DownloadCatalogModRequest,
) (ModInstallResultDTO, error) {
	result, err := controller.svc.DownloadCatalogMod(
		controller.lifecycle.Context(),
		mods.DownloadModRequest{
			ModID: request.ModID, VersionID: request.VersionID,
			InstanceIDs: request.InstanceIDs, DownloadOnly: request.DownloadOnly,
			AllowIncompatible: request.AllowIncompatible,
		},
	)
	return modInstallResultDTO(result), err
}

type DownloadModTargetRequest struct {
	ModID     string `json:"modId"`
	VersionID string `json:"versionId"`
}

type DownloadModsBatchRequest struct {
	InstanceID string                     `json:"instanceId"`
	Targets    []DownloadModTargetRequest `json:"targets"`
}

func (controller *ModCatalogController) DownloadModsBatch(
	request DownloadModsBatchRequest,
) []BatchModInstallResultDTO {
	targets := make([]mods.DownloadModTarget, 0, len(request.Targets))
	for _, target := range request.Targets {
		targets = append(targets, mods.DownloadModTarget{
			ModID: target.ModID, VersionID: target.VersionID,
		})
	}
	return batchModInstallResultsDTO(controller.svc.DownloadCatalogModsBatch(
		controller.lifecycle.Context(),
		mods.BatchDownloadModsRequest{InstanceID: request.InstanceID, Targets: targets},
	))
}

type InstallDownloadedModRequest struct {
	ModID             string   `json:"modId"`
	VersionID         string   `json:"versionId"`
	InstanceIDs       []string `json:"instanceIds"`
	AllowIncompatible bool     `json:"allowIncompatible"`
}

func (controller *ModCatalogController) InstallDownloadedMod(
	request InstallDownloadedModRequest,
) (ModInstallResultDTO, error) {
	result, err := controller.svc.InstallDownloadedMod(
		controller.lifecycle.Context(), request.ModID, request.VersionID,
		request.InstanceIDs, request.AllowIncompatible,
	)
	return modInstallResultDTO(result), err
}

func (controller *ModCatalogController) RemoveDownloadedMod(
	modID string,
	versionID string,
) error {
	return controller.svc.RemoveDownloadedMod(controller.lifecycle.Context(), modID, versionID)
}

func (controller *ModCatalogController) PreviewUnusedDownloadedMods() (DownloadedModCleanupResultDTO, error) {
	result, err := controller.svc.PreviewUnusedDownloadedMods(controller.lifecycle.Context())
	return downloadedModCleanupResultDTO(result), err
}

func (controller *ModCatalogController) RemoveUnusedDownloadedMods() (DownloadedModCleanupResultDTO, error) {
	result, err := controller.svc.RemoveUnusedDownloadedMods(controller.lifecycle.Context())
	return downloadedModCleanupResultDTO(result), err
}

func (controller *ModCatalogController) UploadMods(
	sourcePaths []string,
) (UploadModsResultDTO, error) {
	result, err := controller.svc.UploadMods(controller.lifecycle.Context(), sourcePaths)
	return uploadModsResultDTO(result), err
}

func (controller *ModCatalogController) CancelModTask(taskID string) error {
	return controller.svc.CancelModTask(taskID)
}

func (controller *ModCatalogController) CheckModUpdates(
	modID string,
) ([]DownloadedModDTO, error) {
	mods, err := controller.svc.CheckModUpdates(controller.lifecycle.Context(), modID)
	dto := make([]DownloadedModDTO, 0, len(mods))
	for _, mod := range mods {
		dto = append(dto, downloadedModDTO(mod))
	}
	return dto, err
}
