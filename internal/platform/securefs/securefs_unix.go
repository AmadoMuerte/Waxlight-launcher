//go:build !windows

package securefs

import (
	"os"
)

func Apply(path string, mode os.FileMode, _ bool) error { return os.Chmod(path, mode) }
