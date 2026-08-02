//go:build linux

package filesystem

import "golang.org/x/sys/unix"

func (DiskSpace) Available(path string) (int64, error) {
	var statistics unix.Statfs_t
	if err := unix.Statfs(path, &statistics); err != nil {
		return 0, err
	}
	return int64(statistics.Bavail) * int64(statistics.Bsize), nil
}
