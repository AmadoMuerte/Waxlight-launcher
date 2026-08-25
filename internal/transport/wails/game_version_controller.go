package wails

import (
	"context"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/operations"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/versions"
)

type gameVersionCapabilities interface {
	List(context.Context) ([]versions.GameVersion, error)
	ListAvailable(context.Context) ([]versions.AvailableGameVersion, error)
	InstallCatalog(context.Context, string) (versions.Install, error)
	InstallLocal(context.Context, string, string, string, string, string) (operations.Operation, error)
	Remove(context.Context, string, bool) error
}

// GameVersionController exposes installed and available game versions and
// drives installation and removal. It stays limited to DTO conversion and
// feature invocation.
type GameVersionController struct {
	svc       gameVersionCapabilities
	lifecycle lifecycle
}

func NewGameVersionController(service gameVersionCapabilities, lifecycle lifecycle) *GameVersionController {
	return &GameVersionController{svc: service, lifecycle: lifecycle}
}

// InstallVersionRequest selects a game release and installation options.
type InstallVersionRequest struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	SourcePath             string `json:"sourcePath"`
	ExecutableRelativePath string `json:"executableRelativePath"`
	ExpectedSHA256         string `json:"expectedSha256"`
}

// ListInstalledVersions returns game versions installed and managed by the launcher.
func (controller *GameVersionController) ListInstalledVersions() (
	[]GameVersionDTO,
	error,
) {
	versions, err := controller.svc.List(controller.lifecycle.Context())
	result := make([]GameVersionDTO, 0, len(versions))
	for _, version := range versions {
		result = append(result, versionDTO(version))
	}
	return result, err
}

// ListAvailableVersions returns downloadable releases compatible with the current platform.
func (controller *GameVersionController) ListAvailableVersions() (
	[]AvailableGameVersionDTO,
	error,
) {
	versions, err := controller.svc.ListAvailable(controller.lifecycle.Context())
	result := make([]AvailableGameVersionDTO, 0, len(versions))
	for _, version := range versions {
		result = append(result, availableVersionDTO(version))
	}
	return result, err
}

// InstallVersion starts downloading and installing a selected game release.
func (controller *GameVersionController) InstallVersion(
	versionID string,
) (OperationDTO, error) {
	install, err := controller.svc.InstallCatalog(
		controller.lifecycle.Context(),
		versionID,
	)
	return operationDTO(install.Operation), err
}

// InstallLocalVersion imports a game installation from a local archive or directory.
func (controller *GameVersionController) InstallLocalVersion(
	request InstallVersionRequest,
) (OperationDTO, error) {
	operation, err := controller.svc.InstallLocal(
		controller.lifecycle.Context(),
		request.ID,
		request.Name,
		request.SourcePath,
		request.ExecutableRelativePath,
		request.ExpectedSHA256,
	)
	return operationDTO(operation), err
}

// RemoveVersion removes an installed game version when no managed instance requires it.
func (controller *GameVersionController) RemoveVersion(
	id string,
	deleteFiles bool,
) error {
	return controller.svc.Remove(controller.lifecycle.Context(), id, deleteFiles)
}
