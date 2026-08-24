package versionfs

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
)

type Filesystem struct {
	root string
}

func New(root string) Filesystem { return Filesystem{root: root} }

func (fs Filesystem) DownloadPath(filename string) string {
	return filepath.Join(fs.root, "downloads", filename)
}

func (fs Filesystem) VersionPath(id string) string {
	return filepath.Join(fs.root, "versions", safeSegment(id))
}

func (Filesystem) ExecutableExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (Filesystem) MakeExecutable(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return os.Chmod(path, 0o755)
}

func (Filesystem) WriteMarker(directory, id string) error {
	return os.WriteFile(filepath.Join(directory, ".waxlight-version"), []byte(id), 0o600)
}

func (Filesystem) RemoveDownload(path string) error {
	for _, candidate := range []string{path + ".partial", path} {
		if err := os.Remove(candidate); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (fs Filesystem) RemoveInstallTarget(path, id string) error {
	if !samePath(path, fs.VersionPath(id)) {
		return errs.NewError(errs.ErrValidation, "Refusing to remove an unexpected game version directory")
	}
	return os.RemoveAll(path)
}

func (fs Filesystem) RemoveVersion(path, id string) error {
	cleanRoot, err := filepath.Abs(fs.root)
	if err != nil {
		return err
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errs.NewError(errs.ErrValidation, "Refusing to remove a directory outside the launcher data folder")
	}
	marker, err := os.ReadFile(filepath.Join(cleanPath, ".waxlight-version"))
	if err != nil || string(marker) != id {
		return errs.NewError(errs.ErrValidation, "Refusing to remove a directory not owned by the launcher")
	}
	return os.RemoveAll(cleanPath)
}

func (fs Filesystem) RemoveVersionsRootIfEmpty(installationDir string) error {
	root := filepath.Join(fs.root, "versions")
	if !samePath(filepath.Dir(installationDir), root) {
		return nil
	}
	if err := os.Remove(root); err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, os.ErrExist) {
		return err
	}
	return nil
}

func safeSegment(value string) string {
	if value != "" && value != "." && value != ".." {
		safe := true
		for _, char := range value {
			if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '.' || char == '-' || char == '_') {
				safe = false
				break
			}
		}
		base := strings.ToUpper(strings.TrimSuffix(value, filepath.Ext(value)))
		switch base {
		case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
			safe = false
		}
		if strings.HasSuffix(value, ".") {
			safe = false
		}
		if safe {
			return value
		}
	}
	// '~' is excluded from ordinary IDs, making this reversible encoding a
	// disjoint namespace with no lossy path collisions.
	return "~" + base64.RawURLEncoding.EncodeToString([]byte(value))
}

func samePath(left, right string) bool {
	leftPath, leftErr := filepath.Abs(left)
	rightPath, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftPath) == filepath.Clean(rightPath)
}
