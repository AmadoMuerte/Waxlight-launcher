package application

import (
	"context"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

// ModIdentity adapts the application-level mod source helpers to the narrow
// instance package port until the mods feature owns this behavior.
func (s *Service) ParseModDBSource(source string) (string, string, bool) {
	return parseModDBSource(source)
}

func (s *Service) ModDBSource(modID, versionID string) string {
	return modDBSource(modID, versionID)
}

func (s *Service) FindModVersion(versions []domain.ModVersion, id string) (domain.ModVersion, bool) {
	return findModVersion(versions, id)
}

func (s *Service) ModSupportsVersion(versions []string, requested string) bool {
	return modSupportsVersion(versions, requested)
}

// CatalogModInstaller exposes the catalog download and enable-toggle methods
// that package import needs until the mods feature owns them.
func (s *Service) DownloadCatalogModForPackage(ctx context.Context, request domain.DownloadModRequest) (domain.ModInstallResult, error) {
	return s.DownloadCatalogMod(ctx, request)
}

// GetDownloadedMod reads a cached catalog download for package export.
func (s *Service) GetDownloadedMod(ctx context.Context, modID, versionID string) (domain.DownloadedMod, error) {
	if s.modDownloads == nil {
		return domain.DownloadedMod{}, domain.NewError(domain.ErrModCatalog, "Mod downloads are not configured")
	}
	return s.modDownloads.Get(ctx, modID, versionID)
}

var _ instancesModIdentity = (*Service)(nil)

type instancesModIdentity interface {
	ParseModDBSource(string) (string, string, bool)
	ModDBSource(string, string) string
	FindModVersion([]domain.ModVersion, string) (domain.ModVersion, bool)
	ModSupportsVersion([]string, string) bool
}

var _ instancesCatalogModInstaller = (*Service)(nil)

type instancesCatalogModInstaller interface {
	DownloadCatalogMod(context.Context, domain.DownloadModRequest) (domain.ModInstallResult, error)
	SetModEnabled(context.Context, string, bool) (domain.InstalledMod, error)
}

var _ instancesPackageCatalog = (*Service)(nil)

type instancesPackageCatalog interface {
	GetCatalogMod(context.Context, string) (domain.ModDetails, error)
}

var _ instancesPackageDownloaded = (*Service)(nil)

type instancesPackageDownloaded interface {
	GetDownloadedMod(context.Context, string, string) (domain.DownloadedMod, error)
}

// SafeRemoveAll exports the verified instance-directory removal used by
// instance and package cleanup wiring.
func SafeRemoveAll(path, dataRoot, marker string) error {
	return safeRemoveAll(path, dataRoot, marker)
}
