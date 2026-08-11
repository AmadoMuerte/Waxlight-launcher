package wails

import (
	"errors"
	"os"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/settings"
)

type settingsDialogs interface {
	SelectDataFolder() (string, error)
	SelectGameArchive() (string, error)
	SelectGameDirectory() (string, error)
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
	lifecycle lifecycle
	dialogs   settingsDialogs
	opener    directoryOpener
}

func NewSettingsController(
	reader *settings.Reader,
	service *settings.Service,
	dataRoot *settings.DataRootService,
	lifecycle lifecycle,
	dialogs settingsDialogs,
	opener directoryOpener,
) *SettingsController {
	return &SettingsController{reader: reader, service: service, dataRoot: dataRoot, lifecycle: lifecycle, dialogs: dialogs, opener: opener}
}

func (controller *SettingsController) GetSettings() (SettingsDTO, error) {
	value, err := controller.reader.Get(controller.lifecycle.Context())
	return settingsDTO(value), err
}

func (controller *SettingsController) UpdateSettings(request SettingsDTO) (SettingsDTO, error) {
	value, err := controller.service.Update(controller.lifecycle.Context(), settings.Settings{
		Language: request.Language, DownloadsParallel: request.DownloadsParallel,
		ConfirmDeletion: request.ConfirmDeletion, GlobalLaunchArguments: request.GlobalLaunchArguments,
		CheckForUpdates: request.CheckForUpdates, UpdateChannel: request.UpdateChannel,
		SkippedUpdateVersion: request.SkippedUpdateVersion, TelemetryEnabled: request.TelemetryEnabled,
		AutomaticSafetySnapshots: request.AutomaticSafetySnapshots,
	})
	return settingsDTO(value), err
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

func (controller *SettingsController) SelectModFile() (string, error) {
	return controller.dialogs.SelectModFile()
}

func (controller *SettingsController) SelectModFiles() ([]string, error) {
	return controller.dialogs.SelectModFiles()
}

func (controller *SettingsController) OpenDirectory(path string) error {
	if err := controller.opener.OpenDirectory(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.NewError(domain.ErrValidation, "Directory not found")
		}
		return &domain.AppError{Code: domain.ErrFilePermission, Message: "Could not open the directory", Cause: err}
	}
	return nil
}
