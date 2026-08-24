package instancepackage

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
)

const (
	ManifestFileName = "manifest.json"
	IconFileName     = "icon.png"
	ConfigsPrefix    = "configs/"
	ModsPrefix       = "mods/"

	maxManifestBytes     = 1 << 20
	maxConfigBytes       = 16 << 20
	maxEmbeddedModBytes  = 512 << 20
	maxTotalUncompressed = 2 << 30
	maxEntryCount        = 10000
	maxIconBytes         = 8 << 20
	maxNestedConfigDepth = 24
)

// CleanArchiveName normalizes an archive entry name and rejects unsafe paths.
// Backslashes are treated as separators so Windows-generated archives behave
// identically on every platform.
func CleanArchiveName(name string) (string, error) {
	if name == "" {
		return "", errs.NewError(errs.ErrPackageSecurity, "Package contains an empty file name")
	}
	clean := filepath.ToSlash(name)
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimSuffix(clean, "/")
	if strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", errs.NewError(errs.ErrPackageSecurity, "Package contains an absolute or parent path")
	}
	if strings.ContainsRune(clean, 0) {
		return "", errs.NewError(errs.ErrPackageSecurity, "Package contains a malformed path")
	}
	parts := strings.Split(clean, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", errs.NewError(errs.ErrPackageSecurity, "Package contains an unsafe path segment")
		}
	}
	return clean, nil
}

// ValidateEntryName ensures an entry maps to an allowed package location and
// returns the on-disk path that must be created underneath root.
func ValidateEntryName(name string, root string) (string, error) {
	clean, err := CleanArchiveName(name)
	if err != nil {
		return "", err
	}
	if clean == ManifestFileName {
		return "", errs.NewError(errs.ErrPackageSecurity, "Package contains a duplicate manifest")
	}
	switch {
	case clean == IconFileName:
		return filepath.Join(root, IconFileName), nil
	case strings.HasPrefix(clean, ConfigsPrefix):
		relative := strings.TrimPrefix(clean, ConfigsPrefix)
		if err := validateConfigRelative(relative); err != nil {
			return "", err
		}
		return filepath.Join(root, filepath.FromSlash(relative)), nil
	case strings.HasPrefix(clean, ModsPrefix):
		relative := strings.TrimPrefix(clean, ModsPrefix)
		if filepath.Base(relative) != relative {
			return "", errs.NewError(errs.ErrPackageSecurity, "Package contains a mod file in a subdirectory")
		}
		if !isModFile(relative) {
			return "", errs.NewError(errs.ErrPackageSecurity, "Package contains an unsupported mod file type")
		}
		return filepath.Join(root, relative), nil
	default:
		return "", errs.NewError(errs.ErrPackageSecurity, "Package contains an unexpected file")
	}
}

func validateConfigRelative(relative string) error {
	if relative == "" {
		return errs.NewError(errs.ErrPackageSecurity, "Package contains an empty config path")
	}
	parts := strings.Split(relative, "/")
	if len(parts) > maxNestedConfigDepth {
		return errs.NewError(errs.ErrPackageSecurity, "Package config path is too deep")
	}
	base := parts[len(parts)-1]
	if base == "" || base == "." || base == ".." {
		return errs.NewError(errs.ErrPackageSecurity, "Package contains an unsafe config path")
	}
	for _, part := range parts {
		if part == ".." || part == "." || part == "" {
			return errs.NewError(errs.ErrPackageSecurity, "Package contains an unsafe config path")
		}
	}
	if strings.HasPrefix(strings.ToLower(base), ".waxlight") {
		return errs.NewError(errs.ErrPackageSecurity, "Package contains a launcher metadata file")
	}
	return nil
}

func isModFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".zip", ".cs", ".dll":
		return true
	default:
		return false
	}
}

func entryLimitMessage(name string, limit int64) error {
	return errs.NewError(errs.ErrPackageInvalid, fmt.Sprintf("Package entry %s exceeds the allowed size", name))
}
