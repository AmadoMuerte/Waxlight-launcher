//go:build windows

package updater

import (
	"context"
	"fmt"
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

	command := exec.Command(installerPath,
		"/S",
		"/CURRENT_PID="+strconv.Itoa(currentPID),
	)
	command.Dir = filepath.Dir(installerPath)
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	if err := command.Start(); err != nil {
		return classifyWindowsError(err)
	}
	return nil
}

func classifyWindowsError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()

	switch {
	case strings.Contains(msg, "file not found") || strings.Contains(msg, "cannot find the file"):
		return fmt.Errorf("%w: %v", ErrInstallerNotFound, err)
	case strings.Contains(msg, "Access is denied") || strings.Contains(msg, "Operation not permitted"):
		return fmt.Errorf("%w: %v", ErrInstallerAccessDenied, err)
	case strings.Contains(msg, "not a valid Win32 application"):
		return fmt.Errorf("%w: %v", ErrInstallerInvalid, err)
	case strings.Contains(msg, "required privilege") || strings.Contains(msg, "elevation"):
		return fmt.Errorf("%w: %v", ErrInstallerElevationRequired, err)
	default:
		return fmt.Errorf("start launcher installer: %w", err)
	}
}

var (
	ErrInstallerNotFound          = fmt.Errorf("installer executable not found")
	ErrInstallerAccessDenied      = fmt.Errorf("access denied starting installer")
	ErrInstallerInvalid           = fmt.Errorf("invalid Windows executable")
	ErrInstallerElevationRequired = fmt.Errorf("administrator privileges required")
)
