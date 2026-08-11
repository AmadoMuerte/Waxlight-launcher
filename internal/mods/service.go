package mods

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/operations"
	"github.com/waxlight/waxlight-launcher/internal/snapshots"
)

// operationTitleInstallingMod is the i18n key of the mod-install operation
// title.
const operationTitleInstallingMod = "operation_installing_mod"

// Service owns the installed-mod lifecycle of one instance: discovery and
// reconciliation, local file installation, enable/disable, removal with
// dependency previews, and binding local files to their catalog entries.
type Service struct {
	repository  Repository
	files       FileManager
	catalog     Catalog
	downloads   DownloadedStore
	operations  *operations.Manager
	gate        MutationGate
	lock        InstanceLock
	snapshotter snapshots.SafetySnapshotter
	events      Publisher
	telemetry   Telemetry
	now         Clock
	newID       IDGenerator
}

// NewService wires the installed-mod service with immutable dependencies.
func NewService(
	repository Repository,
	files FileManager,
	catalog Catalog,
	downloads DownloadedStore,
	operationManager *operations.Manager,
	gate MutationGate,
	lock InstanceLock,
	snapshotter snapshots.SafetySnapshotter,
	events Publisher,
	telemetry Telemetry,
	now Clock,
	newID IDGenerator,
) *Service {
	return &Service{
		repository:  repository,
		files:       files,
		catalog:     catalog,
		downloads:   downloads,
		operations:  operationManager,
		gate:        gate,
		lock:        lock,
		snapshotter: snapshotter,
		events:      events,
		telemetry:   telemetry,
		now:         now,
		newID:       newID,
	}
}

