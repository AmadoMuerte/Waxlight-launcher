package atomicfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/waxlight/waxlight-launcher/internal/platform/securefs"
)

func Write(path string, data []byte, permissions os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	parentInfo, err := os.Lstat(filepath.Dir(path))
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("atomic file parent is not a safe directory")
	}
	if targetInfo, targetErr := os.Lstat(path); targetErr == nil {
		if !targetInfo.Mode().IsRegular() || targetInfo.Mode()&os.ModeSymlink != 0 {
			return errors.New("atomic file target is not a regular file")
		}
	} else if !errors.Is(targetErr, os.ErrNotExist) {
		return fmt.Errorf("inspect atomic file target: %w", targetErr)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".waxlight-atomic-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	temporaryPath := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(temporaryPath)
	}
	if _, err := file.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := file.Chmod(permissions); err != nil {
		cleanup()
		return fmt.Errorf("set temporary file permissions: %w", err)
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := replaceFile(temporaryPath, path); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("replace target file: %w", err)
	}
	if err := securefs.Apply(path, permissions, false); err != nil {
		return fmt.Errorf("secure target permissions: %w", err)
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return fmt.Errorf("sync parent directory: %w", err)
	}
	return nil
}
