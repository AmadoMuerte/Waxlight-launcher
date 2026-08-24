// Package instancedirectory owns allocation and initialization of instance directories.
package instancedirectory

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/securefs"
)

const markerName = ".waxlight-instance"

type Layout interface {
	EnsureLayout(string) error
}

type Storage struct {
	layout Layout
	mu     sync.Mutex
}

func New(layout Layout) *Storage {
	return &Storage{layout: layout}
}

func (storage *Storage) Allocate(directory, instanceID string) (instances.DirectoryAllocation, error) {
	storage.mu.Lock()
	allocation := &allocation{storage: storage}
	fail := func(err error) (instances.DirectoryAllocation, error) {
		_ = allocation.rollback()
		storage.mu.Unlock()
		allocation.released = true
		return nil, err
	}

	directory, err := filepath.Abs(directory)
	if err != nil {
		return fail(err)
	}
	allocation.directory = directory
	if err := os.MkdirAll(filepath.Dir(directory), 0o755); err != nil {
		return fail(&errs.AppError{Code: errs.ErrFilePermission, Message: "Failed to create the instance directory", Cause: err})
	}
	if err := os.Mkdir(directory, 0o755); err == nil {
		allocation.ownsDirectory = true
	} else if !errors.Is(err, os.ErrExist) {
		return fail(&errs.AppError{Code: errs.ErrFilePermission, Message: "Failed to create the instance directory", Cause: err})
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return fail(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fail(errors.New("instance directory is a symlink"))
	}
	allocation.directoryInfo = info

	markerPath := filepath.Join(directory, markerName)
	marker, err := os.OpenFile(markerPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fail(errs.NewError(instances.ErrDirectoryConflict, "The directory is already used by another instance"))
		}
		return fail(err)
	}
	allocation.markerPath = markerPath
	allocation.markerInfo, err = marker.Stat()
	if err != nil {
		_ = marker.Close()
		return fail(err)
	}
	if _, err := marker.Write([]byte(instanceID)); err != nil {
		_ = marker.Close()
		return fail(err)
	}
	if err := marker.Close(); err != nil {
		return fail(err)
	}
	if err := securefs.Apply(markerPath, 0o600, false); err != nil {
		return fail(err)
	}

	for _, name := range []string{"Mods", "ModsDisabled", "Logs"} {
		if _, err := os.Lstat(filepath.Join(directory, name)); errors.Is(err, os.ErrNotExist) {
			allocation.createdPaths = append(allocation.createdPaths, filepath.Join(directory, name))
		}
	}
	if err := storage.layout.EnsureLayout(directory); err != nil {
		return fail(err)
	}
	if err := HardenLogs(filepath.Join(directory, "Logs")); err != nil {
		return fail(err)
	}
	return allocation, nil
}

type allocation struct {
	storage       *Storage
	directory     string
	directoryInfo os.FileInfo
	markerPath    string
	markerInfo    os.FileInfo
	createdPaths  []string
	ownsDirectory bool
	released      bool
}

func (allocation *allocation) Directory() string { return allocation.directory }

func (allocation *allocation) Commit() {
	if allocation.released {
		return
	}
	allocation.released = true
	allocation.storage.mu.Unlock()
}

func (allocation *allocation) Rollback() error {
	if allocation.released {
		return nil
	}
	err := allocation.rollback()
	allocation.released = true
	allocation.storage.mu.Unlock()
	return err
}

func (allocation *allocation) rollback() error {
	if allocation.ownsDirectory {
		if allocation.markerPath == "" {
			if err := os.Remove(allocation.directory); err != nil && !errors.Is(err, os.ErrNotExist) &&
				!errors.Is(err, syscall.ENOTEMPTY) && !errors.Is(err, syscall.EEXIST) {
				return err
			}
			return nil
		}
		ownsDirectory, err := allocation.ownsPath(allocation.directory, allocation.directoryInfo)
		if err != nil || !ownsDirectory {
			return err
		}
		ownsMarker, err := allocation.ownsPath(allocation.markerPath, allocation.markerInfo)
		if err != nil || !ownsMarker {
			return err
		}
		rollbackRoot, err := os.MkdirTemp(filepath.Dir(allocation.directory), ".waxlight-rollback-")
		if err != nil {
			return err
		}
		if err := os.Rename(allocation.directory, filepath.Join(rollbackRoot, "instance")); err != nil {
			_ = os.Remove(rollbackRoot)
			return err
		}
		return os.RemoveAll(rollbackRoot)
	}
	var rollbackErr error
	for index := len(allocation.createdPaths) - 1; index >= 0; index-- {
		// Remove only empty directories. Existing or migrated user content remains untouched.
		if err := os.Remove(allocation.createdPaths[index]); err != nil &&
			!errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ENOTEMPTY) && !errors.Is(err, syscall.EEXIST) && rollbackErr == nil {
			rollbackErr = err
		}
	}
	if allocation.markerPath != "" {
		ownsMarker, err := allocation.ownsPath(allocation.markerPath, allocation.markerInfo)
		if err != nil && rollbackErr == nil {
			rollbackErr = err
		} else if ownsMarker {
			if err := os.Remove(allocation.markerPath); err != nil && !errors.Is(err, os.ErrNotExist) && rollbackErr == nil {
				rollbackErr = err
			}
		}
	}
	return rollbackErr
}

func (allocation *allocation) ownsPath(path string, expected os.FileInfo) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return expected != nil && os.SameFile(expected, info), nil
}

func HardenLogs(logsDirectory string) error {
	if err := os.MkdirAll(logsDirectory, 0o700); err != nil {
		return err
	}
	return filepath.Walk(logsDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("log path contains a symlink")
		}
		if info.IsDir() {
			return securefs.Apply(path, 0o700, true)
		}
		if !info.Mode().IsRegular() {
			return errors.New("log path contains a non-regular file")
		}
		return securefs.Apply(path, 0o600, false)
	})
}
