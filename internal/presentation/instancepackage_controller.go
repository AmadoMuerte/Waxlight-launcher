package presentation

import (
	"context"
	"path/filepath"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/domain"
)

type InstancePackageController struct {
	svc  *application.Service
	base *Base
}

func NewInstancePackageController(service *application.Service, base *Base) *InstancePackageController {
	return &InstancePackageController{svc: service, base: base}
}

type ExportInstanceRequest struct {
	InstanceID  string            `json:"instanceId"`
	TargetPath  string            `json:"targetPath"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Author      *PackageAuthorDTO `json:"author,omitempty"`
}

func (controller *InstancePackageController) ExportInstance(
	request ExportInstanceRequest,
) (PackageManifestDTO, error) {
	var author *domain.PackageAuthor
	if request.Author != nil {
		author = &domain.PackageAuthor{
			Name:     request.Author.Name,
			Homepage: request.Author.Homepage,
			Source:   request.Author.Source,
		}
	}
	manifest, err := controller.svc.ExportInstance(
		context.Background(),
		request.InstanceID,
		request.TargetPath,
		domain.ExportInstanceOptions{
			Name:        request.Name,
			Description: request.Description,
			Author:      author,
		},
	)
	return packageManifestDTO(manifest), err
}

func (controller *InstancePackageController) InspectPackage(
	packagePath string,
) (PackageInspectionDTO, error) {
	inspection, err := controller.svc.InspectPackage(context.Background(), packagePath)
	return packageInspectionDTO(inspection), err
}

type ImportInstanceRequest struct {
	PackagePath       string `json:"packagePath"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	Directory         string `json:"directory"`
	GameVersionID     string `json:"gameVersionId"`
	InstallVersion    bool   `json:"installVersion"`
	AllowIncompatible bool   `json:"allowIncompatible"`
	SkipUnavailable   bool   `json:"skipUnavailable"`
}

func (controller *InstancePackageController) ImportPackage(
	request ImportInstanceRequest,
) (ImportReportDTO, error) {
	report, err := controller.svc.ImportPackage(
		context.Background(),
		request.PackagePath,
		domain.ImportInstanceOptions{
			Name:              request.Name,
			Description:       request.Description,
			Directory:         request.Directory,
			GameVersionID:     request.GameVersionID,
			InstallVersion:    request.InstallVersion,
			AllowIncompatible: request.AllowIncompatible,
			SkipUnavailable:   request.SkipUnavailable,
		},
	)
	return importReportDTO(report), err
}

func (controller *InstancePackageController) SelectExportPath(
	suggestedName string,
) (string, error) {
	if controller.base.ctx == nil {
		return "", nil
	}
	if suggestedName == "" {
		suggestedName = "instance.waxlight"
	}
	if filepath.Ext(suggestedName) == "" {
		suggestedName += ".waxlight"
	}
	return wruntime.SaveFileDialog(
		controller.base.ctx,
		wruntime.SaveDialogOptions{
			Title:           "Export Waxlight instance",
			DefaultFilename: suggestedName,
			Filters: []wruntime.FileFilter{
				{
					DisplayName: "Waxlight packages (*.waxlight)",
					Pattern:     "*.waxlight",
				},
			},
		},
	)
}

func (controller *InstancePackageController) SelectPackageFile() (string, error) {
	if controller.base.ctx == nil {
		return "", nil
	}
	return wruntime.OpenFileDialog(
		controller.base.ctx,
		wruntime.OpenDialogOptions{
			Title: "Import a Waxlight instance",
			Filters: []wruntime.FileFilter{
				{
					DisplayName: "Waxlight packages (*.waxlight)",
					Pattern:     "*.waxlight",
				},
			},
		},
	)
}
