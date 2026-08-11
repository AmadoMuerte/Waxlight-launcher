package gameversion

import (
	"context"

	"github.com/waxlight/waxlight-launcher/internal/platform/filesystem"
)

type Installer struct {
	archives filesystem.ArchiveInstaller
}

func NewInstaller() Installer {
	return Installer{archives: filesystem.ArchiveInstaller{}}
}

func (installer Installer) Install(
	ctx context.Context,
	sourcePath string,
	targetPath string,
	progress func(copied, total int64),
) (string, int64, error) {
	return installer.installPlatform(ctx, sourcePath, targetPath, progress)
}
