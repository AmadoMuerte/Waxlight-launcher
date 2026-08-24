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

func (controller *OperationController) ListOperations() ([]OperationDTO, error) {
	tracked, err := controller.operations.List(controller.lifecycle.Context())
	result := make([]OperationDTO, 0, len(tracked))
	for _, operation := range tracked {
		result = append(result, operationDTO(operation))
	}
	return result, err
}

func (controller *OperationController) CancelOperation(id string) error {
	return controller.operations.Cancel(id)
}

func (controller *OperationController) DeleteOperation(id string) error {
	return controller.operations.Delete(controller.lifecycle.Context(), id)
}

func (controller *OperationController) ClearOperationHistory() (int64, error) {
	return controller.operations.Clear(controller.lifecycle.Context())
}
