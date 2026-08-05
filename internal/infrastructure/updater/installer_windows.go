//go:build windows

package updater

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	shellExecuteSuccessThreshold               = 32
	windowsErrorCancelled        syscall.Errno = 1223
)

var shellExecuteW = syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")

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
		return classifyWindowsError(err)
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

	parameters := windowsCommandLine(
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-File", helperPath,
		"-InstallerPath", installerPath,
		"-CurrentPID", strconv.Itoa(currentPID),
		"-CurrentExecutable", currentExecutable,
	)

	if err := shellExecuteElevated("powershell.exe", parameters, filepath.Dir(installerPath)); err != nil {
		_ = os.Remove(helperPath)
		return classifyWindowsError(err)
	}
	return nil
}

func windowsCommandLine(arguments ...string) string {
	escaped := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		escaped = append(escaped, syscall.EscapeArg(argument))
	}
	return strings.Join(escaped, " ")
}

func shellExecuteElevated(file, parameters, workingDirectory string) error {
	operationPtr, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	filePtr, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	parametersPtr, err := syscall.UTF16PtrFromString(parameters)
	if err != nil {
		return err
	}
	workingDirectoryPtr, err := syscall.UTF16PtrFromString(workingDirectory)
	if err != nil {
		return err
	}

	result, _, callErr := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(operationPtr)),
		uintptr(unsafe.Pointer(filePtr)),
		uintptr(unsafe.Pointer(parametersPtr)),
		uintptr(unsafe.Pointer(workingDirectoryPtr)),
		uintptr(syscall.SW_HIDE),
	)
	if result > shellExecuteSuccessThreshold {
		return nil
	}
	if callErr != nil && !errors.Is(callErr, syscall.Errno(0)) {
		return callErr
	}
	return syscall.Errno(result)
}

func classifyWindowsError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())

	switch {
	case errors.Is(err, os.ErrNotExist),
		strings.Contains(msg, "file not found"),
		strings.Contains(msg, "cannot find the file"):
		return fmt.Errorf("%w: %v", ErrInstallerNotFound, err)
	case errors.Is(err, windowsErrorCancelled):
		return fmt.Errorf("%w: update elevation was cancelled", ErrInstallerElevationRequired)
	case errors.Is(err, syscall.ERROR_ACCESS_DENIED),
		strings.Contains(msg, "access is denied"),
		strings.Contains(msg, "operation not permitted"):
		return fmt.Errorf("%w: %v", ErrInstallerAccessDenied, err)
	case strings.Contains(msg, "not a valid win32 application"):
		return fmt.Errorf("%w: %v", ErrInstallerInvalid, err)
	case strings.Contains(msg, "required privilege"), strings.Contains(msg, "elevation"):
		return fmt.Errorf("%w: %v", ErrInstallerElevationRequired, err)
	default:
		return fmt.Errorf("start launcher update helper: %w", err)
	}
}

var (
	ErrInstallerNotFound          = fmt.Errorf("installer executable not found")
	ErrInstallerAccessDenied      = fmt.Errorf("access denied starting installer")
	ErrInstallerInvalid           = fmt.Errorf("invalid Windows executable")
	ErrInstallerElevationRequired = fmt.Errorf("administrator privileges required")
)
