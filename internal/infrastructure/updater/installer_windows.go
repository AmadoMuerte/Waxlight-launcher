//go:build windows

package updater

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func (*Installer) Apply(_ context.Context, installerPath string, currentPID int) error {
	if !strings.EqualFold(filepath.Ext(installerPath), ".exe") {
		return fmt.Errorf("Windows launcher update is not an installer executable")
	}
	if currentPID <= 0 {
		return fmt.Errorf("invalid launcher process ID")
	}

	installerPath, err := filepath.Abs(installerPath)
	if err != nil {
		return fmt.Errorf("resolve launcher installer path: %w", err)
	}
	installerInfo, err := os.Lstat(installerPath)
	if err != nil {
		return fmt.Errorf("inspect launcher installer: %w", err)
	}
	if !installerInfo.Mode().IsRegular() || installerInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("launcher installer is not a safe regular file")
	}

	currentExecutable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current launcher executable: %w", err)
	}
	currentExecutable, err = filepath.EvalSymlinks(currentExecutable)
	if err != nil {
		return fmt.Errorf("resolve current launcher executable symlinks: %w", err)
	}

	helperPath := filepath.Join(filepath.Dir(installerPath), "apply-waxlight-update.ps1")
	if err := os.WriteFile(helperPath, []byte(windowsUpdateHelperScript), 0o600); err != nil {
		return fmt.Errorf("write Windows update helper: %w", err)
	}

	cmd := exec.Command(
		"powershell.exe",
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-File", helperPath,
		"-InstallerPath", installerPath,
		"-CurrentPID", strconv.Itoa(currentPID),
		"-CurrentExecutable", currentExecutable,
	)
	cmd.Dir = filepath.Dir(installerPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(helperPath)
		return fmt.Errorf("start Windows update helper: %w", err)
	}
	return nil
}
