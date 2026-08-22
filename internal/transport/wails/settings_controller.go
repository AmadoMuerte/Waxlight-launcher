package wails

import (
	"errors"
	"os"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/optimum"
	"github.com/waxlight/waxlight-launcher/internal/settings"
)

const optimumInstallationGuideURL = "https://github.com/Zaldaryon/Optimum/wiki/Installation"

type settingsDialogs interface {
	SelectDataFolder() (string, error)
	SelectGameArchive() (string, error)
	SelectGameDirectory() (string, error)
	SelectOptimumInstallation() (string, error)
	SelectModFile() (string, error)
	SelectModFiles() ([]string, error)
}

type directoryOpener interface {
	OpenDirectory(string) error
}

// SettingsController exposes launcher settings, data-folder relocation, and
// native file dialogs to the frontend. It stays limited to DTO conversion and
// feature invocation.
type SettingsController struct {
	reader    *settings.Reader
	service   *settings.Service
	dataRoot  *settings.DataRootService
	optimum   *optimum.Service
	lifecycle lifecycle
	dialogs   settingsDialogs
	opener    directoryOpener
}

func NewSettingsController(
	reader *settings.Reader,
	service *settings.Service,
	dataRoot *settings.DataRootService,
	optimumService *optimum.Service,
	lifecycle lifecycle,
	dialogs settingsDialogs,
	opener directoryOpener,
) *SettingsController {
	return &SettingsController{reader: reader, service: service, dataRoot: dataRoot, optimum: optimumService, lifecycle: lifecycle, dialogs: dialogs, opener: opener}
}

func (controller *SettingsController) GetSettings() (SettingsDTO, error) {
	value, err := controller.reader.Get(controller.lifecycle.Context())
	return settingsDTO(value), err
}

func (controller *SettingsController) UpdateSettings(request SettingsDTO) (SettingsDTO, error) {
	value, err := controller.service.Update(controller.lifecycle.Context(), settings.Settings{
		Language: request.Language, DownloadsParallel: request.DownloadsParallel,
		ConfirmDeletion: request.ConfirmDeletion, GlobalLaunchArguments: request.GlobalLaunchArguments,
		OptimumPath:     request.OptimumPath,
		CheckForUpdates: request.CheckForUpdates, UpdateChannel: request.UpdateChannel,
		SkippedUpdateVersion: request.SkippedUpdateVersion, TelemetryEnabled: request.TelemetryEnabled,
		AutomaticSafetySnapshots: request.AutomaticSafetySnapshots,
		LibrarySort:              request.LibrarySort,
	})
	return settingsDTO(value), err
}

func (controller *SettingsController) SetLibrarySort(value string) (SettingsDTO, error) {
	updated, err := controller.service.SetLibrarySort(controller.lifecycle.Context(), value)
	return settingsDTO(updated), err
}

func (controller *SettingsController) GetDataFolder() (DataFolderDTO, error) {
	value, err := controller.dataRoot.Get()
	return DataFolderDTO(value), err
}

func (controller *SettingsController) SelectDataFolder() (string, error) {
	return controller.dialogs.SelectDataFolder()
}

func (controller *SettingsController) MoveDataFolder(target string) error {
	return controller.dataRoot.Move(controller.lifecycle.Context(), target)
}

func (controller *SettingsController) SelectGameArchive() (string, error) {
	return controller.dialogs.SelectGameArchive()
}

func (controller *SettingsController) SelectGameDirectory() (string, error) {
	return controller.dialogs.SelectGameDirectory()
}

func (controller *SettingsController) GetOptimumStatus() (OptimumStatusDTO, error) {
	settings, err := controller.reader.Get(controller.lifecycle.Context())
	if err != nil {
		return OptimumStatusDTO{}, err
	}
	return optimumStatusDTO(controller.optimum.Status(settings.OptimumPath)), nil
}

func (controller *SettingsController) DetectOptimum() OptimumStatusDTO {
	return optimumStatusDTO(controller.optimum.Status(""))
}

func (controller *SettingsController) InspectOptimum(path string) (OptimumStatusDTO, error) {
	status, err := controller.optimum.Inspect(path)
	return optimumStatusDTO(status), err
}

func (controller *SettingsController) SelectOptimumInstallation() (string, error) {
	return controller.dialogs.SelectOptimumInstallation()
}

func (controller *SettingsController) OpenOptimumInstallationGuide() {
	wruntime.BrowserOpenURL(controller.lifecycle.Context(), optimumInstallationGuideURL)
}

func (controller *SettingsController) SelectModFile() (string, error) {
	return controller.dialogs.SelectModFile()
}

func (controller *SettingsController) SelectModFiles() ([]string, error) {
	return controller.dialogs.SelectModFiles()
}

func (controller *SettingsController) OpenDirectory(path string) error {
	if err := controller.opener.OpenDirectory(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errs.NewError(errs.ErrValidation, "Directory not found")
		}
		return &errs.AppError{Code: errs.ErrFilePermission, Message: "Could not open the directory", Cause: err}
	}
	return nil
}
