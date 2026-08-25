package wails

import (
	"path/filepath"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
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

// ExportInstanceRequest selects an instance, destination, and package metadata for export.
type ExportInstanceRequest struct {
	InstanceID  string `json:"instanceId"`
	TargetPath  string `json:"targetPath"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ExportInstance writes a portable package containing sanitized instance data and metadata.
func (controller *InstancePackageController) ExportInstance(
	request ExportInstanceRequest,
) (PackageManifestDTO, error) {
	manifest, err := controller.svc.ExportInstance(
		controller.lifecycle.Context(),
		request.InstanceID,
		request.TargetPath,
		instances.ExportInstanceOptions{
			Name:        request.Name,
			Description: request.Description,
		},
	)
	return packageManifestDTO(manifest), err
}

// InspectPackage validates a package and reports its contents before import.
func (controller *InstancePackageController) InspectPackage(
	packagePath string,
) (PackageInspectionDTO, error) {
	inspection, err := controller.svc.InspectPackage(controller.lifecycle.Context(), packagePath)
	return packageInspectionDTO(inspection), err
}

// ImportInstanceRequest selects an instance package and conflict-handling options for import.
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

// ImportPackage creates a managed instance from a validated package.
func (controller *InstancePackageController) ImportPackage(
	request ImportInstanceRequest,
) (OperationDTO, error) {
	operation, err := controller.svc.StartImport(
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
	return operationDTO(operation), err
}

// SelectExportPath prompts for the destination of an exported instance package.
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

// SelectPackageFile prompts for an instance package to inspect or import.
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
