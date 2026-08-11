package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/downloads"
	"github.com/waxlight/waxlight-launcher/internal/events"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/snapshotstore"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/launching"
	"github.com/waxlight/waxlight-launcher/internal/mutations"
	"github.com/waxlight/waxlight-launcher/internal/operations"
	"github.com/waxlight/waxlight-launcher/internal/sessions"
	settingscore "github.com/waxlight/waxlight-launcher/internal/settings"
	"github.com/waxlight/waxlight-launcher/internal/telemetry"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

type Service struct {
	store              Store
	clientSettings     ClientSettingsPatcher
	downloader         downloads.Downloader
	diskSpace          DiskSpaceChecker
	versions           VersionCapabilities
	modFiles           ModFileManager
	modCatalog         ModCatalog
	modDownloads       DownloadedModStore
	dataRoot           string
	snapshots          *snapshotstore.Store
	events             events.Publisher
	telemetry          *telemetry.Service
	modsMu             sync.Mutex
	mutationGate       *mutations.Gate
	settings           *settingscore.Reader
	operations         *operations.Manager
	sessions           *sessions.Service
	instanceQueries    *instances.QueryService
	instanceCreator    *instances.CreateService
	instanceUpdater    *instances.UpdateService
	instanceDeleter    *instances.DeleteService
	instanceCloner     *instances.CloneService
	instanceSlot       *mutations.Slot
	launchRegistry     *launching.Registry
	modTasksMu         sync.Mutex
	modTaskCancels     map[string]context.CancelFunc
	activeModDownloads map[string]string
}

type VersionCapabilities interface {
	Get(context.Context, string) (versions.GameVersion, error)
	List(context.Context) ([]versions.GameVersion, error)
	ListAvailable(context.Context) ([]versions.AvailableGameVersion, error)
	ResolveExecutable(context.Context, string) (versions.GameVersion, error)
	InstallCatalogAndWait(context.Context, string) (versions.GameVersion, error)
}

func NewService(
	store Store,
	modFiles ModFileManager,
	dataRoot string,
	operationManager *operations.Manager,
	sessionService *sessions.Service,
	instanceQueries *instances.QueryService,
	instanceCreator *instances.CreateService,
	instanceCloneStorage instances.CloneStorage,
	versionService VersionCapabilities,
	downloader downloads.Downloader,
	diskSpace DiskSpaceChecker,
	mutationGate *mutations.Gate,
	settingsReader *settingscore.Reader,
	instanceSlot *mutations.Slot,
	launchRegistry *launching.Registry,
) *Service {
	service := &Service{
		store:              store,
		modFiles:           modFiles,
		dataRoot:           dataRoot,
		snapshots:          snapshotstore.New(dataRoot),
		operations:         operationManager,
		sessions:           sessionService,
		instanceQueries:    instanceQueries,
		instanceCreator:    instanceCreator,
		versions:           versionService,
		downloader:         downloader,
		diskSpace:          diskSpace,
		mutationGate:       mutationGate,
		settings:           settingsReader,
		instanceSlot:       instanceSlot,
		launchRegistry:     launchRegistry,
		modTaskCancels:     make(map[string]context.CancelFunc),
		activeModDownloads: make(map[string]string),
	}
	service.instanceUpdater = instances.NewUpdateService(
		store,
		versionService,
		mutationGate,
		launchRegistry,
		instances.SafetySnapshotterFunc(func(ctx context.Context, instanceID string, reason domain.SnapshotReason, snapshotContext map[string]string) error {
			_, err := service.createSafetySnapshot(ctx, instanceID, reason, snapshotContext)
			return err
		}),
		func(path string) error {
			if service.clientSettings == nil {
				return nil
			}
			return service.clientSettings.Clear(path)
		},
		instances.PublishFunc(service.emit),
		time.Now,
	)
	service.instanceDeleter = instances.NewDeleteService(
		store,
		mutationGate,
		launchRegistry,
		func(path string) error { return safeRemoveAll(path, dataRoot, ".waxlight-instance") },
		func(path string) error {
			if service.clientSettings == nil {
				return nil
			}
			return service.clientSettings.Clear(path)
		},
		store.DeleteLastKnownGood,
		instances.PublishFunc(service.emit),
		service.reportEvent,
	)
	service.instanceCloner = instances.NewCloneService(
		store,
		store,
		instanceCreator,
		mutationGate,
		launchRegistry,
		instanceCloneStorage,
		func(path string) error { return safeRemoveAll(path, dataRoot, ".waxlight-instance") },
		time.Now,
		newID,
	)
	return service
}

