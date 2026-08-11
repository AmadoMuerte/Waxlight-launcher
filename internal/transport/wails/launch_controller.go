package wails

import (
	"log/slog"

	"github.com/waxlight/waxlight-launcher/internal/launching"
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

type LaunchRequest struct {
	InstanceID string  `json:"instanceId"`
	AccountID  *string `json:"accountId,omitempty"`
}

type ServerLaunchRequest struct {
	InstanceID string  `json:"instanceId"`
	AccountID  *string `json:"accountId,omitempty"`
	Address    string  `json:"address"`
}

type LaunchValidationDTO struct {
	Valid    bool     `json:"valid"`
	Issues   []string `json:"issues"`
	Warnings []string `json:"warnings"`
}

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

func (controller *LaunchController) StopInstance(id string) error {
	err := controller.svc.Stop(controller.lifecycle.Context(), id, false)
	if err != nil {
		slog.Warn("stop request failed", "error", err)
	}
	return err
}

func (controller *LaunchController) ForceStopInstance(id string) error {
	err := controller.svc.Stop(controller.lifecycle.Context(), id, true)
	if err != nil {
		slog.Warn("force stop request failed", "error", err)
	}
	return err
}

func (controller *LaunchController) GetRunningInstances() []string {
	return controller.svc.RunningInstanceIDs()
}
