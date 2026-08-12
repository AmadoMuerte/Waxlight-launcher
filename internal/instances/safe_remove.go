package instances

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/errs"
)

// SafeRemoveAll verifies that path is an owned instance directory (it carries
// the marker file) before deleting it recursively. It refuses to touch home,
// the data root, the volume root, or any short path, so a malformed instance
// record can never lead the launcher to delete unrelated data.
func SafeRemoveAll(path, dataRoot, marker string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	root, rootError := filepath.Abs(dataRoot)
	if rootError != nil {
		return rootError
	}
	home, _ := os.UserHomeDir()
	volumeRoot := filepath.VolumeName(abs) + string(os.PathSeparator)
	if abs == "/" || abs == volumeRoot || abs == home || abs == root || len(abs) < 5 {
		return errs.NewError(errs.ErrValidation, "Unsafe deletion path")
	}
	if _, err = os.Stat(filepath.Join(abs, marker)); err != nil {
		return errs.NewError(errs.ErrValidation, "The directory is not managed by Waxlight; no files were deleted")
	}
	return removeAllReliably(abs)
}

func removeAllReliably(path string) error {
	var lastError error
	for attempt := 0; attempt < 5; attempt++ {
		if runtime.GOOS == "windows" {
			// Extracted installers may leave read-only attributes behind. Go's
			// chmod implementation clears that attribute on Windows.
			_ = filepath.Walk(path, func(currentPath string, info os.FileInfo, walkErr error) error {
				if walkErr != nil {
					slog.Debug("walk failed while clearing read-only attributes", "path", currentPath, "error", walkErr)
					return nil
				}
				if info == nil {
					return nil
				}
				if chmodErr := os.Chmod(currentPath, info.Mode()|0o200); chmodErr != nil {
					slog.Debug("could not clear the read-only attribute", "path", currentPath, "error", chmodErr)
				}
				return nil
			})
		}

		lastError = os.RemoveAll(path)
		if lastError == nil {
			_, statError := os.Lstat(path)
			if errors.Is(statError, os.ErrNotExist) {
				return nil
			}
			if statError != nil {
				return statError
			}
			lastError = fmt.Errorf("directory still exists after recursive removal: %s", path)
		}
		if runtime.GOOS != "windows" {
			break
		}
		time.Sleep(time.Duration(attempt+1) * 50 * time.Millisecond)
	}
	return lastError
}
