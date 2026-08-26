package wails

import (
	"log/slog"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/launching"
)

// LaunchController exposes launch validation, game start/stop, and running
// instance queries to the frontend. It stays limited to DTO conversion and
// feature invocation.
type LaunchController struct {
	svc       *launching.Coordinator
	lifecycle lifecycle
}

func NewLaunchController(service *launching.Coordinator, lifecycle lifecycle) *LaunchController {
	return &LaunchController{svc: service, lifecycle: lifecycle}
}

// LaunchRequest selects the instance and optional account for a game launch.
type LaunchRequest struct {
	// InstanceID is the managed instance to launch.
	InstanceID string `json:"instanceId"`
	// AccountID overrides the instance default account for this launch.
	AccountID *string `json:"accountId,omitempty"`
}

// ServerLaunchRequest selects the instance, account, and server address for joining multiplayer.
type ServerLaunchRequest struct {
	// InstanceID is the managed instance to launch.
	InstanceID string `json:"instanceId"`
	// AccountID overrides the instance default account for this launch.
	AccountID *string `json:"accountId,omitempty"`
	// Address is the server address to join after the game starts.
	Address string `json:"address"`
}

// LaunchValidationDTO reports launch blockers and warnings before starting the game.
type LaunchValidationDTO struct {
	// Valid reports whether the instance can start without blocking issues.
	Valid bool `json:"valid"`
	// Issues are blocking problems that prevent the launch.
	Issues []string `json:"issues"`
	// Warnings are non-blocking conditions the user should be aware of.
	Warnings []string `json:"warnings"`
}

// ValidateLaunch checks whether an instance can start and reports blocking problems.
func (controller *LaunchController) ValidateLaunch(
	request LaunchRequest,
) (LaunchValidationDTO, error) {
	validation, err := controller.svc.ValidateLaunch(
		controller.lifecycle.Context(),
		request.InstanceID,
		request.AccountID,
	)
	return LaunchValidationDTO{
		Valid:    validation.Valid,
		Issues:   nonNilStrings(validation.Issues),
		Warnings: nonNilStrings(validation.Warnings),
	}, err
}

// LaunchInstance validates the instance, starts the game process, and returns
// after its play session is persisted; it does not wait for the process to exit.
//
// Errors:
//   - validation_error: launch validation reported blocking issues
//   - instance_not_found: the requested instance does not exist
//   - game_version_not_found: the instance game version is not installed
//   - account_not_found: the selected account does not exist
//   - client_settings_error: game credentials could not be prepared
//   - process_start_failed: the game process could not be started
//   - snapshot_in_progress: a snapshot is currently being taken
//   - data_folder_busy: a data-folder relocation is in progress
func (controller *LaunchController) LaunchInstance(
	request LaunchRequest,
) (PlaySessionDTO, error) {
	session, err := controller.svc.Launch(
		controller.lifecycle.Context(),
		request.InstanceID,
		request.AccountID,
	)
	if err != nil {
		slog.Warn("launch request failed", "error", err)
		return PlaySessionDTO{}, err
	}
	return sessionDTO(session), nil
}

// LaunchServer starts the dedicated server for an instance with the requested arguments.
func (controller *LaunchController) LaunchServer(
	request ServerLaunchRequest,
) (PlaySessionDTO, error) {
	session, err := controller.svc.LaunchServer(
		controller.lifecycle.Context(),
		request.InstanceID,
		request.AccountID,
		request.Address,
	)
	if err != nil {
		slog.Warn("server launch request failed", "error", err)
		return PlaySessionDTO{}, err
	}
	return sessionDTO(session), nil
}

// StopInstance requests a graceful stop of a running game or server process.
func (controller *LaunchController) StopInstance(id string) error {
	err := controller.svc.Stop(controller.lifecycle.Context(), id, false)
	if err != nil {
		slog.Warn("stop request failed", "error", err)
	}
	return err
}

// ForceStopInstance terminates a running game or server process immediately.
func (controller *LaunchController) ForceStopInstance(id string) error {
	err := controller.svc.Stop(controller.lifecycle.Context(), id, true)
	if err != nil {
		slog.Warn("force stop request failed", "error", err)
	}
	return err
}

// GetRunningInstances returns identifiers for instances with active game or server processes.
func (controller *LaunchController) GetRunningInstances() []string {
	return controller.svc.RunningInstanceIDs()
}
