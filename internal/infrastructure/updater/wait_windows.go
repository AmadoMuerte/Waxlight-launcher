//go:build windows

package updater

import "time"

func WaitForParent(int, time.Duration) {}
