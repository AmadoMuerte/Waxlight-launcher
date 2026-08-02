package gameversion

import (
	"context"

	"github.com/waxlight/waxlight-launcher/internal/infrastructure/filesystem"
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
) (string, int64, error) {
	return installer.installPlatform(ctx, sourcePath, targetPath)
}
