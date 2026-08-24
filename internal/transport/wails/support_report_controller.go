package wails

import (
	"context"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/supportreports"
)

type supportReportService interface {
	Preview(context.Context, string, string) (supportreports.Preview, error)
	Submit(context.Context, string) (supportreports.Result, error)
}

type SupportReportController struct {
	service   supportReportService
	lifecycle lifecycle
}

func NewSupportReportController(service supportReportService, lifecycle lifecycle) *SupportReportController {
	return &SupportReportController{service: service, lifecycle: lifecycle}
}

func (controller *SupportReportController) Preview(description, instanceID string) (supportreports.Preview, error) {
	return controller.service.Preview(controller.lifecycle.Context(), description, instanceID)
}

func (controller *SupportReportController) Submit(snapshotID string) (supportreports.Result, error) {
	return controller.service.Submit(controller.lifecycle.Context(), snapshotID)
}
