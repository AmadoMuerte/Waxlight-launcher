package instancepackage

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/platform/filesystem"
)

// WriteSource describes everything needed to produce a package.
type WriteSource struct {
	Manifest     domain.PackageManifest
	InstanceDir  string
	EmbeddedMods map[string]string
	IconPath     string
}

// Write creates a .waxlight archive at targetPath. Config files are read from
// InstanceDir, with client settings sanitized before they are embedded. Mod
// files referenced by EmbeddedMods (fileName to source path) are copied into
// the archive as-is.
func Write(ctx context.Context, targetPath string, source WriteSource) error {
	targetPath = filepath.Clean(targetPath)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(targetPath+".partial", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(output)

	writeEntry := func(name string, contents []byte) error {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o644)
		entry, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = entry.Write(contents)
		return err
	}

	encoded, err := json.MarshalIndent(source.Manifest, "", "  ")
	if err != nil {
		abortPackageWrite(archive, output, targetPath)
		return err
	}
	encoded = append(encoded, '\n')
	if err := writeEntry(ManifestFileName, encoded); err != nil {
		abortPackageWrite(archive, output, targetPath)
		return err
	}

	for _, relative := range source.Manifest.ConfigFiles {
		if err := ctx.Err(); err != nil {
			abortPackageWrite(archive, output, targetPath)
			return err
		}
		contents, err := readConfigFile(source.InstanceDir, relative)
		if err != nil {
			abortPackageWrite(archive, output, targetPath)
			return err
		}
		if err := writeEntry(ConfigsPrefix+filepath.ToSlash(relative), contents); err != nil {
			abortPackageWrite(archive, output, targetPath)
			return err
		}
	}

	for fileName, sourcePath := range source.EmbeddedMods {
		if err := ctx.Err(); err != nil {
			abortPackageWrite(archive, output, targetPath)
			return err
		}
		if filepath.Base(fileName) != fileName {
			abortPackageWrite(archive, output, targetPath)
			return domain.NewError(domain.ErrValidation, "Embedded mod name is not a plain file name")
		}
		contents, err := readEmbeddedMod(sourcePath)
		if err != nil {
			abortPackageWrite(archive, output, targetPath)
			return err
		}
		if err := writeEntry(ModsPrefix+fileName, contents); err != nil {
			abortPackageWrite(archive, output, targetPath)
			return err
		}
	}

	if source.Manifest.HasIcon && source.IconPath != "" {
		contents, err := readIcon(source.IconPath)
		if err != nil {
			abortPackageWrite(archive, output, targetPath)
			return err
		}
		if err := writeEntry(IconFileName, contents); err != nil {
			abortPackageWrite(archive, output, targetPath)
			return err
		}
	}

	if err := archive.Close(); err != nil {
		abortPackageWrite(archive, output, targetPath)
		return err
	}
	if err := output.Close(); err != nil {
		if removeErr := os.Remove(targetPath + ".partial"); removeErr != nil {
			slog.Debug("could not remove the partial package", "target", targetPath, "error", removeErr)
		}
		return err
	}
	return os.Rename(targetPath+".partial", targetPath)
}

// abortPackageWrite best-effort closes the archive and removes the partial
// package after a write failure. The primary error is already returned by the
// caller, so cleanup failures are logged at debug level only.
func abortPackageWrite(archive *zip.Writer, output *os.File, targetPath string) {
	if err := archive.Close(); err != nil {
		slog.Debug("could not close the failed package archive", "target", targetPath, "error", err)
	}
	if err := output.Close(); err != nil {
		slog.Debug("could not close the failed package file", "target", targetPath, "error", err)
	}
	if err := os.Remove(targetPath + ".partial"); err != nil {
		slog.Debug("could not remove the partial package", "target", targetPath, "error", err)
	}
}

func readConfigFile(instanceDir string, relative string) ([]byte, error) {
	clean, err := CleanArchiveName(relative)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(clean, ConfigsPrefix) || strings.HasPrefix(clean, ModsPrefix) || clean == ManifestFileName || clean == IconFileName {
		return nil, domain.NewError(domain.ErrValidation, "Unsafe config path in package")
	}
	path, err := safeJoin(instanceDir, filepath.FromSlash(clean))
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("config file is unavailable: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, domain.NewError(domain.ErrValidation, "Config path is not a regular file")
	}
	if info.Size() > maxConfigBytes {
		return nil, domain.NewError(domain.ErrValidation, "Config file is too large to export")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Base(clean), "clientsettings.json") {
		sanitized, err := filesystem.SanitizeClientSettings(contents)
		if err != nil {
			return nil, domain.NewError(domain.ErrValidation, "Could not sanitize client settings for export")
		}
		return sanitized, nil
	}
	return contents, nil
}

func readEmbeddedMod(sourcePath string) ([]byte, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, domain.NewError(domain.ErrValidation, "Mod source is not a regular file")
	}
	if info.Size() > maxEmbeddedModBytes {
		return nil, domain.NewError(domain.ErrValidation, "Mod file is too large to embed in the package")
	}
	return os.ReadFile(sourcePath)
}

func readIcon(iconPath string) ([]byte, error) {
	info, err := os.Stat(iconPath)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, domain.NewError(domain.ErrValidation, "Icon path is not a regular file")
	}
	if info.Size() > maxIconBytes {
		return nil, domain.NewError(domain.ErrValidation, "Icon file is too large to export")
	}
	return os.ReadFile(iconPath)
}

func safeJoin(root string, relative string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	joined := filepath.Join(absRoot, relative)
	if joined != absRoot && !strings.HasPrefix(joined, absRoot+string(os.PathSeparator)) {
		return "", errors.New("path escapes the instance directory")
	}
	return joined, nil
}
