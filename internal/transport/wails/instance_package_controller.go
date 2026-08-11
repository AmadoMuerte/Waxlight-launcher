package wails

import (
	"path/filepath"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/waxlight/waxlight-launcher/internal/instances"
)

// InstancePackageController exposes instance package export, inspection, and
// import to the frontend. It stays limited to DTO conversion and feature
// invocation.
type InstancePackageController struct {
	svc       *instances.PackageService
	lifecycle lifecycle
}

func NewInstancePackageController(service *instances.PackageService, lifecycle lifecycle) *InstancePackageController {
	return &InstancePackageController{svc: service, lifecycle: lifecycle}
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
	var author *instances.PackageAuthor
	if request.Author != nil {
		author = &instances.PackageAuthor{
			Name:     request.Author.Name,
			Homepage: request.Author.Homepage,
			Source:   request.Author.Source,
		}
	}
	manifest, err := controller.svc.ExportInstance(
		controller.lifecycle.Context(),
		request.InstanceID,
		request.TargetPath,
		instances.ExportInstanceOptions{
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
	inspection, err := controller.svc.InspectPackage(controller.lifecycle.Context(), packagePath)
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
		controller.lifecycle.Context(),
		request.PackagePath,
		instances.ImportInstanceOptions{
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
	if suggestedName == "" {
		suggestedName = "instance.waxlight"
	}
	if filepath.Ext(suggestedName) == "" {
		suggestedName += ".waxlight"
	}
	return wruntime.SaveFileDialog(
		controller.lifecycle.Context(),
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
	return wruntime.OpenFileDialog(
		controller.lifecycle.Context(),
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
