//go:build !windows

package credentials

import (
	"errors"
	"io"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func readLegacyFile(path string) ([]byte, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, syscall.ENOENT) {
			return nil, os.ErrNotExist
		}
		if errors.Is(err, syscall.ELOOP) {
			return nil, errors.New("legacy credential path is a symlink")
		}
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("legacy credential path is not a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("legacy credential file permissions are not owner-only")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return nil, errors.New("legacy credential file is not owned by the current user")
	}
	return io.ReadAll(io.LimitReader(file, maxLegacyFileBytes+1))
}
