//go:build windows

package gameversion

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
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
	command := exec.Command(
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
	// The silent run records targetPath as Inno's previous install directory
	// and leaves an Uninstall entry behind, which would later prefill the path
	// page when the user runs the installer manually. Put the pre-run state
	// back once the run finishes, regardless of its outcome.
	state := snapshotInnoState(targetPath)
	output, runErr := runWindowsInstaller(ctx, command)
	// Restore even after a failed run: the installer may have written its
	// registry state before exiting.
	restoreErr := state.Restore()
	if runErr != nil {
		_ = os.RemoveAll(targetPath)
		if ctx.Err() != nil {
			return "", 0, ctx.Err()
		}
		if restoreErr != nil {
			runErr = fmt.Errorf("%w (registry state restore failed: %v)", runErr, restoreErr)
		}
		return "", 0, fmt.Errorf(
			"Windows game installer failed: %w (%s)",
			runErr,
			strings.TrimSpace(string(output)),
		)
	}
	if restoreErr != nil {
		_ = os.RemoveAll(targetPath)
		return "", 0, fmt.Errorf("restore installer registry state: %w", restoreErr)
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

func runWindowsInstaller(ctx context.Context, command *exec.Cmd) ([]byte, error) {
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		return nil, err
	}

	job, err := installerJob(uint32(command.Process.Pid))
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return output.Bytes(), fmt.Errorf("control Windows game installer process: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		_ = windows.CloseHandle(job)
		return output.Bytes(), err
	case <-ctx.Done():
		_ = windows.CloseHandle(job)
		<-done
		return output.Bytes(), ctx.Err()
	}
}

func installerJob(pid uint32) (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
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