// ListMods reconciles the persisted installed-mod records with the files on
// disk and returns the fresh list. Files added outside the launcher are
// imported; records whose files disappeared are removed.
func (service *Service) ListMods(ctx context.Context, instanceID string) ([]InstalledMod, error) {
	if err := service.gate.Begin(); err != nil {
		return nil, err
	}
	defer service.gate.End()

	instance, err := service.repository.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if err = service.files.EnsureLayout(instance.Directory); err != nil {
		return nil, err
	}
	mods, err := service.repository.ListMods(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	discovered, err := service.files.Scan(instance.Directory)
	if err != nil {
		return nil, err
	}

	matched := make([]bool, len(discovered))
	now := service.now().UTC()
	for index := range mods {
		discoveredIndex := findDiscoveredMod(discovered, matched, mods[index])
		if discoveredIndex < 0 {
			if err = service.repository.DeleteMod(ctx, mods[index].ID); err != nil {
				return nil, err
			}
			continue
		}

		found := discovered[discoveredIndex]
		matched[discoveredIndex] = true
		mods[index].FileName = found.FileName
		mods[index].FilePath = found.FilePath
		mods[index].Enabled = found.Enabled
		mods[index].SizeBytes = found.SizeBytes
		mods[index].UpdatedAt = now
		if err = service.repository.SaveMod(ctx, mods[index]); err != nil {
			return nil, err
		}
	}

	for index, found := range discovered {
		if matched[index] {
			continue
		}
		installedAt := found.ModifiedAt.UTC()
		if installedAt.IsZero() {
			installedAt = now
		}
		mod := InstalledMod{
			ID:          service.newID(),
			InstanceID:  instanceID,
			Name:        found.Name,
			Version:     found.Version,
			FileName:    found.FileName,
			FilePath:    found.FilePath,
			Enabled:     found.Enabled,
			Managed:     false,
			Source:      "local",
			SizeBytes:   found.SizeBytes,
			InstalledAt: installedAt,
			UpdatedAt:   now,
		}
		if err = service.repository.SaveMod(ctx, mod); err != nil {
			return nil, err
		}
	}

	return service.repository.ListMods(ctx, instanceID)
}

func findDiscoveredMod(
	discovered []DiscoveredMod,
	matched []bool,
	installed InstalledMod,
) int {
	for index := range discovered {
		if !matched[index] && samePath(discovered[index].FilePath, installed.FilePath) {
			return index
		}
	}
	for index := range discovered {
		if !matched[index] && strings.EqualFold(discovered[index].FileName, installed.FileName) {
			return index
		}
	}
	return -1
}

func samePath(left string, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

// InstallModFile installs one local mod file into an instance. The returned
// operation reflects the install progress and outcome.
func (service *Service) InstallModFile(
	ctx context.Context,
	instanceID, sourcePath, name, version string,
) (operations.Operation, error) {
	if err := service.gate.Begin(); err != nil {
		return operations.Operation{}, err
	}
	defer service.gate.End()
	instance, err := service.repository.GetInstance(ctx, instanceID)
	if err != nil {
		return operations.Operation{}, err
	}
	instanceRelease, err := service.lockInstanceMutations(instanceID)
	if err != nil {
		return operations.Operation{}, err
	}
	defer instanceRelease()
	return service.installModFile(ctx, instance, sourcePath, name, version)
}

func (service *Service) installModFile(
	ctx context.Context,
	instance InstanceRef,
	sourcePath, name, version string,
) (operations.Operation, error) {
	slog.Info("installing mod file", "instance", instance.Name, "mod", name)
	if sourcePath == "" {
		return operations.Operation{}, domain.NewError(domain.ErrValidation, "Select a mod file")
	}
	now := service.now().UTC()
	resource := instance.ID
	operation := operations.Operation{
		ID:         service.newID(),
		Type:       "mod_install",
		ResourceID: &resource,
		Title:      "Installing mod",
		TitleKey:   operationTitleInstallingMod,
		Status:     operations.StatusRunning,
		Progress:   0.1,
		CreatedAt:  now,
		StartedAt:  &now,
	}
	if err := service.operations.Save(ctx, operation, ""); err != nil {
		slog.Warn("could not persist the created operation", "operationId", operation.ID, "error", err)
	}

	path, size, err := service.files.Install(ctx, sourcePath, instance.Directory)
	finished := service.now().UTC()
	operation.FinishedAt = &finished
	if err != nil {
		operation.Status = operations.StatusFailed
		msg := err.Error()
		code := "MOD_INSTALL_FAILED"
		operation.ErrorCode = &code
		operation.ErrorMessage = &msg
		service.operations.SaveBestEffort(operation, "")
		return operation, err
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	}
	if version == "" {
		version = "unknown"
	}
	mod := InstalledMod{
		ID:          service.newID(),
		InstanceID:  instance.ID,
		Name:        name,
		Version:     version,
		FileName:    filepath.Base(path),
		FilePath:    path,
		Enabled:     true,
		Managed:     false,
		Source:      "local",
		SizeBytes:   size,
		InstalledAt: finished,
		UpdatedAt:   finished,
	}
	if err = service.repository.SaveMod(ctx, mod); err != nil {
		return operation, err
	}
	service.bindInstalledModToExistingCache(ctx, mod)

	operation.Status = operations.StatusCompleted
	operation.Progress = 1
	operation.CurrentBytes = size
	operation.TotalBytes = size
	service.operations.SaveBestEffort(operation, "")
	service.events.Publish("mod:installed", mod)
	return operation, nil
}

// InstallModFiles installs several local mod files into one instance, skipping
// duplicates and collecting per-file failures.
func (service *Service) InstallModFiles(
	ctx context.Context,
	instanceID string,
	sourcePaths []string,
) (InstallModFilesResult, error) {
	result := InstallModFilesResult{}
	if err := service.gate.Begin(); err != nil {
		return result, err
	}
	defer service.gate.End()
	if len(sourcePaths) == 0 {
		return result, domain.NewError(domain.ErrValidation, "Select at least one mod file")
	}
	instance, err := service.repository.GetInstance(ctx, instanceID)
	if err != nil {
		return result, err
	}
	instanceRelease, err := service.lockInstanceMutations(instanceID)
	if err != nil {
		return result, err
	}
	defer instanceRelease()
	for _, sourcePath := range sourcePaths {
		if sourcePath == "" {
			result.Failed = append(result.Failed, ModFileFailure{Path: sourcePath, Error: "empty path"})
			continue
		}
		_, err := service.installModFile(ctx, instance, sourcePath, "", "")
		switch {
		case err == nil:
			result.Installed = append(result.Installed, filepath.Base(sourcePath))
		case errors.Is(err, ErrModFileExists):
			result.Skipped = append(result.Skipped, filepath.Base(sourcePath))
		default:
			result.Failed = append(result.Failed, ModFileFailure{Path: sourcePath, Error: err.Error()})
		}
	}
	if len(result.Installed) == 0 && len(result.Failed) > 0 {
		return result, domain.NewError(ErrInvalidModFile, "no mods were installed")
	}
	return result, nil
}

// SetModEnabled enables or disables an installed mod by moving it between the
// Mods and ModsDisabled directories.
func (service *Service) SetModEnabled(ctx context.Context, id string, enabled bool) (InstalledMod, error) {
	if err := service.gate.Begin(); err != nil {
		return InstalledMod{}, err
	}
	defer service.gate.End()
	mod, err := service.repository.GetMod(ctx, id)
	if err != nil {
		return mod, err
	}
	instanceRelease, err := service.lockInstanceMutations(mod.InstanceID)
	if err != nil {
		return mod, err
	}
	defer instanceRelease()
	instance, err := service.repository.GetInstance(ctx, mod.InstanceID)
	if err != nil {
		return mod, err
	}
	path, err := service.files.SetEnabled(mod.FilePath, instance.Directory, enabled)
	if err != nil {
		return mod, err
	}
	mod.FilePath = path
	mod.Enabled = enabled
	mod.UpdatedAt = service.now().UTC()
	err = service.repository.SaveMod(ctx, mod)
	if err == nil {
		event := "mod:disabled"
		if enabled {
			event = "mod:enabled"
		}
		service.events.Publish(event, mod)
	}
	return mod, err
}

// DeleteMod removes an installed mod, optionally together with every
// dependency that no remaining installed mod still requires. Exactly one
// automatic safety snapshot is created before the first mod is removed.
func (service *Service) DeleteMod(ctx context.Context, id string, deleteDependencies bool) error {
	if err := service.gate.Begin(); err != nil {
		return err
	}
	defer service.gate.End()
	mod, err := service.repository.GetMod(ctx, id)
	if err != nil {
		return err
	}
	toDelete := []InstalledMod{mod}
	if deleteDependencies {
		toDelete, err = service.modDeletionSet(ctx, mod)
		if err != nil {
			return err
		}
	}
	instanceRelease, err := service.lockInstanceMutations(mod.InstanceID)
	if err != nil {
		return err
	}
	defer instanceRelease()
	if err := service.snapshotter.Create(ctx, mod.InstanceID, snapshots.ReasonBeforeModRemoval, map[string]string{
		"affectedMods": strconv.Itoa(len(toDelete)),
	}); err != nil {
		return err
	}
	for _, mod := range toDelete {
		if err := service.removeInstalledMod(ctx, mod); err != nil {
			return err
		}
	}
	return nil
}

// RemoveMods removes several installed mods of one instance in a single
// destructive transaction. Exactly one automatic safety snapshot is created
// before the first mod is removed; a failed snapshot aborts the removal.
func (service *Service) RemoveMods(
	ctx context.Context,
	instanceID string,
	modIDs []string,
	deleteDependencies bool,
) error {
	if err := service.gate.Begin(); err != nil {
		return err
	}
	defer service.gate.End()
	if len(modIDs) == 0 {
		return domain.NewError(domain.ErrValidation, "Select at least one mod to remove")
	}
	instance, err := service.repository.GetInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(modIDs))
	var toDelete []InstalledMod
	for _, id := range modIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		mod, getErr := service.repository.GetMod(ctx, id)
		if getErr != nil {
			return getErr
		}
		if mod.InstanceID != instance.ID {
			return domain.NewError(domain.ErrValidation, "The selected mod does not belong to this instance")
		}
		if deleteDependencies {
			set, setErr := service.modDeletionSet(ctx, mod)
			if setErr != nil {
				return setErr
			}
			toDelete = append(toDelete, set...)
		} else {
			toDelete = append(toDelete, mod)
		}
	}
	if len(toDelete) == 0 {
		return nil
	}

	instanceRelease, err := service.lockInstanceMutations(instance.ID)
	if err != nil {
		return err
	}
	defer instanceRelease()
	if err := service.snapshotter.Create(ctx, instance.ID, snapshots.ReasonBeforeModRemoval, map[string]string{
		"affectedMods": strconv.Itoa(len(toDelete)),
	}); err != nil {
		return err
	}
	for _, mod := range toDelete {
		if err := service.removeInstalledMod(ctx, mod); err != nil {
			return err
		}
	}
	return nil
}

