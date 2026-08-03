//go:build !windows

package credentials

import (
	"errors"
	"os"
	"syscall"
)

func validatePendingPermissions(info os.FileInfo) error {
	if info.Mode().Perm()&0o077 != 0 {
		return errors.New("pending credential journal permissions are not owner-only")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return errors.New("pending credential journal has an unexpected owner")
	}
	return nil
}