func (s *Service) ConfigureMods(
	catalog ModCatalog,
	downloads DownloadedModStore,
) {
	s.modCatalog = catalog
	s.modDownloads = downloads
	slog.Info("mod subsystem configured")
}

func (s *Service) SetEventPublisher(publisher events.Publisher) {
	s.events = publisher
}

// ConfigureTelemetry wires the privacy-preserving telemetry service into the
// application layer. All telemetry calls inside this service are optional and
// never affect the outcome of the operations that produce them.
func (s *Service) ConfigureTelemetry(t *telemetry.Service) {
	s.telemetry = t
}

// reportEvent forwards an allowlisted telemetry event. Telemetry is strictly
// best-effort: delivery failures never surface to the caller.
func (s *Service) reportEvent(ctx context.Context, name string) {
	if s.telemetry != nil {
		s.telemetry.Event(ctx, name)
	}
}

// reportError forwards a structured telemetry error category. Raw errors are
// never attached; only allowlisted codes reach the telemetry backend.
func (s *Service) reportError(ctx context.Context, code, component, operation string) {
	if s.telemetry != nil {
		s.telemetry.Error(ctx, code, component, operation)
	}
}

func (s *Service) beginMutation() (func(), error) {
	if err := s.mutationGate.Begin(); err != nil {
		return nil, err
	}
	return s.mutationGate.End, nil
}

// ConfigureClientSettings wires the client-settings patcher into the snapshot
// and instance-credential-cleanup paths.
func (s *Service) ConfigureClientSettings(clientSettings ClientSettingsPatcher) {
	s.clientSettings = clientSettings
	slog.Info("client settings subsystem configured")
}

func (s *Service) emit(name string, payload any) {
	if s.events != nil {
		s.events.Publish(name, payload)
	}
}
func (s *Service) Close() error {
	return s.store.Close()
}

func newID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
func cleanName(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", domain.NewError(domain.ErrValidation, "Name cannot be empty")
	}
	if len([]rune(v)) > 80 {
		return "", domain.NewError(domain.ErrValidation, "Name cannot exceed 80 characters")
	}
	return v, nil
}

func isAppErrorCode(err error, code string) bool {
	var appError *domain.AppError
	return errors.As(err, &appError) && appError.Code == code
}

func (s *Service) CreateInstance(ctx context.Context, input instances.CreateInput) (instances.Instance, error) {
	return s.instanceCreator.Create(ctx, input)
}

func (s *Service) ListInstances(ctx context.Context) ([]instances.Instance, error) {
	return s.instanceQueries.List(ctx)
}
func (s *Service) GetInstance(ctx context.Context, id string) (instances.Instance, error) {
	return s.instanceQueries.Get(ctx, id)
}

func (s *Service) InstanceUpdater() *instances.UpdateService {
	return s.instanceUpdater
}

func (s *Service) InstanceDeleter() *instances.DeleteService {
	return s.instanceDeleter
}

func (s *Service) InstanceCloner() *instances.CloneService {
	return s.instanceCloner
}

func safeRemoveAll(path, dataRoot, marker string) error {
	abs, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	root, rootError := filepath.Abs(dataRoot)
	if rootError != nil {
		return rootError
	}
	home, _ := os.UserHomeDir()
	volumeRoot := filepath.VolumeName(abs) + string(os.PathSeparator)
	if abs == "/" || abs == volumeRoot || abs == home || abs == root || len(abs) < 5 {
		return domain.NewError(domain.ErrValidation, "Unsafe deletion path")
	}
	if _, e = os.Stat(filepath.Join(abs, marker)); e != nil {
		return domain.NewError(domain.ErrValidation, "The directory is not managed by Waxlight; no files were deleted")
	}
	return removeAllReliably(abs)
}

