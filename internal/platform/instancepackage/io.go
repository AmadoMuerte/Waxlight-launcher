package instancepackage

import (
	"context"

	"github.com/waxlight/waxlight-launcher/internal/instances"
)

// Store adapts the archive implementation to the feature-owned PackageIO port
// so instance orchestration stays independent of the .waxlight format.
type Store struct{}

func (Store) Open(path string) (instances.PackageArchive, error) {
	pkg, err := Open(path)
	if err != nil {
		return nil, err
	}
	return archiveAdapter{pkg: pkg}, nil
}

func (Store) Write(ctx context.Context, targetPath string, source instances.PackageWriteSource) error {
	return Write(ctx, targetPath, WriteSource{
		Manifest:     source.Manifest,
		InstanceDir:  source.InstanceDir,
		EmbeddedMods: source.EmbeddedMods,
		IconPath:     source.IconPath,
	})
}

type archiveAdapter struct {
	pkg *Package
}

func (adapter archiveAdapter) Manifest() instances.PackageManifest {
	return adapter.pkg.Manifest
}

func (adapter archiveAdapter) TotalSize() int64 {
	var total int64
	for _, size := range adapter.pkg.Entries {
		total += size
	}
	return total
}

func (adapter archiveAdapter) ExtractConfigs(ctx context.Context, targetDir string) error {
	return adapter.pkg.ExtractConfigs(ctx, targetDir)
}

func (adapter archiveAdapter) ExtractEmbeddedMod(ctx context.Context, fileName string, directory string) error {
	return adapter.pkg.ExtractEmbeddedMod(ctx, fileName, directory)
}

func (adapter archiveAdapter) ExtractIcon(ctx context.Context, destination string) error {
	return adapter.pkg.ExtractIcon(ctx, destination)
}

var _ instances.PackageArchive = archiveAdapter{}
var _ instances.PackageIO = Store{}
