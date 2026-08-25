package wails

import (
	"context"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/supportreports"
)

type supportReportService interface {
	Preview(context.Context, string, string) (supportreports.Preview, error)
	Submit(context.Context, string) (supportreports.Result, error)
}

// SupportReportController exposes sanitized support-report review and submission to the frontend.
type SupportReportController struct {
	service   supportReportService
	lifecycle lifecycle
}

func NewSupportReportController(service supportReportService, lifecycle lifecycle) *SupportReportController {
	return &SupportReportController{service: service, lifecycle: lifecycle}
}

// Preview creates a sanitized support-report snapshot for user review before submission.
func (controller *SupportReportController) Preview(description, instanceID string) (supportreports.Preview, error) {
	return controller.service.Preview(controller.lifecycle.Context(), description, instanceID)
}

// Submit uploads a previously reviewed support-report snapshot.
func (controller *SupportReportController) Submit(snapshotID string) (supportreports.Result, error) {
	return controller.service.Submit(controller.lifecycle.Context(), snapshotID)
}
