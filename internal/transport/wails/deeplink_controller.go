package wails

import "github.com/AmadoMuerte/Waxlight-launcher/internal/platform/deeplink"

type deepLinkConsumer interface {
	Consume() []deeplink.Target
}

// DeepLinkController exposes cold-start deep links after the frontend subscribes.
type DeepLinkController struct {
	links deepLinkConsumer
}

func NewDeepLinkController(links deepLinkConsumer) *DeepLinkController {
	return &DeepLinkController{links: links}
}

// ConsumePendingDeepLinks returns queued external navigation targets and clears the queue.
func (controller *DeepLinkController) ConsumePendingDeepLinks() []deeplink.Target {
	return controller.links.Consume()
}
