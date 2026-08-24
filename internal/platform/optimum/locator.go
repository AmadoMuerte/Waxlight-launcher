package optimum

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	optimumfeature "github.com/AmadoMuerte/Waxlight-launcher/internal/optimum"
)

var versionMarker = regexp.MustCompile(`^version-([0-9]+\.[0-9]+\.[0-9]+)\.txt$`)

func gameVersion(directory string) string {
	entries, err := os.ReadDir(filepath.Join(directory, "assets"))
	if err != nil {
		return ""
	}
	versions := make([]string, 0, 1)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := versionMarker.FindStringSubmatch(entry.Name())
		if len(match) == 2 {
			versions = append(versions, match[1])
		}
	}
	sort.Strings(versions)
	if len(versions) != 1 {
		return ""
	}
	return versions[0]
}

func requireDirectory(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", errors.Join(optimumfeature.ErrInvalidInstall, err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.Join(optimumfeature.ErrNotFound, err)
		}
		return "", err
	}
	if !info.IsDir() {
		absolute = filepath.Dir(absolute)
	}
	return absolute, nil
}

func requireRegular(path string, executable bool) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.Join(optimumfeature.ErrExecutableMissing, err)
		}
		return err
	}
	if !info.Mode().IsRegular() {
		return optimumfeature.ErrExecutableMissing
	}
	if executable && info.Mode().Perm()&0o111 == 0 {
		return optimumfeature.ErrExecutableMissing
	}
	return nil
}

func requireDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return errors.Join(optimumfeature.ErrInvalidInstall, err)
	}
	if !info.IsDir() {
		return optimumfeature.ErrInvalidInstall
	}
	return nil
}
