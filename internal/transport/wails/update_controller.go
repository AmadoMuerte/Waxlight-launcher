package wails

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/events"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/updates"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// LauncherUpdateDTO reports release metadata and whether a launcher update is available.
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

// LauncherUpdateController exposes launcher update checks and installation to
// the frontend. It stays limited to DTO conversion and feature invocation.
type LauncherUpdateController struct {
	service   *updates.Service
	lifecycle lifecycle
	events    events.Publisher
}

func NewLauncherUpdateController(
	service *updates.Service,
	lifecycle lifecycle,
	eventPublisher events.Publisher,
) *LauncherUpdateController {
	return &LauncherUpdateController{service: service, lifecycle: lifecycle, events: eventPublisher}
}

// CurrentVersion returns the running launcher version for display and update comparison.
func (controller *LauncherUpdateController) CurrentVersion() string {
	return controller.service.CurrentVersion()
}

// CheckUpdates checks the selected release channel for a newer launcher version.
func (controller *LauncherUpdateController) CheckUpdates(channel string) (LauncherUpdateDTO, error) {
	update, err := controller.service.Check(controller.lifecycle.Context(), channel)
	return launcherUpdateDTO(update), err
}

// InstallUpdate downloads and applies the newest update from the selected channel.
func (controller *LauncherUpdateController) InstallUpdate(channel string) error {
	ctx := controller.lifecycle.Context()
	err := controller.service.Install(ctx, channel, func(progress updates.Progress) {
		controller.events.Publish("updates:progress", progress)
	})
	if err != nil {
		return err
	}
	controller.lifecycle.Go(func(workerCtx context.Context) {
		select {
		case <-time.After(250 * time.Millisecond):
			wruntime.Quit(workerCtx)
		case <-workerCtx.Done():
		}
	})
	return nil
}

// OpenReleasePage opens the selected update channel's release page.
func (controller *LauncherUpdateController) OpenReleasePage(channel string) error {
	update, err := controller.service.Check(controller.lifecycle.Context(), channel)
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
	if !validExternalURL(rawURL) {
		return errs.NewError(errs.ErrInvalidURL, "only http and https links can be opened")
	}
	wruntime.BrowserOpenURL(controller.lifecycle.Context(), rawURL)
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

func launcherUpdateDTO(update updates.Update) LauncherUpdateDTO {
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
