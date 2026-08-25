package wails

import (
	"github.com/AmadoMuerte/Waxlight-launcher/internal/operations"
)

// OperationController exposes the persistent operation list to the frontend.
// It stays limited to DTO conversion and feature invocation.
type OperationController struct {
	operations *operations.Manager
	lifecycle  lifecycle
}

func NewOperationController(manager *operations.Manager, lifecycle lifecycle) *OperationController {
	return &OperationController{operations: manager, lifecycle: lifecycle}
}

// ListOperations returns tracked background operations for progress and history views.
func (controller *OperationController) ListOperations() ([]OperationDTO, error) {
	tracked, err := controller.operations.List(controller.lifecycle.Context())
	result := make([]OperationDTO, 0, len(tracked))
	for _, operation := range tracked {
		result = append(result, operationDTO(operation))
	}
	return result, err
}

// CancelOperation requests cancellation of a running background operation.
func (controller *OperationController) CancelOperation(id string) error {
	return controller.operations.Cancel(id)
}

// DeleteOperation removes a completed operation from history.
func (controller *OperationController) DeleteOperation(id string) error {
	return controller.operations.Delete(controller.lifecycle.Context(), id)
}

// ClearOperationHistory removes completed operation records and returns the count removed.
func (controller *OperationController) ClearOperationHistory() (int64, error) {
	return controller.operations.Clear(controller.lifecycle.Context())
}
