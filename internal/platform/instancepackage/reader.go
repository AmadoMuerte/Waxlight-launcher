package instancepackage

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/atomicfile"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/filesystem"
)

// Package is a validated read-only view of a .waxlight archive.
type Package struct {
	Path     string
	Manifest instances.PackageManifest
	Entries  map[string]int64
	IconSize int64
}

// Open validates the archive format, manifest schema and every entry path and
// size limit without extracting anything to disk.
func Open(path string) (*Package, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, errs.NewError(errs.ErrPackageInvalid, "The file is not a valid Waxlight package")
	}
	defer archive.Close()

	manifest, err := readManifest(archive)
	if err != nil {
		return nil, err
	}

	result := &Package{
		Path:     path,
		Manifest: manifest,
		Entries:  make(map[string]int64, len(archive.File)),
	}
	if len(archive.File) > maxEntryCount {
		return nil, errs.NewError(errs.ErrPackageInvalid, "The package contains too many files")
	}

	var totalUncompressed int64
	seenManifest := false
	for _, file := range archive.File {
		clean, cleanErr := CleanArchiveName(file.Name)
		if cleanErr != nil {
			return nil, cleanErr
		}
		if clean == ManifestFileName {
			seenManifest = true
			continue
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return nil, errs.NewError(errs.ErrPackageSecurity, "Package contains a symbolic link")
		}
		if file.Mode()&os.ModeType != 0 {
			return nil, errs.NewError(errs.ErrPackageSecurity, "Package contains a special file")
		}

		if _, err := ValidateEntryName(clean, ""); err != nil {
			return nil, err
		}

		size := file.UncompressedSize64
		var limit int64
		switch {
		case clean == IconFileName:
			limit = maxIconBytes
			result.IconSize = int64(size)
		case strings.HasPrefix(clean, ConfigsPrefix):
			limit = maxConfigBytes
		case strings.HasPrefix(clean, ModsPrefix):
			limit = maxEmbeddedModBytes
		}
		if int64(size) > limit {
			return nil, entryLimitMessage(clean, limit)
		}
		totalUncompressed += int64(size)
		if totalUncompressed > maxTotalUncompressed {
			return nil, errs.NewError(errs.ErrPackageInvalid, "The package is too large to import")
		}
		result.Entries[clean] = int64(size)
	}

	if !seenManifest {
		return nil, errs.NewError(errs.ErrPackageInvalid, "The package does not contain a manifest")
	}
	if _, ok := result.Entries[IconFileName]; !ok && manifest.HasIcon {
		return nil, errs.NewError(errs.ErrPackageInvalid, "The package declares an icon that is missing")
	}
	if err := validateManifestReferencedEntries(manifest, result.Entries); err != nil {
		return nil, err
	}
	return result, nil
}

func readManifest(archive *zip.ReadCloser) (instances.PackageManifest, error) {
	var raw []byte
	for _, file := range archive.File {
		clean, err := CleanArchiveName(file.Name)
		if err != nil || clean != ManifestFileName {
			continue
		}
		file, err := file.Open()
		if err != nil {
			return instances.PackageManifest{}, errs.NewError(errs.ErrPackageInvalid, "Could not read the package manifest")
		}
		defer file.Close()
		raw, err = io.ReadAll(io.LimitReader(file, maxManifestBytes+1))
		if err != nil {
			return instances.PackageManifest{}, errs.NewError(errs.ErrPackageInvalid, "Could not read the package manifest")
		}
		break
	}
	if len(raw) == 0 {
		return instances.PackageManifest{}, errs.NewError(errs.ErrPackageInvalid, "The package does not contain a manifest")
	}
	if len(raw) > maxManifestBytes {
		return instances.PackageManifest{}, errs.NewError(errs.ErrPackageInvalid, "The package manifest is too large")
	}

	var manifest instances.PackageManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return instances.PackageManifest{}, errs.NewError(errs.ErrPackageInvalid, "The package manifest is not valid JSON")
	}
	if manifest.SchemaVersion == 0 {
		return instances.PackageManifest{}, errs.NewError(errs.ErrPackageUnsupported, "The package does not declare a format version")
	}
	if manifest.SchemaVersion < instances.InstancePackageLegacySchemaVersion || manifest.SchemaVersion > instances.InstancePackageSchemaVersion {
		return instances.PackageManifest{}, errs.NewError(
			errs.ErrPackageUnsupported,
			fmt.Sprintf("The package format version %d is not supported by this launcher", manifest.SchemaVersion),
		)
	}
	if manifest.SchemaVersion == instances.InstancePackageLegacySchemaVersion {
		manifest.GameClient = instances.GameClientVanilla
	} else if normalized, valid := instances.NormalizeGameClient(manifest.GameClient); !valid {
		return instances.PackageManifest{}, errs.NewError(errs.ErrPackageInvalid, "The package manifest declares an invalid game client")
	} else {
		manifest.GameClient = normalized
	}
	if strings.TrimSpace(manifest.Name) == "" {
		return instances.PackageManifest{}, errs.NewError(errs.ErrPackageInvalid, "The package manifest does not name the instance")
	}
	if strings.TrimSpace(manifest.GameVersion.ID) == "" && strings.TrimSpace(manifest.GameVersion.Name) == "" {
		return instances.PackageManifest{}, errs.NewError(errs.ErrPackageInvalid, "The package manifest does not declare a game version")
	}
	return manifest, nil
}

