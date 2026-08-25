package wails

import "github.com/AmadoMuerte/Waxlight-launcher/internal/version"

// AppController exposes launcher metadata to the frontend.
type AppController struct{}

func NewAppController() *AppController {
	return &AppController{}
}

// AppInfo returns launcher metadata needed by the application shell.
func (controller *AppController) AppInfo() map[string]any {
	return map[string]any{
		"name":       "Waxlight Launcher",
		"shortName":  "Waxlight",
		"version":    version.Version(),
		"unofficial": true,
	}
}
