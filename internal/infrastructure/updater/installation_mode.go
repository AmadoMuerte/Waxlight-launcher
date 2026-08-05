package updater

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type InstallationMode string

const (
	InstallationModeInstalled InstallationMode = "installed"
	InstallationModePortable  InstallationMode = "portable"
)

func DetectInstallationMode() InstallationMode {
	executable, err := os.Executable()
	if err != nil {
		return InstallationModePortable
	}

	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return InstallationModePortable
	}

	dir := filepath.Dir(executable)

	if runtime.GOOS == "windows" {
		return detectWindowsMode(dir)
	}

	return detectUnixMode(dir)
}

func detectWindowsMode(dir string) InstallationMode {
	lower := strings.ToLower(dir)

	programFiles := []string{
		`\program files\`,
		`\program files (x86)\`,
		`\windows\`,
		`\programdata\`,
	}

	for _, pf := range programFiles {
		if strings.Contains(lower, pf) {
			return InstallationModeInstalled
		}
	}

	if !isDirectoryWritable(dir) {
		return InstallationModeInstalled
	}

	return InstallationModePortable
}

func detectUnixMode(dir string) InstallationMode {
	standardDirs := []string{
		"/usr/",
		"/opt/",
		"/snap/",
		"/var/",
	}

	for _, stdDir := range standardDirs {
		if strings.HasPrefix(dir, stdDir) {
			return InstallationModeInstalled
		}
	}

	if !isDirectoryWritable(dir) {
		return InstallationModeInstalled
	}

	return InstallationModePortable
}

func isDirectoryWritable(dir string) bool {
	testFile := filepath.Join(dir, ".waxlight-write-test")
	f, err := os.Create(testFile)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(testFile)
	return true
}
