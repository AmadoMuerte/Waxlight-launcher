package wails

import (
	"github.com/AmadoMuerte/Waxlight-launcher/internal/recovery"
)

// LastKnownGoodController exposes the Last Known Good marker of instances to
// the frontend. It stays limited to DTO conversion and feature invocation.
type LastKnownGoodController struct {
	svc       *recovery.Service
	lifecycle lifecycle
}

func NewLastKnownGoodController(service *recovery.Service, lifecycle lifecycle) *LastKnownGoodController {
	return &LastKnownGoodController{svc: service, lifecycle: lifecycle}
}

// GetInstanceLastKnownGood returns the Last Known Good marker of an instance
// together with the comparison against its current configuration. A zero DTO
// with no error means no marker was recorded yet.
func (controller *LastKnownGoodController) GetInstanceLastKnownGood(
	instanceID string,
) (LastKnownGoodDTO, error) {
	status, err := controller.svc.Status(
		controller.lifecycle.Context(),
		instanceID,
	)
	return lastKnownGoodDTO(status), err
}
