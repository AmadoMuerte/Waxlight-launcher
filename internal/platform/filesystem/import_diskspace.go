package filesystem

import (
	"errors"
	"os"
	"path/filepath"
)

// ImportDiskSpace resolves a possibly not-yet-created custom instance path to
// the nearest existing parent on the same volume.
type ImportDiskSpace struct{}

func (ImportDiskSpace) Available(path string) (int64, error) {
	current, err := filepath.Abs(path)
	if err != nil {
		return 0, err
	}
	for {
		if _, err := os.Stat(current); err == nil {
			return (DiskSpace{}).Available(current)
		} else if !errors.Is(err, os.ErrNotExist) {
			return 0, err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return 0, os.ErrNotExist
		}
		current = parent
	}
}
