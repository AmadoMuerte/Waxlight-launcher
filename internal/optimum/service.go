package optimum

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
)

var (
	ErrNotFound          = errors.New("optimum installation not found")
	ErrExecutableMissing = errors.New("optimum executable missing")
	ErrInvalidInstall    = errors.New("invalid optimum installation")
	ErrUnsupported       = errors.New("optimum is unsupported on this platform")
)

type Service struct {
	locator Locator
}

func NewService(locator Locator) *Service {
	return &Service{locator: locator}
}

func (service *Service) Status(configuredPath string) Status {
	installation, err := service.locate(configuredPath)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			slog.Debug("could not inspect Optimum installation", "error", err)
		}
		message := userMessage(err)
		if strings.TrimSpace(configuredPath) == "" && errors.Is(err, ErrNotFound) {
			message = "Optimum was not detected. Install it separately, then use Detect or Browse."
		}
		return Status{Path: strings.TrimSpace(configuredPath), Message: message}
	}
	return Status{
		Path: installation.Path, Executable: installation.Executable,
		GameVersion: installation.GameVersion, Ready: true,
	}
}

func (service *Service) Inspect(path string) (Status, error) {
	installation, err := service.locator.Inspect(strings.TrimSpace(path))
	if err != nil {
		return Status{Path: strings.TrimSpace(path), Message: userMessage(err)}, validationError(err)
	}
	return Status{
		Path: installation.Path, Executable: installation.Executable,
		GameVersion: installation.GameVersion, Ready: true,
	}, nil
}

func (service *Service) Resolve(configuredPath, vanillaDirectory string) (Installation, error) {
	installation, err := service.locate(configuredPath)
	if err != nil {
		return Installation{}, validationError(err)
	}
	vanillaVersion := service.locator.GameVersion(vanillaDirectory)
	if vanillaVersion != "" && installation.GameVersion != "" && vanillaVersion != installation.GameVersion {
		return Installation{}, errs.NewError(
			errs.ErrValidation,
			fmt.Sprintf(
				"This Optimum installation targets Vintage Story %s, but this instance uses Vintage Story %s. Install or configure a compatible Optimum version, or switch the instance to Vanilla.",
				installation.GameVersion,
				vanillaVersion,
			),
		)
	}
	inUse, err := service.locator.InUse(installation)
	if err != nil {
		return Installation{}, &errs.AppError{Code: errs.ErrFilePermission, Message: "Could not check whether Optimum is already running", Cause: err}
	}
	if inUse {
		return Installation{}, errs.NewError(errs.ErrValidation, "This Optimum installation is already being used by another running instance")
	}
	return installation, nil
}

func (service *Service) locate(configuredPath string) (Installation, error) {
	configuredPath = strings.TrimSpace(configuredPath)
	if configuredPath != "" {
		return service.locator.Inspect(configuredPath)
	}
	return service.locator.Detect()
}

func validationError(err error) error {
	return &errs.AppError{Code: errs.ErrValidation, Message: userMessage(err), Cause: err}
}

func userMessage(err error) string {
	switch {
	case errors.Is(err, ErrUnsupported):
		return "Optimum is not supported on this platform. Switch the instance to Vanilla."
	case errors.Is(err, ErrExecutableMissing):
		return "The selected Optimum installation does not contain the expected executable. Select the folder created by the official Optimum installer."
	case errors.Is(err, ErrInvalidInstall):
		return "The selected path is not a valid Optimum installation. Select the folder created by the official Optimum installer."
	default:
		return "Optimum is configured for this instance but its installation could not be found. Configure Optimum or switch this instance to Vanilla."
	}
}
