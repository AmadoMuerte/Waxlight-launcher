//go:build windows

package optimum

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	optimumfeature "github.com/AmadoMuerte/Waxlight-launcher/internal/optimum"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type Locator struct{}

func NewLocator() Locator { return Locator{} }

func (Locator) Detect() (optimumfeature.Installation, error) {
	candidates := []string{}
	key, err := registry.OpenKey(registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\Optimum_is1`, registry.QUERY_VALUE)
	if err == nil {
		if path, _, valueErr := key.GetStringValue("InstallLocation"); valueErr == nil {
			candidates = append(candidates, strings.TrimSpace(path))
		}
		_ = key.Close()
	}
	candidates = append(candidates, `C:\Games\Optimum`)
	for _, candidate := range candidates {
		installation, inspectErr := (Locator{}).Inspect(candidate)
		if inspectErr == nil {
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
	executable := filepath.Join(directory, "Optimum.exe")
	if err := requireRegular(executable, false); err != nil {
		return optimumfeature.Installation{}, err
	}
	if err := requireRegular(filepath.Join(directory, "Vintagestory.exe"), false); err != nil {
		return optimumfeature.Installation{}, err
	}
	if err := requireRegular(filepath.Join(directory, ".optimum", "package-complete"), false); err != nil {
		return optimumfeature.Installation{}, errors.Join(optimumfeature.ErrInvalidInstall, err)
	}
	return optimumfeature.Installation{
		Path: directory, Executable: executable, WorkingDirectory: directory,
		GameVersion: gameVersion(directory), Exclusive: true,
	}, nil
}

func (Locator) GameVersion(path string) string { return gameVersion(path) }

func (Locator) InUse(installation optimumfeature.Installation) (bool, error) {
	lockPath := filepath.Join(installation.Path, ".optimum", "game.lock")
	if _, err := os.Stat(filepath.Dir(lockPath)); err != nil {
		return false, err
	}
	path, err := windows.UTF16PtrFromString(lockPath)
	if err != nil {
		return false, err
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, windows.CloseHandle(handle)
}
