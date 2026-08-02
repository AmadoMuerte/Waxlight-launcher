package presentation

import (
	"context"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/domain"
)

type ModCatalogController struct {
	svc *application.Service
}

func NewModCatalogController(service *application.Service) *ModCatalogController {
	return &ModCatalogController{svc: service}
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
	query := domain.ModSearchQuery{
		Text: request.Text, GameVersion: request.GameVersion,
		Side: domain.ModSide(request.Side), Tags: request.Tags,
		CompatibleOnly: request.CompatibleOnly, InstanceID: request.InstanceID,
		Sort: request.Sort, Page: request.Page, PageSize: request.PageSize,
	}
	if request.UpdatedAfter != nil && *request.UpdatedAfter != "" {
		value, err := time.Parse(time.RFC3339, *request.UpdatedAfter)
		if err != nil {
			return ModSearchResultDTO{}, domain.NewError(domain.ErrValidation, "Invalid updated date")
		}
		query.UpdatedAfter = &value
	}
	result, err := controller.svc.SearchMods(context.Background(), query)
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
	mod, err := controller.svc.GetCatalogMod(context.Background(), modID)
	return modDetailsDTO(mod), err
}

func (controller *ModCatalogController) ListDownloadedMods() ([]DownloadedModDTO, error) {
	mods, err := controller.svc.ListDownloadedMods(context.Background())
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
		context.Background(),
		domain.DownloadModRequest{
			ModID: request.ModID, VersionID: request.VersionID,
			InstanceIDs: request.InstanceIDs, DownloadOnly: request.DownloadOnly,
			AllowIncompatible: request.AllowIncompatible,
		},
	)
	return modInstallResultDTO(result), err
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
		context.Background(), request.ModID, request.VersionID,
		request.InstanceIDs, request.AllowIncompatible,
	)
	return modInstallResultDTO(result), err
}

func (controller *ModCatalogController) RemoveDownloadedMod(
	modID string,
	versionID string,
) error {
	return controller.svc.RemoveDownloadedMod(context.Background(), modID, versionID)
}

func (controller *ModCatalogController) CancelModTask(taskID string) error {
	return controller.svc.CancelModTask(taskID)
}

func (controller *ModCatalogController) CheckModUpdates(
	modID string,
) ([]DownloadedModDTO, error) {
	mods, err := controller.svc.CheckModUpdates(context.Background(), modID)
	dto := make([]DownloadedModDTO, 0, len(mods))
	for _, mod := range mods {
		dto = append(dto, downloadedModDTO(mod))
	}
	return dto, err
}
