package presentation

import (
	"github.com/waxlight/waxlight-launcher/internal/app"
	"github.com/waxlight/waxlight-launcher/internal/application"
)

type LastKnownGoodController struct {
	svc       *application.Service
	lifecycle *app.Lifecycle
}

func NewLastKnownGoodController(service *application.Service, lifecycle *app.Lifecycle) *LastKnownGoodController {
	return &LastKnownGoodController{svc: service, lifecycle: lifecycle}
}

// GetInstanceLastKnownGood returns the Last Known Good marker of an instance
// together with the comparison against its current configuration. A zero DTO
// with no error means no marker was recorded yet.
func (controller *LastKnownGoodController) GetInstanceLastKnownGood(
	instanceID string,
) (LastKnownGoodDTO, error) {
	status, err := controller.svc.GetLastKnownGoodStatus(
		controller.lifecycle.Context(),
		instanceID,
	)
	return lastKnownGoodDTO(status), err
}