func validateManifestReferencedEntries(manifest instances.PackageManifest, entries map[string]int64) error {
	seenConfig := make(map[string]struct{}, len(manifest.ConfigFiles))
	for _, relative := range manifest.ConfigFiles {
		clean, err := CleanArchiveName(relative)
		if err != nil {
			return errs.NewError(errs.ErrPackageInvalid, "The package manifest references an unsafe config path")
		}
		if _, duplicate := seenConfig[clean]; duplicate {
			return errs.NewError(errs.ErrPackageInvalid, "The package manifest references a config path twice")
		}
		seenConfig[clean] = struct{}{}
		if _, ok := entries[ConfigsPrefix+clean]; !ok {
			return errs.NewError(errs.ErrPackageInvalid, "The package manifest references a config file that is missing from the archive")
		}
	}
	for _, mod := range manifest.Mods {
		if mod.Source != instances.PackageModSourceEmbedded {
			continue
		}
		if strings.TrimSpace(mod.ArchivePath) == "" && strings.TrimSpace(mod.FileName) == "" {
			return errs.NewError(errs.ErrPackageInvalid, "The package manifest references an embedded mod without a file")
		}
		clean, err := CleanArchiveName(modsArchiveName(mod))
		if err != nil {
			return errs.NewError(errs.ErrPackageInvalid, "The package manifest references an unsafe embedded mod path")
		}
		if _, ok := entries[clean]; !ok {
			return errs.NewError(errs.ErrPackageInvalid, "The package manifest references an embedded mod that is missing from the archive")
		}
	}
	return nil
}

// ExtractConfigs safely writes every config entry into targetDir. Config files
// are written relative to the instance root, mirroring the original layout.
func (p *Package) ExtractConfigs(ctx context.Context, targetDir string) error {
	archive, err := zip.OpenReader(p.Path)
	if err != nil {
		return err
	}
	defer archive.Close()

	written := make(map[string]struct{}, len(p.Manifest.ConfigFiles))
	for _, file := range archive.File {
		clean, cleanErr := CleanArchiveName(file.Name)
		if cleanErr != nil || !strings.HasPrefix(clean, ConfigsPrefix) || file.FileInfo().IsDir() {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		destination, err := ValidateEntryName(clean, targetDir)
		if err != nil {
			return err
		}
		if err := ensureInside(targetDir, destination); err != nil {
			return err
		}
		relative := strings.TrimPrefix(clean, ConfigsPrefix)
		if _, referenced := p.Manifest.ConfigFileSet()[relative]; !referenced {
			continue
		}
		if err := extractEntry(file, destination, maxConfigBytes); err != nil {
			return err
		}
		// Packages created by older launchers may carry machine-specific mod
		// paths or authentication fields inside client settings. Strip them
		// again on import so the game discovers mods in the new instance.
		if strings.EqualFold(filepath.Base(destination), "clientsettings.json") {
			if err := sanitizeClientSettingsFile(destination); err != nil {
				return err
			}
		}
		written[relative] = struct{}{}
	}
	for _, relative := range p.Manifest.ConfigFiles {
		if _, ok := written[relative]; !ok {
			return errs.NewError(errs.ErrPackageInvalid, "A declared config file could not be extracted")
		}
	}
	return nil
}

// ExtractEmbeddedMod writes one embedded mod file into instanceDirectory. The
// caller chooses Mods/ or ModsDisabled/ through the enabled state.
func (p *Package) ExtractEmbeddedMod(ctx context.Context, fileName string, directory string) error {
	clean, err := CleanArchiveName(ModsPrefix + fileName)
	if err != nil {
		return errs.NewError(errs.ErrPackageInvalid, "The package contains an unsafe embedded mod path")
	}
	archive, err := zip.OpenReader(p.Path)
	if err != nil {
		return err
	}
	defer archive.Close()
	for _, file := range archive.File {
		entry, entryErr := CleanArchiveName(file.Name)
		if entryErr != nil || entry != clean || file.FileInfo().IsDir() {
			continue
		}
		destination := filepath.Join(directory, fileName)
		if err := ensureInside(directory, destination); err != nil {
			return err
		}
		return extractEntry(file, destination, maxEmbeddedModBytes)
	}
	return errs.NewError(errs.ErrPackageInvalid, "The embedded mod file could not be found in the package")
}

func (p *Package) ExtractIcon(ctx context.Context, destination string) error {
	if !p.Manifest.HasIcon {
		return nil
	}
	archive, err := zip.OpenReader(p.Path)
	if err != nil {
		return err
	}
	defer archive.Close()
	for _, file := range archive.File {
		clean, cleanErr := CleanArchiveName(file.Name)
		if cleanErr != nil || clean != IconFileName || file.FileInfo().IsDir() {
			continue
		}
		if err := ensureInside(filepath.Dir(destination), destination); err != nil {
			return err
		}
		return extractEntry(file, destination, maxIconBytes)
	}
	return errs.NewError(errs.ErrPackageInvalid, "The declared package icon could not be found")
}

func extractEntry(file *zip.File, destination string, maxBytes int64) error {
	source, err := file.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(source, maxBytes+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > maxBytes {
		return entryLimitMessage(file.Name, maxBytes)
	}
	return atomicfile.Write(destination, data, 0o600)
}

func ensureInside(root string, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return errs.NewError(errs.ErrPackageSecurity, "Package file would escape the instance directory")
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return errs.NewError(errs.ErrPackageSecurity, "Package file would escape the instance directory")
	}
	return nil
}

func sanitizeClientSettingsFile(path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sanitized, err := filesystem.SanitizeClientSettings(contents)
	if err != nil {
		return errs.NewError(errs.ErrPackageInvalid, "The package contains invalid client settings")
	}
	return atomicfile.Write(path, sanitized, 0o600)
}

func modsArchiveName(mod instances.PackageMod) string {
	name := mod.ArchivePath
	if name == "" {
		name = ModsPrefix + mod.FileName
	}
	return name
}
