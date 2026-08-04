//go:build windows

package updater

func directoryWritable(string) bool { return false }
