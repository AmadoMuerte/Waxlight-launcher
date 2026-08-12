package vintagestory

import (
	"context"
	"net/http"

	vsversions "github.com/AmadoMuerte/vintagestory-go/versions"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

// VersionCatalog maps generic Vintage Story releases into Waxlight's stable
// game-installation model.
type VersionCatalog struct {
	catalog *vsversions.Catalog
}

func NewVersionCatalog(client *http.Client) *VersionCatalog {
	return &VersionCatalog{catalog: vsversions.NewCatalog(client)}
}

func NewVersionCatalogForPlatform(client *http.Client, endpoint, platform, architecture string) *VersionCatalog {
	return &VersionCatalog{catalog: vsversions.NewCatalogForPlatform(client, endpoint, platform, architecture)}
}

func (catalog *VersionCatalog) List(ctx context.Context) ([]versions.AvailableGameVersion, error) {
	releases, err := catalog.catalog.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]versions.AvailableGameVersion, 0, len(releases))
	for _, release := range releases {
		result = append(result, versions.AvailableGameVersion{
			ID: release.ID, Name: release.Name, Channel: release.Channel,
			Platform: release.Platform, Architecture: release.Architecture,
			Filename: release.Filename, DownloadURL: release.DownloadURL,
			DownloadSize: release.DownloadSize, Checksum: release.Checksum,
			ChecksumAlgorithm: release.ChecksumAlgorithm, Latest: release.Latest,
		})
	}
	return result, nil
}
