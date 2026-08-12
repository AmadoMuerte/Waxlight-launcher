//go:build windows

package credentials

import (
	"errors"
	"io"
	"os"
)

func readLegacyFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("legacy credential path is a symlink")
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("legacy credential path is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, maxLegacyFileBytes+1))
}
