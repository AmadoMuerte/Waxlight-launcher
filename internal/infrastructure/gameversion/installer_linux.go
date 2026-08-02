//go:build linux

package gameversion

import "context"

func (installer Installer) installPlatform(
	ctx context.Context,
	sourcePath string,
	targetPath string,
) (string, int64, error) {
	return installer.archives.Install(ctx, sourcePath, targetPath, "", "")
}
