package presentation

import (
	"context"
	"errors"
	"net/url"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/domain"
)

type LauncherUpdateDTO struct {
	InstalledVersion string `json:"installedVersion"`
	Version          string `json:"version"`
	Available        bool   `json:"available"`
	Downgrade        bool   `json:"downgrade"`
	Prerelease       bool   `json:"prerelease"`
	ReleaseNotes     string `json:"releaseNotes"`
	ReleasePageURL   string `json:"releasePageUrl"`
	AssetName        string `json:"assetName"`
	AssetSize        int64  `json:"assetSize"`
	InstallationMode string `json:"installationMode"`
}

type LauncherUpdateController struct {
	service *application.LauncherUpdateService
	base    *Base
}

func NewLauncherUpdateController(
	service *application.LauncherUpdateService,
	base *Base,
) *LauncherUpdateController {
	return &LauncherUpdateController{service: service, base: base}
}

func (controller *LauncherUpdateController) CurrentVersion() string {
	return controller.service.CurrentVersion()
}

func (controller *LauncherUpdateController) CheckUpdates(channel string) (LauncherUpdateDTO, error) {
	update, err := controller.service.Check(context.Background(), channel)
	return launcherUpdateDTO(update), err
}

func (controller *LauncherUpdateController) InstallUpdate(channel string) error {
	ctx := controller.base.ctx
	if ctx == nil {
		return errors.New("application runtime is unavailable")
	}
	err := controller.service.Install(ctx, channel, func(progress domain.LauncherUpdateProgress) {
		wruntime.EventsEmit(ctx, "updates:progress", progress)
	})
	if err != nil {
		return err
	}
	go func() {
		time.Sleep(250 * time.Millisecond)
		wruntime.Quit(ctx)
	}()
	return nil
}

func (controller *LauncherUpdateController) OpenReleasePage(channel string) error {
	update, err := controller.service.Check(context.Background(), channel)
	if err != nil {
		return err
	}
	if update.ReleasePageURL == "" {
		return errors.New("release page is unavailable")
	}
	return controller.openExternalURL(update.ReleasePageURL)
}

// OpenUrl opens an http(s) link in the user's default browser. Only well-formed
// web links are accepted; any other scheme or a missing host is rejected.
func (controller *LauncherUpdateController) OpenUrl(rawURL string) error {
	return controller.openExternalURL(rawURL)
}

func (controller *LauncherUpdateController) openExternalURL(rawURL string) error {
	if controller.base.ctx == nil {
		return errors.New("application runtime is unavailable")
	}
	if !validExternalURL(rawURL) {
		return domain.NewError(domain.ErrInvalidURL, "only http and https links can be opened")
	}
	wruntime.BrowserOpenURL(controller.base.ctx, rawURL)
	return nil
}

func validExternalURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return parsed.Host != ""
}

func launcherUpdateDTO(update domain.LauncherUpdate) LauncherUpdateDTO {
	return LauncherUpdateDTO{
		InstalledVersion: update.InstalledVersion,
		Version:          update.Version,
		Available:        update.Available,
		Downgrade:        update.Downgrade,
		Prerelease:       update.Prerelease,
		ReleaseNotes:     update.ReleaseNotes,
		ReleasePageURL:   update.ReleasePageURL,
		AssetName:        update.AssetName,
		AssetSize:        update.AssetSize,
		InstallationMode: update.InstallationMode,
	}
}
