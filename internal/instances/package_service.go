package instances

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/operations"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/versions"
)

// PackageService builds portable .waxlight packages from instances and
// installs validated packages as new, isolated instances. The archive format
// stays behind the PackageIO port.
type PackageService struct {
	repository PackageRepository
	creator    InstanceCreator
	versions   PackageVersionReader
	mods       PackageModStore
	catalog    PackageCatalog
	downloads  PackageDownloadedMods
	installer  CatalogModInstaller
	identity   ModIdentity
	io         PackageIO
	gate       MutationGate
	operations *operations.Manager
	events     Publisher
	remove     DirectoryRemover
	dataRoot   string
	now        Clock
	newID      IDGenerator
}

func NewPackageService(
	repository PackageRepository,
	creator InstanceCreator,
	versions PackageVersionReader,
	mods PackageModStore,
	catalog PackageCatalog,
	downloads PackageDownloadedMods,
	installer CatalogModInstaller,
	identity ModIdentity,
	io PackageIO,
	gate MutationGate,
	operationManager *operations.Manager,
	events Publisher,
	remove DirectoryRemover,
	dataRoot string,
	now Clock,
	newID IDGenerator,
) *PackageService {
	return &PackageService{
		repository: repository,
		creator:    creator,
		versions:   versions,
		mods:       mods,
		catalog:    catalog,
		downloads:  downloads,
		installer:  installer,
		identity:   identity,
		io:         io,
		gate:       gate,
		operations: operationManager,
		events:     events,
		remove:     remove,
		dataRoot:   dataRoot,
		now:        now,
		newID:      newID,
	}
}

// ExportInstance builds a portable .waxlight package describing instanceID.
// Catalog-managed mods are stored as references; every other installed mod is
// embedded in the archive. Game settings are sanitized so no authentication
// data leaves the machine, and save data, logs and caches are never included.
func (service *PackageService) ExportInstance(
	ctx context.Context,
	instanceID string,
	targetPath string,
	options ExportInstanceOptions,
) (PackageManifest, error) {
	release, err := service.beginMutation()
	if err != nil {
		return PackageManifest{}, err
	}
	defer release()
	instance, err := service.repository.GetInstance(ctx, instanceID)
	if err != nil {
		return PackageManifest{}, err
	}
	slog.Info("exporting instance package", "instance", instance.Name)
	version, err := service.versions.Get(ctx, instance.GameVersionID)
	if err != nil {
		return PackageManifest{}, err
	}

	mods, err := service.mods.ListMods(ctx, instanceID)
	if err != nil {
		return PackageManifest{}, err
	}

	name := instance.Name
	if strings.TrimSpace(options.Name) != "" {
		if name, err = cleanName(options.Name); err != nil {
			return PackageManifest{}, err
		}
	}
	manifest := PackageManifest{
		SchemaVersion:   InstancePackageSchemaVersion,
		Name:            name,
		Description:     strings.TrimSpace(options.Description),
		GameVersion:     PackageGameVersion{ID: version.ID, Name: version.Name},
		GameClient:      instance.GameClient,
		LaunchArguments: append([]string(nil), instance.LaunchArguments...),
		CreatedAt:       service.now().UTC(),
	}
	if manifest.Description == "" {
		manifest.Description = instance.Description
	}
	if manifest.GameVersion.Name == "" {
		manifest.GameVersion.Name = manifest.GameVersion.ID
	}

	embedded := make(map[string]string, len(mods))
	seenEmbedded := make(map[string]struct{}, len(mods))
	for _, mod := range mods {
		packageMod := PackageMod{
			Name:    mod.Name,
			Version: mod.Version,
			Enabled: mod.Enabled,
		}
		if modID, versionID, ok := service.identity.ParseModDBSource(mod.Source); ok {
			packageMod.Source = PackageModSourceCatalog
			packageMod.ModID = modID
			packageMod.VersionID = versionID
			packageMod.FileName = mod.FileName
			if service.downloads != nil {
				if downloaded, getErr := service.downloads.Get(ctx, modID, versionID); getErr == nil {
					packageMod.Checksum = downloaded.Checksum
					packageMod.DownloadURL = downloaded.DownloadURL
					if packageMod.FileName == "" {
						packageMod.FileName = downloaded.FileName
					}
				}
			}
		} else {
			packageMod.Source = PackageModSourceEmbedded
			packageMod.FileName = uniqueEmbeddedName(mod.FilePath, seenEmbedded)
			seenEmbedded[packageMod.FileName] = struct{}{}
			checksum, sumErr := sha256File(mod.FilePath)
			if sumErr != nil {
				return PackageManifest{}, sumErr
			}
			packageMod.Checksum = checksum
			embedded[packageMod.FileName] = mod.FilePath
		}
		manifest.Mods = append(manifest.Mods, packageMod)
	}

	configFiles, err := collectInstanceConfigs(instance.Directory)
	if err != nil {
		return PackageManifest{}, err
	}
	manifest.ConfigFiles = configFiles

	iconPath := ""
	if instance.CoverPath != nil && *instance.CoverPath != "" {
		if info, statErr := os.Lstat(*instance.CoverPath); statErr == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			manifest.HasIcon = true
			iconPath = *instance.CoverPath
		}
	}

	if err := service.io.Write(ctx, targetPath, PackageWriteSource{
		Manifest:     manifest,
		InstanceDir:  instance.Directory,
		EmbeddedMods: embedded,
		IconPath:     iconPath,
	}); err != nil {
		return PackageManifest{}, err
	}
	return manifest, nil
}

