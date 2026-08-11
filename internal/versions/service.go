package versions

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

const (
	titleInstalling  = "operation_installing_game_version"
	titleDownloading = "operation_downloading_game_version"
)

type QueryService struct {
	repository     Repository
	catalog        Catalog
	localInstaller LocalInstaller
	filesystem     Filesystem
	now            func() time.Time
}

func NewQueryService(
	repository Repository,
	catalog Catalog,
	localInstaller LocalInstaller,
	filesystem Filesystem,
	now func() time.Time,
) *QueryService {
	return &QueryService{repository: repository, catalog: catalog, localInstaller: localInstaller, filesystem: filesystem, now: now}
}

func (service *QueryService) Get(ctx context.Context, id string) (GameVersion, error) {
	return service.repository.GetVersion(ctx, id)
}

func (service *QueryService) ResolveExecutable(ctx context.Context, id string) (GameVersion, error) {
	version, err := service.repository.GetVersion(ctx, id)
	if err != nil {
		return GameVersion{}, err
	}
	return service.repair(ctx, version)
}

func (service *QueryService) List(ctx context.Context) ([]GameVersion, error) {
	installed, err := service.repository.ListVersions(ctx)
	if err != nil {
		return nil, err
	}
	for index := range installed {
		if repaired, repairErr := service.repair(ctx, installed[index]); repairErr == nil {
			installed[index] = repaired
		}
	}
	return installed, nil
}

func (service *QueryService) repair(ctx context.Context, version GameVersion) (GameVersion, error) {
	if service.filesystem.ExecutableExists(version.ExecutablePath) {
		return version, nil
	}
	executable, err := service.localInstaller.FindExecutable(version.InstallationDir, "")
	if err != nil {
		return version, err
	}
	if err := service.filesystem.MakeExecutable(executable); err != nil {
		return version, err
	}
	version.ExecutablePath = executable
	version.Status = "installed"
	now := service.now().UTC()
	version.VerifiedAt = &now
	if err := service.repository.UpdateVersion(ctx, version); err != nil {
		return version, err
	}
	return version, nil
}

func (service *QueryService) ListAvailable(ctx context.Context) ([]AvailableGameVersion, error) {
	if service.catalog == nil {
		return nil, domain.NewError(domain.ErrVersionCatalog, "The game version catalog is not configured")
	}
	available, err := service.catalog.List(ctx)
	if err != nil {
		return nil, &domain.AppError{Code: domain.ErrVersionCatalog, Message: "Could not load the official game version catalog", Retryable: true, Cause: err}
	}
	installed, err := service.repository.ListVersions(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]GameVersion, len(installed))
	for _, version := range installed {
		byID[version.ID] = version
	}
	for index := range available {
		if version, ok := byID[available[index].ID]; ok {
			available[index].Installed = true
			status := version.Status
			available[index].InstallStatus = &status
		}
	}
	return available, nil
}

func validateID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", domain.NewError(domain.ErrValidation, "Enter a version ID")
	}
	if len([]byte(id)) > 180 {
		return "", domain.NewError(domain.ErrValidation, "Version ID cannot exceed 180 bytes")
	}
	for _, char := range id {
		if unicode.IsControl(char) {
			return "", domain.NewError(domain.ErrValidation, "Version ID cannot contain control characters")
		}
	}
	return id, nil
}

func validateCatalogFilename(filename string) error {
	if filename == "" || filename == "." || filepath.IsAbs(filename) || filepath.Base(filename) != filename || strings.ContainsAny(filename, `/\\`) {
		return domain.NewError(domain.ErrVersionCatalog, "The game version catalog contains an invalid package filename")
	}
	return nil
}

func isCode(err error, code string) bool {
	var appError *domain.AppError
	return errors.As(err, &appError) && appError.Code == code
}

func titleParams(name string) map[string]string { return map[string]string{"name": name} }

func trim(value string) string { return strings.TrimSpace(value) }
