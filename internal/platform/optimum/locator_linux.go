//go:build linux

package optimum

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	optimumfeature "github.com/AmadoMuerte/Waxlight-launcher/internal/optimum"
)

type Locator struct{}

func NewLocator() Locator { return Locator{} }

func (Locator) Detect() (optimumfeature.Installation, error) {
	for _, candidate := range linuxCandidates() {
		installation, err := (Locator{}).Inspect(candidate)
		if err == nil {
			return installation, nil
		}
	}
	return optimumfeature.Installation{}, optimumfeature.ErrNotFound
}

func (Locator) Inspect(path string) (optimumfeature.Installation, error) {
	directory, err := requireDirectory(path)
	if err != nil {
		return optimumfeature.Installation{}, err
	}
	runner := filepath.Join(directory, "run.sh")
	if err := requireRegular(runner, true); err != nil {
		return optimumfeature.Installation{}, err
	}
	if err := requireRegular(filepath.Join(directory, "Optimum"), true); err != nil {
		return optimumfeature.Installation{}, err
	}
	if err := requireDir(filepath.Join(directory, ".optimum", "donors")); err != nil {
		return optimumfeature.Installation{}, err
	}
	return optimumfeature.Installation{
		Path: directory, Executable: runner, WorkingDirectory: directory,
		GameVersion: gameVersion(directory),
	}, nil
}

func (Locator) GameVersion(path string) string { return gameVersion(path) }

func (Locator) InUse(optimumfeature.Installation) (bool, error) { return false, nil }

func linuxCandidates() []string {
	home, _ := os.UserHomeDir()
	dataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if dataHome == "" && home != "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	result := make([]string, 0, 2)
	if dataHome != "" {
		if path := desktopInstallPath(filepath.Join(dataHome, "applications", "optimum.desktop")); path != "" {
			result = append(result, path)
		}
		result = append(result, filepath.Join(dataHome, "optimum"))
	}
	return result
}

func desktopInstallPath(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "Exec=") {
			continue
		}
		command := strings.TrimSpace(strings.TrimPrefix(line, "Exec="))
		if strings.HasPrefix(command, `"`) {
			if end := strings.Index(command[1:], `"`); end >= 0 {
				command = command[1 : end+1]
			}
		} else if fields := strings.Fields(command); len(fields) > 0 {
			command = fields[0]
		}
		if strings.EqualFold(filepath.Base(command), "optimum-launch.sh") {
			return filepath.Dir(command)
		}
	}
	return ""
}