func uniqueEmbeddedName(name string, seen map[string]struct{}) string {
	base := filepath.Base(name)
	if _, exists := seen[base]; !exists {
		return base
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for index := 2; ; index++ {
		candidate := stem + "-" + strconv.Itoa(index) + ext
		if _, exists := seen[candidate]; !exists {
			return candidate
		}
	}
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func collectInstanceConfigs(instanceDir string) ([]string, error) {
	var files []string
	rootSettings := filepath.Join(instanceDir, "clientsettings.json")
	if info, err := os.Lstat(rootSettings); err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
		files = append(files, "clientsettings.json")
	}
	configDir := filepath.Join(instanceDir, "Config")
	subdir, err := os.ReadDir(configDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return files, nil
		}
		return nil, err
	}
	for _, entry := range subdir {
		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil, infoErr
		}
		if info.IsDir() {
			nested, walkErr := walkConfigDirectory(configDir, entry.Name())
			if walkErr != nil {
				return nil, walkErr
			}
			files = append(files, nested...)
			continue
		}
		if info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 &&
			!strings.HasPrefix(entry.Name(), ".waxlight") {
			files = append(files, filepath.Join("Config", entry.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

func walkConfigDirectory(root string, name string) ([]string, error) {
	var files []string
	base := filepath.Join(root, name)
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil || strings.HasPrefix(relative, "..") {
			return errs.NewError(errs.ErrValidation, "Config file escapes the instance directory")
		}
		if strings.Contains(relative, string(os.PathSeparator)+".waxlight") || strings.HasPrefix(filepath.Base(relative), ".waxlight") {
			return nil
		}
		files = append(files, relative)
		return nil
	})
	return files, err
}

// InspectPackage validates a package and reports how it would be imported,
// without modifying anything.
func (service *PackageService) InspectPackage(ctx context.Context, packagePath string) (PackageInspection, error) {
	packagePath = strings.TrimSpace(packagePath)
	if packagePath == "" {
		return PackageInspection{}, errs.NewError(errs.ErrValidation, "Select a package file")
	}
	pkg, err := service.io.Open(packagePath)
	if err != nil {
		return PackageInspection{}, err
	}

	inspection := PackageInspection{
		Path:            packagePath,
		SchemaVersion:   pkg.Manifest().SchemaVersion,
		Name:            pkg.Manifest().Name,
		Description:     pkg.Manifest().Description,
		Author:          pkg.Manifest().Author,
		GameVersion:     pkg.Manifest().GameVersion,
		LaunchArguments: append([]string(nil), pkg.Manifest().LaunchArguments...),
		ConfigFiles:     append([]string(nil), pkg.Manifest().ConfigFiles...),
		HasIcon:         pkg.Manifest().HasIcon,
		TotalSize:       pkg.TotalSize(),
	}

	inspection.VersionStatus = service.packageVersionStatus(ctx, pkg.Manifest().GameVersion)

	for _, mod := range pkg.Manifest().Mods {
		check := PackageModCheck{
			ModID:     mod.ModID,
			VersionID: mod.VersionID,
			Name:      mod.Name,
			Version:   mod.Version,
			Source:    mod.Source,
			Enabled:   mod.Enabled,
		}
		switch mod.Source {
		case PackageModSourceEmbedded:
			check.Status = PackageModEmbedded
			if mod.Checksum == "" {
				inspection.UnverifiedFiles++
			}
		default:
			status, message := service.checkCatalogMod(ctx, mod, pkg.Manifest().GameVersion)
			check.Status = status
			check.Message = message
			if status == PackageModMissing && message == "" {
				inspection.Warnings = append(inspection.Warnings, "Mod "+mod.Name+" could not be resolved in the mod catalog")
			}
			if status == PackageModIncompatible {
				inspection.Warnings = append(inspection.Warnings, "Mod "+mod.Name+" is not compatible with Vintage Story "+pkg.Manifest().GameVersion.Name)
			}
		}
		inspection.Mods = append(inspection.Mods, check)
	}
	if inspection.UnverifiedFiles > 0 {
		inspection.Warnings = append(inspection.Warnings, "The package contains mod files that cannot be verified")
	}
	if inspection.VersionStatus == PackageVersionMissing {
		inspection.Warnings = append(inspection.Warnings, "The required game version is not available")
	}
	return inspection, nil
}

func (service *PackageService) checkCatalogMod(
	ctx context.Context,
	mod PackageMod,
	gameVersion PackageGameVersion,
) (PackageModStatus, string) {
	if service.catalog == nil {
		return PackageModMissing, "The mod catalog is unavailable"
	}
	details, err := service.catalog.Get(ctx, mod.ModID)
	if err != nil {
		return PackageModMissing, "This mod was not found in the mod catalog"
	}
	selected, ok := service.identity.FindModVersion(details.Versions, mod.VersionID)
	if !ok {
		return PackageModMissing, "The requested mod version is no longer available"
	}
	if !service.identity.ModSupportsVersion(selected.GameVersions, gameVersion.Name) {
		return PackageModIncompatible, "This mod version does not support the required game version"
	}
	return PackageModAvailable, ""
}

func (service *PackageService) packageVersionStatus(
	ctx context.Context,
	required PackageGameVersion,
) PackageVersionStatus {
	if _, ok := service.findInstalledVersion(ctx, required); ok {
		return PackageVersionInstalled
	}
	available, err := service.versions.ListAvailable(ctx)
	if err != nil {
		return PackageVersionMissing
	}
	for _, version := range available {
		if version.ID == required.ID || version.Name == required.Name || (required.Name != "" && version.ID == required.Name) {
			return PackageVersionAvailable
		}
	}
	return PackageVersionMissing
}

func (service *PackageService) findInstalledVersion(
	ctx context.Context,
	required PackageGameVersion,
) (versions.GameVersion, bool) {
	installed, err := service.versions.List(ctx)
	if err != nil {
		return versions.GameVersion{}, false
	}
	for _, version := range installed {
		if version.ID == required.ID {
			return version, true
		}
	}
	for _, version := range installed {
		if version.Name != "" && version.Name == required.Name {
			return version, true
		}
	}
	return versions.GameVersion{}, false
}

const importOperationTitle = "operation_importing_instance"

// StartImport starts an instance package import as a cancellable operation.
func (service *PackageService) StartImport(
	ctx context.Context,
	packagePath string,
	options ImportInstanceOptions,
) (operations.Operation, error) {
	if service.operations == nil {
		return operations.Operation{}, errs.NewError(errs.ErrValidation, "Instance imports are not configured")
	}
	packagePath = strings.TrimSpace(packagePath)
	if packagePath == "" {
		return operations.Operation{}, errs.NewError(errs.ErrValidation, "Select a package file")
	}
	now := service.now().UTC()
	operation := operations.Operation{
		ID: service.newID(), Type: "instance_import",
		Title: "Importing instance", TitleKey: importOperationTitle,
		Status: operations.StatusQueued, CreatedAt: now,
	}
	_, err := operations.Start(service.operations, ctx, operation, "instance-import", func(workerCtx context.Context) (ImportReport, error) {
		started := service.now().UTC()
		operation.Status, operation.StartedAt = operations.StatusRunning, &started
		service.operations.SaveBestEffort(operation, operations.EventUpdated)

		report, importErr := service.importPackage(workerCtx, packagePath, options, func(progress float64) {
			operation.Progress = progress
			service.operations.Publish(operations.EventProgress, operation)
			service.operations.Persist(operation)
		})
		if errors.Is(importErr, context.Canceled) {
			return report, service.cancelImport(operation)
		}
		if importErr != nil {
			finished := service.now().UTC()
			operation.Status, operation.FinishedAt = operations.StatusFailed, &finished
			code, message := errs.ErrValidation, importErr.Error()
			operation.ErrorCode, operation.ErrorMessage = &code, &message
			service.operations.SaveBestEffort(operation, operations.EventFailed)
			return report, importErr
		}
		finished := service.now().UTC()
		operation.Status, operation.Progress, operation.FinishedAt = operations.StatusCompleted, 1, &finished
		service.operations.SaveBestEffort(operation, operations.EventCompleted)
		return report, nil
	})
	if err != nil {
		return operations.Operation{}, err
	}
	return operation, nil
}

func (service *PackageService) cancelImport(operation operations.Operation) error {
	finished := service.now().UTC()
	operation.Status, operation.FinishedAt = operations.StatusCancelled, &finished
	if err := service.operations.Save(context.Background(), operation, ""); err != nil {
		return err
	}
	if err := service.operations.Delete(context.Background(), operation.ID); err != nil {
		return err
	}
	service.operations.Publish(operations.EventRemoved, map[string]string{"id": operation.ID})
	return context.Canceled
}

// ImportPackage installs a validated package as a new, isolated instance.
// Existing instances are never modified or overwritten.
func (service *PackageService) ImportPackage(
	ctx context.Context,
	packagePath string,
	options ImportInstanceOptions,
) (ImportReport, error) {
	return service.importPackage(ctx, packagePath, options, nil)
}

func (service *PackageService) importPackage(
	ctx context.Context,
	packagePath string,
	options ImportInstanceOptions,
	setProgress func(float64),
) (report ImportReport, err error) {
	packagePath = strings.TrimSpace(packagePath)
	if packagePath == "" {
		return ImportReport{}, errs.NewError(errs.ErrValidation, "Select a package file")
	}
	slog.Info("importing instance package", "package", packagePath)
	pkg, err := service.io.Open(packagePath)
	if err != nil {
		return ImportReport{}, err
	}
	service.setImportProgress(setProgress, 0.1)

	versionID, err := service.resolveImportVersion(ctx, pkg.Manifest().GameVersion, options)
	if err != nil {
		return ImportReport{}, err
	}
	service.setImportProgress(setProgress, 0.25)

	release, err := service.beginMutation()
	if err != nil {
		return ImportReport{}, err
	}
	defer release()

	name := strings.TrimSpace(options.Name)
	if name == "" {
		name = pkg.Manifest().Name
	}
	description := strings.TrimSpace(options.Description)
	if description == "" {
		description = pkg.Manifest().Description
	}
	gameClient := pkg.Manifest().GameClient
	instance, err := service.creator.Create(ctx, CreateInput{
		Name:            name,
		Description:     description,
		GameVersionID:   versionID,
		GameClient:      gameClient,
		Directory:       strings.TrimSpace(options.Directory),
		LaunchArguments: append([]string(nil), pkg.Manifest().LaunchArguments...),
	})
	if err != nil {
		return ImportReport{}, err
	}

	report = ImportReport{
		InstanceID:    instance.ID,
		InstanceName:  instance.Name,
		GameVersionID: versionID,
		Mods:          []ImportedModResult{},
	}
	var downloadedMods []mods.DownloadedMod
	cleanup := true
	defer func() {
		if cleanup && err != nil {
			_ = service.cleanupFailedImport(context.Background(), instance)
			if service.installer != nil {
				if cleanupErr := service.installer.RemoveDownloadedModsIfUnused(context.Background(), downloadedMods); cleanupErr != nil {
					slog.Warn("could not remove downloads from the failed import", "instance", instance.Name, "error", cleanupErr)
				}
			}
		}
	}()
	service.setImportProgress(setProgress, 0.4)

	if err := pkg.ExtractConfigs(ctx, instance.Directory); err != nil {
		return report, err
	}
	service.setImportProgress(setProgress, 0.65)

	if pkg.Manifest().HasIcon {
		iconPath := filepath.Join(instance.Directory, ".waxlight-cover.png")
		if err := pkg.ExtractIcon(ctx, iconPath); err != nil {
			return report, err
		}
		cover := iconPath
		instance.CoverPath = &cover
		instance.UpdatedAt = service.now().UTC()
		if err := service.repository.SaveInstance(ctx, instance); err != nil {
			return report, err
		}
	}

	mods := pkg.Manifest().Mods
	for index, mod := range mods {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		result := service.installPackageMod(ctx, pkg, mod, instance, options.AllowIncompatible, &downloadedMods)
		report.Mods = append(report.Mods, result)
		if err := ctx.Err(); err != nil {
			return report, err
		}
		service.setImportProgress(setProgress, 0.65+0.3*float64(index+1)/float64(len(mods)))
	}
	report.Warnings = service.packageImportWarnings(report)
	cleanup = false
	return report, nil
}

func (service *PackageService) setImportProgress(setProgress func(float64), progress float64) {
	if setProgress != nil {
		setProgress(progress)
	}
}

func (service *PackageService) cleanupFailedImport(ctx context.Context, instance Instance) error {
	if err := service.remove(instance.Directory); err != nil {
		slog.Warn("could not remove the failed import directory", "instance", instance.Name, "error", err)
		return err
	}
	if err := service.repository.DeleteInstance(ctx, instance.ID); err != nil {
		slog.Warn("could not delete the failed import record", "instance", instance.Name, "error", err)
		return err
	}
	return nil
}

func (service *PackageService) installPackageMod(
	ctx context.Context,
	pkg PackageArchive,
	mod PackageMod,
	instance Instance,
	allowIncompatible bool,
	downloadedMods *[]mods.DownloadedMod,
) ImportedModResult {
	result := ImportedModResult{Name: mod.Name, Version: mod.Version}
	switch mod.Source {
	case PackageModSourceEmbedded:
		modsDirectory := filepath.Join(instance.Directory, modsDirectoryFor(mod.Enabled))
		if err := pkg.ExtractEmbeddedMod(ctx, mod.FileName, modsDirectory); err != nil {
			result.Status = "failed"
			result.Message = err.Error()
			return result
		}
		now := service.now().UTC()
		installed := mods.InstalledMod{
			ID:          service.newID(),
			InstanceID:  instance.ID,
			Name:        mod.Name,
			Version:     mod.Version,
			FileName:    mod.FileName,
			FilePath:    filepath.Join(modsDirectory, mod.FileName),
			Enabled:     mod.Enabled,
			Managed:     false,
			Source:      "local",
			InstalledAt: now,
			UpdatedAt:   now,
		}
		if err := service.mods.SaveMod(ctx, installed); err != nil {
			result.Status = "failed"
			result.Message = "Installed the file but could not save its metadata"
			return result
		}
		result.Status = "installed"
		return result

	default:
		return service.installCatalogPackageMod(ctx, mod, instance, allowIncompatible, downloadedMods)
	}
}

func modsDirectoryFor(enabled bool) string {
	if enabled {
		return "Mods"
	}
	return "ModsDisabled"
}

func (service *PackageService) installCatalogPackageMod(
	ctx context.Context,
	mod PackageMod,
	instance Instance,
	allowIncompatible bool,
	downloadedMods *[]mods.DownloadedMod,
) ImportedModResult {
	result := ImportedModResult{Name: mod.Name, Version: mod.Version}
	if service.installer == nil {
		result.Status = "skipped"
		result.Message = "The mod catalog is unavailable"
		return result
	}
	installResult, err := service.installer.DownloadCatalogMod(ctx, mods.DownloadModRequest{
		ModID:             mod.ModID,
		VersionID:         mod.VersionID,
		InstanceIDs:       []string{instance.ID},
		AllowIncompatible: allowIncompatible,
	})
	*downloadedMods = append(*downloadedMods, installResult.DownloadedNow...)
	if err != nil {
		result.Status = "skipped"
		result.Message = friendlyPackageModError(err)
		return result
	}
	if len(installResult.Installations) == 1 && !installResult.Installations[0].Installed {
		result.Status = "skipped"
		result.Message = installResult.Installations[0].Message
		return result
	}
	if !mod.Enabled {
		if err := service.disableInstalledCatalogMod(ctx, mod, instance.ID); err != nil {
			result.Status = "installed"
			result.Message = "Installed but could not disable the mod"
			return result
		}
	}
	result.Status = "installed"
	return result
}

func friendlyPackageModError(err error) string {
	var appError *errs.AppError
	if errors.As(err, &appError) {
		if appError.Code == mods.ErrModIncompatible {
			return "This mod is not compatible with the selected game version"
		}
		if appError.Code == mods.ErrModVersionNotFound || appError.Code == mods.ErrModNotFound {
			return "The mod or its version is no longer available in the catalog"
		}
		if appError.Code == mods.ErrModCatalog {
			return "The mod catalog is unavailable"
		}
		return appError.Message
	}
	return "Could not download the mod"
}

func (service *PackageService) disableInstalledCatalogMod(ctx context.Context, mod PackageMod, instanceID string) error {
	mods, err := service.mods.ListMods(ctx, instanceID)
	if err != nil {
		return err
	}
	expected := service.identity.ModDBSource(mod.ModID, mod.VersionID)
	for _, installed := range mods {
		if installed.Source == expected {
			_, err := service.installer.SetModEnabled(ctx, installed.ID, false)
			return err
		}
	}
	return nil
}

func (service *PackageService) packageImportWarnings(report ImportReport) []string {
	var warnings []string
	for _, mod := range report.Mods {
		if mod.Status == "skipped" && mod.Message != "" {
			warnings = append(warnings, mod.Name+": "+mod.Message)
		}
	}
	return warnings
}

// resolveImportVersion decides which installed game version the new instance
// uses. The manifest version is preferred; a caller-supplied override wins; an
// explicitly requested install can be performed through the normal version
// pipeline before the instance is created.
func (service *PackageService) resolveImportVersion(
	ctx context.Context,
	required PackageGameVersion,
	options ImportInstanceOptions,
) (string, error) {
	override := strings.TrimSpace(options.GameVersionID)
	if override != "" {
		if _, err := service.versions.Get(ctx, override); err != nil {
			return "", err
		}
		return override, nil
	}
	if version, ok := service.findInstalledVersion(ctx, required); ok {
		return version.ID, nil
	}
	if options.InstallVersion {
		available, err := service.versions.ListAvailable(ctx)
		if err != nil {
			return "", err
		}
		var catalogVersion *versions.AvailableGameVersion
		for index := range available {
			if available[index].ID == required.ID || (required.Name != "" && available[index].ID == required.Name) {
				catalogVersion = &available[index]
				break
			}
		}
		if catalogVersion == nil {
			for index := range available {
				if available[index].Name == required.Name {
					catalogVersion = &available[index]
					break
				}
			}
		}
		if catalogVersion == nil {
			return "", errs.NewError(errs.ErrVersionNotFound, "The required game version is not available")
		}
		if _, err := service.versions.InstallCatalogAndWait(ctx, catalogVersion.ID); err != nil {
			return "", err
		}
		return catalogVersion.ID, nil
	}
	return "", errs.NewError(
		errs.ErrVersionNotInstalled,
		"Vintage Story "+required.Name+" is not installed",
	)
}

func (service *PackageService) beginMutation() (func(), error) {
	if err := service.gate.Begin(); err != nil {
		return nil, err
	}
	return service.gate.End, nil
}

// ModIdentity resolves the persisted mod source format and catalog
// compatibility during package inspection and import. It stays a narrow port
// until the mods feature owns this behavior.
type ModIdentity interface {
	ParseModDBSource(source string) (modID, versionID string, ok bool)
	ModDBSource(modID, versionID string) string
	FindModVersion(versions []mods.ModVersion, id string) (mods.ModVersion, bool)
	ModSupportsVersion(versions []string, requested string) bool
}
