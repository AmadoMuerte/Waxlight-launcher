package versions

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
)

const (
	titleInstalling        = "operation_installing_game_version"
	titleInstallingWindows = "operation_installing_windows_game_version"
	titleDownloading       = "operation_downloading_game_version"
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
	version, installed, err := service.verify(ctx, version)
	if err != nil {
		return GameVersion{}, err
	}
	if !installed {
		return GameVersion{}, errs.NewError(errs.ErrVersionNotFound, "Game version executable not found")
	}
	return version, nil
}

func (service *QueryService) List(ctx context.Context) ([]GameVersion, error) {
	installed, err := service.repository.ListVersions(ctx)
	if err != nil {
		return nil, err
	}
	for index := range installed {
		installed[index], _, err = service.verify(ctx, installed[index])
		if err != nil {
			return nil, err
		}
	}
	return installed, nil
}

func (service *QueryService) verify(ctx context.Context, version GameVersion) (GameVersion, bool, error) {
	return verifyVersion(ctx, service.repository, service.localInstaller, service.filesystem, service.now, version)
}

func verifyVersion(
	ctx context.Context,
	repository Repository,
	localInstaller LocalInstaller,
	filesystem Filesystem,
	now func() time.Time,
	version GameVersion,
) (GameVersion, bool, error) {
	if filesystem.ExecutableExists(version.ExecutablePath) {
		if version.Status != "installed" {
			version.Status = "installed"
			verifiedAt := now().UTC()
			version.VerifiedAt = &verifiedAt
			if err := repository.UpdateVersion(ctx, version); err != nil {
				return version, false, err
			}
		}
		return version, true, nil
	}
	executable, err := localInstaller.FindExecutable(version.InstallationDir, "")
	if err != nil {
		return markVersionMissing(ctx, repository, now, version)
	}
	if err := filesystem.MakeExecutable(executable); err != nil {
		return markVersionMissing(ctx, repository, now, version)
	}
	version.ExecutablePath = executable
	version.Status = "installed"
	verifiedAt := now().UTC()
	version.VerifiedAt = &verifiedAt
	if err := repository.UpdateVersion(ctx, version); err != nil {
		return version, false, err
	}
	return version, true, nil
}

func markVersionMissing(ctx context.Context, repository Repository, now func() time.Time, version GameVersion) (GameVersion, bool, error) {
	if version.Status == "failed" && version.ExecutablePath == "" {
		return version, false, nil
	}
	version.ExecutablePath = ""
	version.Status = "failed"
	verifiedAt := now().UTC()
	version.VerifiedAt = &verifiedAt
	if err := repository.UpdateVersion(ctx, version); err != nil {
		return version, false, err
	}
	return version, false, nil
}

func (service *QueryService) ListAvailable(ctx context.Context) ([]AvailableGameVersion, error) {
	if service.catalog == nil {
		return nil, errs.NewError(errs.ErrVersionCatalog, "The game version catalog is not configured")
	}
	available, err := service.catalog.List(ctx)
	if err != nil {
		errs.LogFailure("game version catalog request failed", err)
		return nil, &errs.AppError{Code: errs.ErrVersionCatalog, Message: "Could not load the official game version catalog", Retryable: true, Cause: err}
	}
	installed, err := service.repository.ListVersions(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]GameVersion, len(installed))
	for _, version := range installed {
		version, _, err = service.verify(ctx, version)
		if err != nil {
			return nil, err
		}
		byID[version.ID] = version
	}
	for index := range available {
		if version, ok := byID[available[index].ID]; ok {
			available[index].Installed = version.Status == "installed"
			status := version.Status
			available[index].InstallStatus = &status
		}
	}
	return available, nil
}

func validateID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", errs.NewError(errs.ErrValidation, "Enter a version ID")
	}
	if len([]byte(id)) > 180 {
		return "", errs.NewError(errs.ErrValidation, "Version ID cannot exceed 180 bytes")
	}
	for _, char := range id {
		if unicode.IsControl(char) {
			return "", errs.NewError(errs.ErrValidation, "Version ID cannot contain control characters")
		}
	}
	return id, nil
}

func validateCatalogFilename(filename string) error {
	if filename == "" || filename == "." || filepath.IsAbs(filename) || filepath.Base(filename) != filename || strings.ContainsAny(filename, `/\\`) {
		return errs.NewError(errs.ErrVersionCatalog, "The game version catalog contains an invalid package filename")
	}
	return nil
}

func isCode(err error, code string) bool {
	var appError *errs.AppError
	return errors.As(err, &appError) && appError.Code == code
}

func titleParams(name string) map[string]string { return map[string]string{"name": name} }

func trim(value string) string { return strings.TrimSpace(value) }
