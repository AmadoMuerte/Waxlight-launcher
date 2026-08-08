//go:build linux

package gameversion

import "context"

func (installer Installer) installPlatform(
	ctx context.Context,
	sourcePath string,
	targetPath string,
	progress func(copied, total int64),
) (string, int64, error) {
	return installer.archives.Install(ctx, sourcePath, targetPath, "", "", progress)
}
