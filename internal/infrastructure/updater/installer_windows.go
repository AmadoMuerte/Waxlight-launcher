//go:build windows

package updater

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func (*Installer) Apply(_ context.Context, installerPath string) error {
	if !strings.EqualFold(filepath.Ext(installerPath), ".exe") {
		return fmt.Errorf("Windows launcher update is not an installer executable")
	}
	command := exec.Command(installerPath)
	command.Dir = filepath.Dir(installerPath)
	if err := command.Start(); err != nil {
		return fmt.Errorf("start launcher installer: %w", err)
	}
	return nil
}