// removeInstalledMod deletes a single installed mod file and its record.
// Snapshot creation is the caller's responsibility; this method never
// creates one so nested removal flows cannot produce duplicate backups.
func (service *Service) removeInstalledMod(ctx context.Context, mod InstalledMod) error {
	if err := os.Remove(mod.FilePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := service.repository.DeleteMod(ctx, mod.ID); err != nil {
		return err
	}
	service.events.Publish("mod:removed", map[string]string{"id": mod.ID, "instanceId": mod.InstanceID})
	service.reportEvent(ctx, EventModRemoved)
	slog.Info("mod removed", "mod", mod.Name)
	return nil
}

// ModDeletePreview reports which dependencies would be removed together with
// the given mod, so the UI can ask the user before deleting anything.
func (service *Service) ModDeletePreview(ctx context.Context, id string) (ModDeletePreview, error) {
	mod, err := service.repository.GetMod(ctx, id)
	if err != nil {
		return ModDeletePreview{}, err
	}
	toDelete, err := service.modDeletionSet(ctx, mod)
	if err != nil {
		return ModDeletePreview{}, err
	}
	preview := ModDeletePreview{ModID: mod.ID, ModName: mod.Name, Dependencies: []InstalledMod{}}
	for _, dependency := range toDelete[1:] {
		preview.Dependencies = append(preview.Dependencies, dependency)
	}
	return preview, nil
}

// modDeletionSet returns the mod to delete followed by every dependency that
// no remaining installed mod still requires. Dependencies that another
// installed mod depends on are kept, so deleting one mod never breaks another.
func (service *Service) modDeletionSet(ctx context.Context, target InstalledMod) ([]InstalledMod, error) {
	mods, err := service.repository.ListMods(ctx, target.InstanceID)
	if err != nil {
		return nil, err
	}
	requiredBy := make(map[string][]string, len(mods))
	providers := make(map[string][]InstalledMod)
	for _, mod := range mods {
		info, infoErr := ReadModArchiveInfo(mod.FilePath)
		if infoErr != nil || strings.TrimSpace(info.ModID) == "" {
			continue
		}
		for dependencyID := range info.Dependencies {
			if !isBuiltInModDependency(dependencyID) {
				requiredBy[mod.ID] = append(requiredBy[mod.ID], dependencyID)
			}
		}
		key := strings.ToLower(strings.TrimSpace(info.ModID))
		providers[key] = append(providers[key], mod)
	}
	ordered := []InstalledMod{target}
	deleting := map[string]struct{}{target.ID: {}}
	for index := 0; index < len(ordered); index++ {
		for _, dependencyID := range requiredBy[ordered[index].ID] {
			providerKey := strings.ToLower(strings.TrimSpace(dependencyID))
			if len(providers[providerKey]) == 0 || stillRequiredByOther(mods, deleting, requiredBy, dependencyID) {
				continue
			}
			for _, provider := range providers[providerKey] {
				if _, alreadyDeleting := deleting[provider.ID]; alreadyDeleting {
					continue
				}
				deleting[provider.ID] = struct{}{}
				ordered = append(ordered, provider)
			}
		}
	}
	return ordered, nil
}

// stillRequiredByOther reports whether an installed mod outside the deletion
// set declares the given dependency.
func stillRequiredByOther(
	mods []InstalledMod,
	deleting map[string]struct{},
	requiredBy map[string][]string,
	dependencyID string,
) bool {
	for _, mod := range mods {
		if _, willBeDeleted := deleting[mod.ID]; willBeDeleted {
			continue
		}
		for _, required := range requiredBy[mod.ID] {
			if strings.EqualFold(required, dependencyID) {
				return true
			}
		}
	}
	return false
}

// lockInstanceMutations reserves the per-instance mutation slot. The returned
// release function must be called exactly once when the operation finishes.
func (service *Service) lockInstanceMutations(instanceID string) (func(), error) {
	return service.lock.Lock(instanceID, MutationLockMarker)
}

func (service *Service) reportEvent(ctx context.Context, name string) {
	if service.telemetry != nil {
		service.telemetry.Event(ctx, name)
	}
}

func (service *Service) reportError(ctx context.Context, code, component, operation string) {
	if service.telemetry != nil {
		service.telemetry.Error(ctx, code, component, operation)
	}
}

// bindInstalledModToExistingCache binds a freshly installed local mod when a
// matching catalog record already exists in the library.
func (service *Service) bindInstalledModToExistingCache(ctx context.Context, mod InstalledMod) {
	if mod.FilePath == "" {
		return
	}
	match, found := matchLocalModForFile(ctx, service.catalog, mod.FilePath)
	if !found {
		return
	}
	if _, err := service.downloads.Get(ctx, match.details.ID, match.version.ID); err != nil {
		return
	}
	mod.Source = ModDBSource(match.details.ID, match.version.ID)
	mod.Managed = true
	mod.Version = match.version.Version
	mod.UpdatedAt = service.now().UTC()
	if err := service.repository.SaveMod(ctx, mod); err == nil {
		service.events.Publish("mod:linked", mod)
	}
}
