//go:build windows

package gameversion

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (installer Installer) installPlatform(
	ctx context.Context,
	sourcePath string,
	targetPath string,
	progress func(copied, total int64),
) (string, int64, error) {
	if !strings.EqualFold(filepath.Ext(sourcePath), ".exe") {
		return "", 0, fmt.Errorf("the official Windows distribution must be an EXE installer")
	}
	if err := os.RemoveAll(targetPath); err != nil {
		return "", 0, err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", 0, err
	}

	// The official client installer identifies itself as Inno Setup. These are
	// documented Inno Setup switches; passing each argument separately avoids
	// shell interpretation of the installation path.
	command := exec.CommandContext(
		ctx,
		sourcePath,
		"/SP-",
		"/VERYSILENT",
		"/SUPPRESSMSGBOXES",
		"/NORESTART",
		"/CURRENTUSER",
		"/NOICONS",
		"/MERGETASKS=!desktopicon",
		"/DIR="+targetPath,
	)
	if output, err := command.CombinedOutput(); err != nil {
		_ = os.RemoveAll(targetPath)
		return "", 0, fmt.Errorf(
			"Windows game installer failed: %w (%s)",
			err,
			strings.TrimSpace(string(output)),
		)
	}

	executablePath, err := installer.archives.FindExecutable(targetPath, "")
	if err != nil {
		_ = os.RemoveAll(targetPath)
		return "", 0, err
	}
	size, err := installedSize(ctx, targetPath)
	if err != nil {
		return "", 0, err
	}
	return executablePath, size, nil
}

func installedSize(ctx context.Context, rootPath string) (int64, error) {
	var total int64
	err := filepath.Walk(rootPath, func(
		_ string,
		info os.FileInfo,
		walkErr error,
	) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}
