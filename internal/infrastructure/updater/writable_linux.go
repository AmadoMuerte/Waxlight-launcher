//go:build linux

package updater

import "golang.org/x/sys/unix"

func directoryWritable(path string) bool {
	return unix.Access(path, unix.W_OK) == nil
}
