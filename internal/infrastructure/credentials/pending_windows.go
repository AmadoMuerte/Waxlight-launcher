//go:build windows

package credentials

import "os"

func validatePendingPermissions(os.FileInfo) error { return nil }
