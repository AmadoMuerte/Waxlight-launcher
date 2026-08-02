package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

func Write(path string, data []byte, permissions os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	temporaryPath := path + ".tmp"
	file, err := os.OpenFile(
		temporaryPath,
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
		permissions,
	)
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
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
	return nil
}
