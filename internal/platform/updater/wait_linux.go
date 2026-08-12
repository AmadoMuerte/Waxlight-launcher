//go:build linux

package updater

import (
	"errors"
	"syscall"
	"time"
)

func WaitForParent(pid int, timeout time.Duration) {
	if pid <= 0 {
		return
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