func samePath(left string, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func removeAllReliably(path string) error {
	var lastError error
	for attempt := 0; attempt < 5; attempt++ {
		if runtime.GOOS == "windows" {
			// Extracted installers may leave read-only attributes behind. Go's
			// chmod implementation clears that attribute on Windows.
			_ = filepath.Walk(path, func(currentPath string, info os.FileInfo, walkErr error) error {
				if walkErr != nil {
					slog.Debug("walk failed while clearing read-only attributes", "path", currentPath, "error", walkErr)
					return nil
				}
				if info == nil {
					return nil
				}
				if chmodErr := os.Chmod(currentPath, info.Mode()|0o200); chmodErr != nil {
					slog.Debug("could not clear the read-only attribute", "path", currentPath, "error", chmodErr)
				}
				return nil
			})
		}

		lastError = os.RemoveAll(path)
		if lastError == nil {
			_, statError := os.Lstat(path)
			if errors.Is(statError, os.ErrNotExist) {
				return nil
			}
			if statError != nil {
				return statError
			}
			lastError = fmt.Errorf("directory still exists after recursive removal: %s", path)
		}
		if runtime.GOOS != "windows" {
			break
		}
		time.Sleep(time.Duration(attempt+1) * 50 * time.Millisecond)
	}
	return lastError
}

func (s *Service) ListMods(ctx context.Context, instanceID string) ([]domain.InstalledMod, error) {
	release, err := s.beginMutation()
	if err != nil {
		return nil, err
	}
	defer release()
	s.modsMu.Lock()
	defer s.modsMu.Unlock()

	instance, e := s.store.GetInstance(ctx, instanceID)
	if e != nil {
		return nil, e
	}
	if e = s.modFiles.EnsureLayout(instance.Directory); e != nil {
		return nil, e
	}
	mods, e := s.store.ListMods(ctx, instanceID)
	if e != nil {
		return nil, e
	}
	discovered, e := s.modFiles.Scan(instance.Directory)
	if e != nil {
		return nil, e
	}

	matched := make([]bool, len(discovered))
	now := time.Now().UTC()
	for index := range mods {
		discoveredIndex := findDiscoveredMod(discovered, matched, mods[index])
		if discoveredIndex < 0 {
			if e = s.store.DeleteMod(ctx, mods[index].ID); e != nil {
				return nil, e
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
		if e = s.store.SaveMod(ctx, mods[index]); e != nil {
			return nil, e
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
		mod := domain.InstalledMod{
			ID:          newID(),
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
		if e = s.store.SaveMod(ctx, mod); e != nil {
			return nil, e
		}
	}

	return s.store.ListMods(ctx, instanceID)
}

func findDiscoveredMod(
	discovered []domain.DiscoveredMod,
	matched []bool,
	installed domain.InstalledMod,
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
func (s *Service) InstallModFile(ctx context.Context, instanceID, sourcePath, name, version string) (operations.Operation, error) {
	release, err := s.beginMutation()
	if err != nil {
		return operations.Operation{}, err
	}
	defer release()
	i, e := s.store.GetInstance(ctx, instanceID)
	if e != nil {
		return operations.Operation{}, e
	}
	instanceRelease, err := s.lockInstanceMutations(instanceID)
	if err != nil {
		return operations.Operation{}, err
	}
	defer instanceRelease()
	return s.installModFile(ctx, i, sourcePath, name, version)
}

func (s *Service) installModFile(ctx context.Context, i instances.Instance, sourcePath, name, version string) (operations.Operation, error) {
	slog.Info("installing mod file", "instance", i.Name, "mod", name)
	if sourcePath == "" {
		return operations.Operation{}, domain.NewError(domain.ErrValidation, "Select a mod file")
	}
	now := time.Now().UTC()
	resource := i.ID
	operation := operations.Operation{
		ID:         newID(),
		Type:       "mod_install",
		ResourceID: &resource,
		Title:      "Installing mod",
		TitleKey:   operationTitleInstallingMod,
		Status:     operations.StatusRunning,
		Progress:   0.1,
		CreatedAt:  now,
		StartedAt:  &now,
	}
	if err := s.operations.Save(ctx, operation, ""); err != nil {
		slog.Warn("could not persist the created operation", "operationId", operation.ID, "error", err)
	}

	path, size, e := s.modFiles.Install(ctx, sourcePath, i.Directory)
	finished := time.Now().UTC()
	operation.FinishedAt = &finished
	if e != nil {
		operation.Status = operations.StatusFailed
		msg := e.Error()
		code := "MOD_INSTALL_FAILED"
		operation.ErrorCode = &code
		operation.ErrorMessage = &msg
		s.operations.SaveBestEffort(operation, "")
		return operation, e
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	}
	if version == "" {
		version = "unknown"
	}
	mod := domain.InstalledMod{
		ID:          newID(),
		InstanceID:  i.ID,
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
	if e = s.store.SaveMod(ctx, mod); e != nil {
		return operation, e
	}
	s.bindInstalledModToExistingCache(ctx, mod)

	operation.Status = operations.StatusCompleted
	operation.Progress = 1
	operation.CurrentBytes = size
	operation.TotalBytes = size
	s.operations.SaveBestEffort(operation, "")
	s.emit("mod:installed", mod)
	return operation, nil
}

type InstallModFilesResult struct {
	Installed []string
	Skipped   []string
	Failed    []ModFileFailure
}

type ModFileFailure struct {
	Path  string
	Error string
}

func (s *Service) InstallModFiles(ctx context.Context, instanceID string, sourcePaths []string) (InstallModFilesResult, error) {
	result := InstallModFilesResult{}
	release, err := s.beginMutation()
	if err != nil {
		return result, err
	}
	defer release()
	if len(sourcePaths) == 0 {
		return result, domain.NewError(domain.ErrValidation, "Select at least one mod file")
	}
	i, e := s.store.GetInstance(ctx, instanceID)
	if e != nil {
		return result, e
	}
	instanceRelease, err := s.lockInstanceMutations(instanceID)
	if err != nil {
		return result, err
	}
	defer instanceRelease()
	for _, sourcePath := range sourcePaths {
		if sourcePath == "" {
			result.Failed = append(result.Failed, ModFileFailure{Path: sourcePath, Error: "empty path"})
			continue
		}
		_, err := s.installModFile(ctx, i, sourcePath, "", "")
		switch {
		case err == nil:
			result.Installed = append(result.Installed, filepath.Base(sourcePath))
		case errors.Is(err, domain.ErrModFileExists):
			result.Skipped = append(result.Skipped, filepath.Base(sourcePath))
		default:
			result.Failed = append(result.Failed, ModFileFailure{Path: sourcePath, Error: err.Error()})
		}
	}
	if len(result.Installed) == 0 && len(result.Failed) > 0 {
		return result, domain.NewError(domain.ErrInvalidModFile, "no mods were installed")
	}
	return result, nil
}
func (s *Service) SetModEnabled(ctx context.Context, id string, enabled bool) (domain.InstalledMod, error) {
	release, err := s.beginMutation()
	if err != nil {
		return domain.InstalledMod{}, err
	}
	defer release()
	m, e := s.store.GetMod(ctx, id)
	if e != nil {
		return m, e
	}
	instanceRelease, err := s.lockInstanceMutations(m.InstanceID)
	if err != nil {
		return m, err
	}
	defer instanceRelease()
	i, e := s.store.GetInstance(ctx, m.InstanceID)
	if e != nil {
		return m, e
	}
	path, e := s.modFiles.SetEnabled(m.FilePath, i.Directory, enabled)
	if e != nil {
		return m, e
	}
	m.FilePath = path
	m.Enabled = enabled
	m.UpdatedAt = time.Now().UTC()
	e = s.store.SaveMod(ctx, m)
	if e == nil {
		event := "mod:disabled"
		if enabled {
			event = "mod:enabled"
		}
		s.emit(event, m)
	}
	return m, e
}
func (s *Service) DeleteMod(ctx context.Context, id string, deleteDependencies bool) error {
	release, err := s.beginMutation()
	if err != nil {
		return err
	}
	defer release()
	m, e := s.store.GetMod(ctx, id)
	if e != nil {
		return e
	}
	toDelete := []domain.InstalledMod{m}
	if deleteDependencies {
		toDelete, e = s.modDeletionSet(ctx, m)
		if e != nil {
			return e
		}
	}
	instanceRelease, err := s.lockInstanceMutations(m.InstanceID)
	if err != nil {
		return err
	}
	defer instanceRelease()
	if _, err := s.createSafetySnapshot(ctx, m.InstanceID, domain.SnapshotReasonBeforeModRemoval, map[string]string{
		"affectedMods": strconv.Itoa(len(toDelete)),
	}); err != nil {
		return err
	}
	for _, mod := range toDelete {
		if err := s.removeInstalledMod(ctx, mod); err != nil {
			return err
		}
	}
	return nil
}

// RemoveMods removes several installed mods of one instance in a single
// destructive transaction. Exactly one automatic safety snapshot is created
// before the first mod is removed; a failed snapshot aborts the removal.
func (s *Service) RemoveMods(
	ctx context.Context,
	instanceID string,
	modIDs []string,
	deleteDependencies bool,
) error {
	release, err := s.beginMutation()
	if err != nil {
		return err
	}
	defer release()
	if len(modIDs) == 0 {
		return domain.NewError(domain.ErrValidation, "Select at least one mod to remove")
	}
	instance, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(modIDs))
	var toDelete []domain.InstalledMod
	for _, id := range modIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		mod, getErr := s.store.GetMod(ctx, id)
		if getErr != nil {
			return getErr
		}
		if mod.InstanceID != instance.ID {
			return domain.NewError(domain.ErrValidation, "The selected mod does not belong to this instance")
		}
		if deleteDependencies {
			set, setErr := s.modDeletionSet(ctx, mod)
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

	instanceRelease, err := s.lockInstanceMutations(instance.ID)
	if err != nil {
		return err
	}
	defer instanceRelease()
	if _, err := s.createSafetySnapshot(ctx, instance.ID, domain.SnapshotReasonBeforeModRemoval, map[string]string{
		"affectedMods": strconv.Itoa(len(toDelete)),
	}); err != nil {
		return err
	}
	for _, mod := range toDelete {
		if err := s.removeInstalledMod(ctx, mod); err != nil {
			return err
		}
	}
	return nil
}

// removeInstalledMod deletes a single installed mod file and its record.
// Snapshot creation is the caller's responsibility; this method never
// creates one so nested removal flows cannot produce duplicate backups.
func (s *Service) removeInstalledMod(ctx context.Context, mod domain.InstalledMod) error {
	if e := os.Remove(mod.FilePath); e != nil && !errors.Is(e, os.ErrNotExist) {
		return e
	}
	if e := s.store.DeleteMod(ctx, mod.ID); e != nil {
		return e
	}
	s.emit("mod:removed", map[string]string{"id": mod.ID, "instanceId": mod.InstanceID})
	s.reportEvent(ctx, telemetry.EventModRemoved)
	slog.Info("mod removed", "mod", mod.Name)
	return nil
}

// ModDeletePreview reports which dependencies would be removed together with
// the given mod, so the UI can ask the user before deleting anything.
func (s *Service) ModDeletePreview(ctx context.Context, id string) (ModDeletePreview, error) {
	m, err := s.store.GetMod(ctx, id)
	if err != nil {
		return ModDeletePreview{}, err
	}
	toDelete, err := s.modDeletionSet(ctx, m)
	if err != nil {
		return ModDeletePreview{}, err
	}
	preview := ModDeletePreview{ModID: m.ID, ModName: m.Name, Dependencies: []domain.InstalledMod{}}
	for _, dependency := range toDelete[1:] {
		preview.Dependencies = append(preview.Dependencies, dependency)
	}
	return preview, nil
}

type ModDeletePreview struct {
	ModID        string
	ModName      string
	Dependencies []domain.InstalledMod
}

// modDeletionSet returns the mod to delete followed by every dependency that
// no remaining installed mod still requires. Dependencies that another
// installed mod depends on are kept, so deleting one mod never breaks another.
func (s *Service) modDeletionSet(ctx context.Context, target domain.InstalledMod) ([]domain.InstalledMod, error) {
	mods, err := s.store.ListMods(ctx, target.InstanceID)
	if err != nil {
		return nil, err
	}
	requiredBy := make(map[string][]string, len(mods))
	providers := make(map[string][]domain.InstalledMod)
	for _, mod := range mods {
		info, infoErr := readModArchiveInfo(mod.FilePath)
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
	ordered := []domain.InstalledMod{target}
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
	mods []domain.InstalledMod,
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
