package wails

import "context"

// PresenceService is the port the controller uses to manage Discord Rich Presence.
type PresenceService interface {
	Connect(ctx context.Context)
	SetEnabled(ctx context.Context, enabled bool)
	Close()
}

// PresenceController exposes Discord Rich Presence control to the frontend.
type PresenceController struct {
	service   PresenceService
	lifecycle lifecycle
}

func NewPresenceController(service PresenceService, lifecycle lifecycle) *PresenceController {
	return &PresenceController{service: service, lifecycle: lifecycle}
}

// Connect establishes the Discord Rich Presence connection if enabled in settings.
func (controller *PresenceController) Connect() {
	controller.service.Connect(controller.lifecycle.Context())
}

// SetEnabled enables or disables Discord Rich Presence.
func (controller *PresenceController) SetEnabled(enabled bool) {
	controller.service.SetEnabled(controller.lifecycle.Context(), enabled)
}
