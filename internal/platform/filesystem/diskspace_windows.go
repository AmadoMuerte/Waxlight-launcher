//go:build windows

package filesystem

import "golang.org/x/sys/windows"

func (DiskSpace) Available(path string) (int64, error) {
	directoryName, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var available uint64
	if err := windows.GetDiskFreeSpaceEx(
		directoryName,
		&available,
		nil,
		nil,
	); err != nil {
		return 0, err
	}
	if available > uint64(^uint64(0)>>1) {
		return int64(^uint64(0) >> 1), nil
	}
	return int64(available), nil
}
