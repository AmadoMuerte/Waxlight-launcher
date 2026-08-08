package presentation

import (
	"context"

	"github.com/waxlight/waxlight-launcher/internal/application"
)

type LastKnownGoodController struct {
	svc *application.Service
}

func NewLastKnownGoodController(service *application.Service) *LastKnownGoodController {
	return &LastKnownGoodController{svc: service}
}

// GetInstanceLastKnownGood returns the Last Known Good marker of an instance
// together with the comparison against its current configuration. A zero DTO
// with no error means no marker was recorded yet.
func (controller *LastKnownGoodController) GetInstanceLastKnownGood(
	instanceID string,
) (LastKnownGoodDTO, error) {
	status, err := controller.svc.GetLastKnownGoodStatus(
		context.Background(),
		instanceID,
	)
	return lastKnownGoodDTO(status), err
}
