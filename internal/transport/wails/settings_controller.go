package wails

import (
	"errors"
	"os"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/optimum"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/settings"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
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

// GetSettings returns the launcher's current user-configurable settings.
func (controller *SettingsController) GetSettings() (SettingsDTO, error) {
	value, err := controller.reader.Get(controller.lifecycle.Context())
	return settingsDTO(value), err
}

// UpdateSettings validates and persists the complete settings document.
func (controller *SettingsController) UpdateSettings(request SettingsDTO) (SettingsDTO, error) {
	value, err := controller.service.Update(controller.lifecycle.Context(), settings.Settings{
		Language: request.Language, DownloadsParallel: request.DownloadsParallel,
		ConfirmDeletion: request.ConfirmDeletion, GlobalLaunchArguments: request.GlobalLaunchArguments,
		OptimumPath:     request.OptimumPath,
		CheckForUpdates: request.CheckForUpdates, UpdateChannel: request.UpdateChannel,
		SkippedUpdateVersion: request.SkippedUpdateVersion, TelemetryEnabled: request.TelemetryEnabled,
		RichPresenceEnabled:        request.RichPresenceEnabled,
		AutomaticSafetySnapshots:   request.AutomaticSafetySnapshots,
		AutomaticSnapshotRetention: request.AutomaticSnapshotRetention,
		LibrarySort:                request.LibrarySort,
		UIScale:                    request.UIScale,
	})
	return settingsDTO(value), err
}

// SetLibrarySort updates the ordering used by the instance library.
func (controller *SettingsController) SetLibrarySort(value string) (SettingsDTO, error) {
	updated, err := controller.service.SetLibrarySort(controller.lifecycle.Context(), value)
	return settingsDTO(updated), err
}

// GetDataFolder reports the active launcher data directory and its storage usage.
func (controller *SettingsController) GetDataFolder() (DataFolderDTO, error) {
	value, err := controller.dataRoot.Get()
	return DataFolderDTO(value), err
}

// SelectDataFolder prompts for a launcher data directory.
func (controller *SettingsController) SelectDataFolder() (string, error) {
	return controller.dialogs.SelectDataFolder()
}

// MoveDataFolder validates and prepares the target, then starts relocation in
// the background. A nil return means the move was accepted, not completed;
// progress and failure are reported through data-folder events, and success
// relaunches the launcher.
//
// Errors:
//   - data_folder_busy: another relocation or background work is active
//   - instance_already_running: a game instance is currently running
func (controller *SettingsController) MoveDataFolder(target string) error {
	return controller.dataRoot.Move(controller.lifecycle.Context(), target)
}

// ValidateDataFolderTarget checks whether a target can be used as the launcher
// data folder, including write access, without starting a relocation. It lets
// the frontend warn the user before the move is confirmed.
//
// Errors:
//   - file_permission_denied: the launcher cannot create or write in the target
func (controller *SettingsController) ValidateDataFolderTarget(target string) error {
	return controller.dataRoot.Check(controller.lifecycle.Context(), target)
}

// SelectGameArchive prompts for a local game archive.
func (controller *SettingsController) SelectGameArchive() (string, error) {
	return controller.dialogs.SelectGameArchive()
}

// SelectGameDirectory prompts for an existing game installation directory.
func (controller *SettingsController) SelectGameDirectory() (string, error) {
	return controller.dialogs.SelectGameDirectory()
}

// GetOptimumStatus reports the configured Optimum runtime and whether it is usable.
func (controller *SettingsController) GetOptimumStatus() (OptimumStatusDTO, error) {
	settings, err := controller.reader.Get(controller.lifecycle.Context())
	if err != nil {
		return OptimumStatusDTO{}, err
	}
	return optimumStatusDTO(controller.optimum.Status(settings.OptimumPath)), nil
}

// DetectOptimum searches standard locations for a usable Optimum installation.
func (controller *SettingsController) DetectOptimum() OptimumStatusDTO {
	return optimumStatusDTO(controller.optimum.Status(""))
}

// InspectOptimum validates a selected Optimum installation directory.
func (controller *SettingsController) InspectOptimum(path string) (OptimumStatusDTO, error) {
	status, err := controller.optimum.Inspect(path)
	return optimumStatusDTO(status), err
}

// SelectOptimumInstallation prompts for an Optimum installation to inspect.
func (controller *SettingsController) SelectOptimumInstallation() (string, error) {
	return controller.dialogs.SelectOptimumInstallation()
}

// OpenOptimumInstallationGuide opens installation guidance for the optional Optimum runtime.
func (controller *SettingsController) OpenOptimumInstallationGuide() {
	wruntime.BrowserOpenURL(controller.lifecycle.Context(), optimumInstallationGuideURL)
}

// SelectModFile prompts for one local mod archive.
func (controller *SettingsController) SelectModFile() (string, error) {
	return controller.dialogs.SelectModFile()
}

// SelectModFiles prompts for multiple local mod archives.
func (controller *SettingsController) SelectModFiles() ([]string, error) {
	return controller.dialogs.SelectModFiles()
}

// OpenDirectory opens an approved local directory in the system file manager.
func (controller *SettingsController) OpenDirectory(path string) error {
	if err := controller.opener.OpenDirectory(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errs.NewError(errs.ErrValidation, "Directory not found")
		}
		return &errs.AppError{Code: errs.ErrFilePermission, Message: "Could not open the directory", Cause: err}
	}
	return nil
}
