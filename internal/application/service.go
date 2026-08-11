package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/downloads"
	"github.com/waxlight/waxlight-launcher/internal/events"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/instancedirectory"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/securefs"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/snapshotstore"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/mutations"
	"github.com/waxlight/waxlight-launcher/internal/operations"
	"github.com/waxlight/waxlight-launcher/internal/sessions"
	settingscore "github.com/waxlight/waxlight-launcher/internal/settings"
	"github.com/waxlight/waxlight-launcher/internal/telemetry"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

type Service struct {
	store              Store
	accounts           *accounts.Service
	clientSettings     ClientSettingsPatcher
	serverCatalog      PublicServerCatalog
	downloader         downloads.Downloader
	diskSpace          DiskSpaceChecker
	versions           VersionCapabilities
	modFiles           ModFileManager
	modCatalog         ModCatalog
	modDownloads       DownloadedModStore
	launcher           ProcessLauncher
	dataRoot           string
	snapshots          *snapshotstore.Store
	events             events.Publisher
	telemetry          *telemetry.Service
	runningMu          sync.Mutex
	launchMu           sync.Mutex
	modsMu             sync.Mutex
	snapshotMu         sync.Mutex
	snapshotBusy       map[string]string
	mutationGate       *mutations.Gate
	settings           *settingscore.Reader
	operations         *operations.Manager
	sessions           *sessions.Service
	instanceQueries    *instances.QueryService
	instanceCreator    *instances.CreateService
	modTasksMu         sync.Mutex
	modTaskCancels     map[string]context.CancelFunc
	activeModDownloads map[string]string
	running            map[string]runningGame
}

type VersionCapabilities interface {
	Get(context.Context, string) (versions.GameVersion, error)
	List(context.Context) ([]versions.GameVersion, error)
	ListAvailable(context.Context) ([]versions.AvailableGameVersion, error)
	ResolveExecutable(context.Context, string) (versions.GameVersion, error)
	InstallCatalogAndWait(context.Context, string) (versions.GameVersion, error)
}

type runningGame struct {
	process   RunningProcess
	sessionID string
	started   time.Time
	log       io.Closer
	cleanup   func() error
}

func NewService(
	store Store,
	modFiles ModFileManager,
	launcher ProcessLauncher,
	dataRoot string,
	operationManager *operations.Manager,
	sessionService *sessions.Service,
	instanceQueries *instances.QueryService,
	instanceCreator *instances.CreateService,
	versionService VersionCapabilities,
	downloader downloads.Downloader,
	diskSpace DiskSpaceChecker,
	mutationGate *mutations.Gate,
	settingsReader *settingscore.Reader,
) *Service {
	return &Service{
		store:              store,
		modFiles:           modFiles,
		launcher:           launcher,
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
		modTaskCancels:     make(map[string]context.CancelFunc),
		activeModDownloads: make(map[string]string),
		running:            make(map[string]runningGame),
		snapshotBusy:       make(map[string]string),
	}
}

func (s *Service) ConfigureMods(
	catalog ModCatalog,
	downloads DownloadedModStore,
) {
	s.modCatalog = catalog
	s.modDownloads = downloads
	slog.Info("mod subsystem configured")
}

func (s *Service) ConfigurePublicServerCatalog(catalog PublicServerCatalog) {
	s.serverCatalog = catalog
	slog.Info("public server catalog configured")
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

// CheckDataRootRelocation rejects a move while a game or operation is running.
func (s *Service) CheckDataRootRelocation(ctx context.Context) error {
	s.runningMu.Lock()
	gameRunning := len(s.running) > 0
	s.runningMu.Unlock()
	if gameRunning {
		return domain.NewError(instances.ErrInstanceRunning, "Stop the game before moving the data folder")
	}
	tracked, err := s.operations.ListLimit(ctx, 1000)
	if err != nil {
		return err
	}
	for _, operation := range tracked {
		if operation.Status == operations.StatusRunning || operation.Status == operations.StatusQueued {
			return domain.NewError(
				domain.ErrDataFolderBusy,
				"Wait for running operations to finish before moving the data folder",
			)
		}
	}
	return nil
}

func (s *Service) beginMutation() (func(), error) {
	if err := s.mutationGate.Begin(); err != nil {
		return nil, err
	}
	return s.mutationGate.End, nil
}

func (s *Service) ConfigureAuthentication(
	accountService *accounts.Service,
	clientSettings ClientSettingsPatcher,
) {
	s.accounts = accountService
	s.clientSettings = clientSettings
	slog.Info("authentication subsystem configured")
}

func (s *Service) ReconcileInjectedCredentials(ctx context.Context) error {
	if s.clientSettings == nil {
		return nil
	}
	release, err := s.beginMutation()
	if err != nil {
		return err
	}
	defer release()
	instances, err := s.store.ListInstances(ctx)
	if err != nil {
		return err
	}
	for _, instance := range instances {
		if err := hardenLogs(filepath.Join(instance.Directory, "Logs")); err != nil {
			return err
		}
		if err := s.clientSettings.Reconcile(filepath.Join(instance.Directory, "clientsettings.json")); err != nil {
			return &domain.AppError{Code: domain.ErrClientSettings, Message: "Could not clear stale instance authentication", Cause: err}
		}
	}
	return nil
}

func (s *Service) ClearAccountFromInstances(ctx context.Context, accountID string) error {
	if s.clientSettings == nil {
		return nil
	}
	release, err := s.beginMutation()
	if err != nil {
		return err
	}
	defer release()
	instances, err := s.store.ListInstances(ctx)
	if err != nil {
		return err
	}
	for _, instance := range instances {
		if err := s.clientSettings.Clear(filepath.Join(instance.Directory, "clientsettings.json")); err != nil {
			return &domain.AppError{Code: domain.ErrClientSettings, Message: "Could not clear account authentication from an instance", Cause: err}
		}
		if instance.DefaultAccountID != nil && *instance.DefaultAccountID == accountID {
			instance.DefaultAccountID = nil
			instance.UpdatedAt = time.Now().UTC()
			if err := s.store.SaveInstance(ctx, instance); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) emit(name string, payload any) {
	if s.events != nil {
		s.events.Publish(name, payload)
	}
}
func (s *Service) Close() error {
	// A game still running past the startup window when the launcher shuts
	// down is evidence its configuration works; record it before the database
	// closes. waitForGame never observes this exit because the launcher is
	// already gone.
	s.recordEstablishedLaunches()
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

type SaveFavoriteServerInput struct {
	ID         string
	Name       string
	Address    string
	InstanceID *string
}

func (s *Service) ListFavoriteServers(ctx context.Context) ([]domain.FavoriteServer, error) {
	return s.store.ListFavoriteServers(ctx)
}

func (s *Service) ListPublicServers(ctx context.Context) ([]domain.PublicServer, error) {
	if s.serverCatalog == nil {
		return nil, domain.NewError(domain.ErrValidation, "Public server catalog is unavailable")
	}
	return s.serverCatalog.List(ctx)
}

func (s *Service) SaveFavoriteServer(ctx context.Context, input SaveFavoriteServerInput) (domain.FavoriteServer, error) {
	release, err := s.beginMutation()
	if err != nil {
		return domain.FavoriteServer{}, err
	}
	defer release()
	name := strings.TrimSpace(input.Name)
	address := strings.TrimSpace(input.Address)
	if name == "" || len(name) > 100 || len(address) > 255 || strings.ContainsAny(address, "\r\n\t ") {
		return domain.FavoriteServer{}, domain.NewError(domain.ErrValidation, "Enter a server name and an address without spaces")
	}
	if input.InstanceID != nil {
		if _, err := s.store.GetInstance(ctx, *input.InstanceID); err != nil {
			return domain.FavoriteServer{}, err
		}
	}
	now := time.Now().UTC()
	server := domain.FavoriteServer{ID: input.ID, Name: name, Address: address, InstanceID: input.InstanceID, UpdatedAt: now}
	if server.ID == "" {
		server.ID = newID()
		server.CreatedAt = now
	} else if previous, err := s.store.GetFavoriteServer(ctx, server.ID); err != nil {
		return domain.FavoriteServer{}, err
	} else {
		server.CreatedAt = previous.CreatedAt
	}
	if err := s.store.SaveFavoriteServer(ctx, server); err != nil {
		return domain.FavoriteServer{}, err
	}
	s.emit("favorite-server:updated", server)
	return server, nil
}

func (s *Service) DeleteFavoriteServer(ctx context.Context, id string) error {
	release, err := s.beginMutation()
	if err != nil {
		return err
	}
	defer release()
	if err := s.store.DeleteFavoriteServer(ctx, id); err != nil {
		return err
	}
	s.emit("favorite-server:removed", map[string]string{"id": id})
	return nil
}
func (s *Service) UpdateInstance(ctx context.Context, in instances.Instance) (instances.Instance, error) {
	mutationRelease, mutationErr := s.beginMutation()
	if mutationErr != nil {
		return in, mutationErr
	}
	defer mutationRelease()
	old, e := s.store.GetInstance(ctx, in.ID)
	if e != nil {
		return in, e
	}
	in.Name, e = cleanName(in.Name)
	if e != nil {
		return in, e
	}
	if _, e = s.versions.Get(ctx, in.GameVersionID); e != nil {
		return in, e
	}
	if old.GameVersionID != in.GameVersionID {
		release, err := s.lockInstanceMutations(in.ID)
		if err != nil {
			return in, err
		}
		defer release()
		toVersion := in.GameVersionID
		if version, versionErr := s.versions.Get(ctx, in.GameVersionID); versionErr == nil && strings.TrimSpace(version.Name) != "" {
			toVersion = version.Name
		}
		if _, err := s.createSafetySnapshot(ctx, in.ID, domain.SnapshotReasonBeforeGameVersionChange, map[string]string{
			"fromGameVersion": s.instanceGameVersionName(ctx, old),
			"toGameVersion":   toVersion,
		}); err != nil {
			return in, err
		}
	}
	in.Directory = old.Directory
	in.CreatedAt = old.CreatedAt
	in.LastPlayedAt = old.LastPlayedAt
	in.Status = old.Status
	in.UpdatedAt = time.Now().UTC()
	if !sameOptionalString(old.DefaultAccountID, in.DefaultAccountID) && s.clientSettings != nil {
		if e = s.clientSettings.Clear(filepath.Join(old.Directory, "clientsettings.json")); e != nil {
			return in, e
		}
	}
	e = s.store.SaveInstance(ctx, in)
	if e == nil {
		s.emit("instance:updated", in)
	}
	return in, e
}
func (s *Service) DeleteInstance(ctx context.Context, id string, deleteFiles bool) error {
	release, err := s.beginMutation()
	if err != nil {
		return err
	}
	defer release()
	s.runningMu.Lock()
	_, running := s.running[id]
	s.runningMu.Unlock()
	if running {
		return domain.NewError(instances.ErrInstanceRunning, "Stop the game before deleting this instance")
	}
	if err := s.ensureNoSnapshotOperation(id); err != nil {
		return err
	}
	i, e := s.store.GetInstance(ctx, id)
	if e != nil {
		return e
	}
	if deleteFiles {
		if e = safeRemoveAll(i.Directory, s.dataRoot, ".waxlight-instance"); e != nil {
			return e
		}
	}
	if !deleteFiles && s.clientSettings != nil {
		if e = s.clientSettings.Clear(filepath.Join(i.Directory, "clientsettings.json")); e != nil {
			return e
		}
	}
	if e = s.store.DeleteInstance(ctx, id); e != nil {
		return e
	}
	if e = s.store.DeleteLastKnownGood(ctx, id); e != nil {
		slog.Warn("could not clean up the last known good state of the deleted instance", "instanceId", id, "error", e)
	}
	s.emit("instance:deleted", map[string]string{"id": id})
	s.reportEvent(ctx, telemetry.EventInstanceDeleted)
	slog.Info("instance deleted", "id", id)
	return nil
}

func sameOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
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

type LaunchValidation struct {
	Valid    bool
	Issues   []string
	Warnings []string
}

func (s *Service) ValidateLaunch(
	ctx context.Context,
	instanceID string,
	accountID *string,
) (LaunchValidation, error) {
	validation := LaunchValidation{
		Valid:    true,
		Issues:   []string{},
		Warnings: []string{},
	}

	instance, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return validation, err
	}

	version, err := s.versions.ResolveExecutable(ctx, instance.GameVersionID)
	if err != nil {
		validation.Valid = false
		issue := "The Vintagestory executable could not be found"
		var appError *domain.AppError
		if errors.As(err, &appError) && appError.Code == domain.ErrVersionNotFound {
			issue = "The selected game version is not installed"
		}
		validation.Issues = append(
			validation.Issues,
			issue,
		)
		return validation, nil
	}

	executableInfo, err := os.Stat(version.ExecutablePath)
	if err != nil || executableInfo.IsDir() {
		validation.Valid = false
		validation.Issues = append(
			validation.Issues,
			"The Vintagestory executable could not be found",
		)
	}
	chosen, chooseErr := s.resolveAccountID(ctx, instance, accountID)
	if chooseErr != nil {
		validation.Valid = false
		validation.Issues = append(validation.Issues, "The selected account no longer exists")
	}
	if chosen == nil {
		validation.Warnings = append(
			validation.Warnings,
			"No account is selected. The game will start without authentication data.",
		)
	} else if account, accountErr := s.accounts.GetAccount(ctx, *chosen); accountErr != nil {
		validation.Valid = false
		validation.Issues = append(validation.Issues, "The selected account no longer exists")
	} else if account.Status == accounts.StatusExpired || account.Status == accounts.StatusNeedsReauth {
		validation.Valid = false
		validation.Issues = append(validation.Issues, "The selected account must be authenticated again")
	}

	s.runningMu.Lock()
	_, isRunning := s.running[instanceID]
	s.runningMu.Unlock()
	if isRunning {
		validation.Valid = false
		validation.Issues = append(
			validation.Issues,
			"This instance is already running",
		)
	}

	return validation, nil
}

func (s *Service) Launch(
	ctx context.Context,
	instanceID string,
	accountID *string,
) (sessions.PlaySession, error) {
	return s.launch(ctx, instanceID, accountID, "")
}

// LaunchServer starts an instance and connects it to the requested server.
func (s *Service) LaunchServer(
	ctx context.Context,
	instanceID string,
	accountID *string,
	address string,
) (sessions.PlaySession, error) {
	address = strings.TrimSpace(address)
	if address == "" || len(address) > 255 || strings.ContainsAny(address, " \t\r\n/?#") {
		return sessions.PlaySession{}, domain.NewError(domain.ErrValidation, "Enter a valid server address")
	}
	return s.launch(ctx, instanceID, accountID, address)
}

func (s *Service) launch(
	ctx context.Context,
	instanceID string,
	accountID *string,
	serverAddress string,
) (sessions.PlaySession, error) {
	release, err := s.beginMutation()
	if err != nil {
		return sessions.PlaySession{}, err
	}
	releaseOnReturn := true
	defer func() {
		if releaseOnReturn {
			release()
		}
	}()
	s.launchMu.Lock()
	defer s.launchMu.Unlock()
	if err := s.ensureNoSnapshotOperation(instanceID); err != nil {
		return sessions.PlaySession{}, err
	}
	validation, err := s.ValidateLaunch(ctx, instanceID, accountID)
	if err != nil {
		return sessions.PlaySession{}, err
	}
	if !validation.Valid {
		return sessions.PlaySession{}, domain.NewError(
			domain.ErrValidation,
			strings.Join(validation.Issues, "; "),
		)
	}

	instance, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return sessions.PlaySession{}, err
	}
	version, err := s.versions.ResolveExecutable(ctx, instance.GameVersionID)
	if err != nil {
		return sessions.PlaySession{}, err
	}
	slog.Info("launching instance", "instance", instance.Name, "version", version.Name)
	accountID, err = s.resolveAccountID(ctx, instance, accountID)
	if err != nil {
		return sessions.PlaySession{}, err
	}

	clientSettingsPath := filepath.Join(instance.Directory, "clientsettings.json")
	cleanupCredentials := func() error { return nil }
	if accountID != nil {
		if s.accounts == nil || s.clientSettings == nil {
			return sessions.PlaySession{}, domain.NewError(domain.ErrValidation, "Account authentication is unavailable")
		}
		account, validateErr := s.accounts.ValidateAuthorizedAccount(ctx, *accountID)
		if validateErr != nil {
			return sessions.PlaySession{}, validateErr
		}
		cleanup, patchErr := s.clientSettings.Inject(clientSettingsPath, account)
		if patchErr != nil {
			return sessions.PlaySession{}, &domain.AppError{
				Code:    domain.ErrClientSettings,
				Message: "Could not write authentication to the instance settings",
				Cause:   patchErr,
			}
		}
		cleanupCredentials = cleanup
	} else if s.clientSettings != nil {
		if clearErr := s.clientSettings.Clear(clientSettingsPath); clearErr != nil {
			return sessions.PlaySession{}, &domain.AppError{
				Code:    domain.ErrClientSettings,
				Message: "Could not clear authentication from the instance settings",
				Cause:   clearErr,
			}
		}
	}

	if err := s.modFiles.EnsureLayout(instance.Directory); err != nil {
		s.clearInjectedCredentials(cleanupCredentials, instance)
		return sessions.PlaySession{}, err
	}
	logsDirectory := filepath.Join(instance.Directory, "Logs")
	if err := hardenLogs(logsDirectory); err != nil {
		s.clearInjectedCredentials(cleanupCredentials, instance)
		return sessions.PlaySession{}, err
	}

	settings, err := s.settings.Get(ctx)
	if err != nil {
		s.clearInjectedCredentials(cleanupCredentials, instance)
		return sessions.PlaySession{}, err
	}
	arguments := append([]string{}, settings.GlobalLaunchArguments...)
	arguments = append(arguments, instance.LaunchArguments...)
	arguments = append(arguments, "--dataPath", instance.Directory)
	if serverAddress != "" {
		arguments = append(arguments, "--connect", serverAddress)
	}

	logPath := filepath.Join(
		logsDirectory,
		time.Now().Format("20060102-150405.000000000")+".log",
	)
	logFile, err := os.OpenFile(
		logPath,
		os.O_CREATE|os.O_EXCL|os.O_WRONLY,
		0o600,
	)
	if err != nil {
		s.clearInjectedCredentials(cleanupCredentials, instance)
		return sessions.PlaySession{}, err
	}
	if err := securefs.Apply(logPath, 0o600, false); err != nil {
		closeLaunchLog(logFile, instance.Name)
		s.clearInjectedCredentials(cleanupCredentials, instance)
		return sessions.PlaySession{}, err
	}

	// Record the exact launch command so issues like a wrong data path can be
	// diagnosed from the instance log.
	if _, writeErr := fmt.Fprintf(
		logFile,
		"Executing: %s %s\n",
		version.ExecutablePath,
		strings.Join(quoteLaunchArguments(arguments), " "),
	); writeErr != nil {
		closeLaunchLog(logFile, instance.Name)
		s.clearInjectedCredentials(cleanupCredentials, instance)
		return sessions.PlaySession{}, &domain.AppError{
			Code:    domain.ErrFilePermission,
			Message: "Could not write the launch command to the instance log",
			Cause:   writeErr,
		}
	}

	workingDirectory := filepath.Dir(version.ExecutablePath)
	process, err := s.launcher.Start(
		context.Background(),
		version.ExecutablePath,
		arguments,
		workingDirectory,
		map[string]string{"WAXLIGHT_INSTANCE_DIR": instance.Directory},
		logFile,
	)
	if err != nil {
		closeLaunchLog(logFile, instance.Name)
		s.clearInjectedCredentials(cleanupCredentials, instance)
		s.reportEvent(ctx, telemetry.EventGameLaunchFailed)
		s.reportError(ctx, telemetry.ErrorGameLaunchFailed, telemetry.ComponentGameLauncher, telemetry.OperationLaunchGame)
		return sessions.PlaySession{}, &domain.AppError{
			Code:    domain.ErrProcessStart,
			Message: "Failed to start Vintage Story",
			Cause:   err,
		}
	}

	now := time.Now().UTC()
	processID := process.PID()
	session := sessions.PlaySession{
		ID:         newID(),
		InstanceID: instance.ID,
		AccountID:  accountID,
		VersionID:  version.ID,
		ProcessID:  &processID,
		StartedAt:  now,
	}
	if err := s.sessions.Create(ctx, session); err != nil {
		if killErr := process.Kill(); killErr != nil {
			slog.Debug("could not kill the game process after a failed session save", "error", killErr)
		}
		closeLaunchLog(logFile, instance.Name)
		s.clearInjectedCredentials(cleanupCredentials, instance)
		return session, err
	}

	instance.Status = instances.StatusRunning
	instance.LastPlayedAt = &now
	instance.UpdatedAt = now
	if err := s.store.SaveInstance(ctx, instance); err != nil {
		slog.Warn("could not persist the running instance", "instance", instance.Name, "error", err)
	}

	s.runningMu.Lock()
	s.running[instance.ID] = runningGame{
		process:   process,
		sessionID: session.ID,
		started:   now,
		log:       logFile,
		cleanup:   cleanupCredentials,
	}
	s.runningMu.Unlock()

	s.emit("game:started", session)
	s.reportEvent(ctx, telemetry.EventGameLaunchSucceeded)
	slog.Info("game started", "instance", instance.Name)
	// Snapshot the startup window once on the caller goroutine; the launch
	// goroutines below read only this captured value (the package variable is
	// mutable in tests).
	startupWindow := gameStartupWindow
	s.operations.Go(func(ctx context.Context) {
		s.markLaunchEstablished(ctx, instance, session.ID, startupWindow)
	})
	releaseOnReturn = false
	go s.waitForGame(instance, process, session.ID, now, logFile, cleanupCredentials, s.watchGameLog(instance, logPath), startupWindow, release)
	return session, nil
}

func quoteLaunchArguments(arguments []string) []string {
	result := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		if strings.ContainsAny(argument, " \t\"") {
			result = append(result, `"`+strings.ReplaceAll(argument, `"`, `\"`)+`"`)
		} else {
			result = append(result, argument)
		}
	}
	return result
}

func hardenLogs(logsDirectory string) error {
	return instancedirectory.HardenLogs(logsDirectory)
}

func (s *Service) resolveAccountID(
	ctx context.Context,
	instance instances.Instance,
	requested *string,
) (*string, error) {
	if requested != nil && strings.TrimSpace(*requested) != "" {
		if s.accounts == nil {
			return nil, domain.NewError(domain.ErrAccountNotFound, "Account not found")
		}
		if _, err := s.accounts.GetAccount(ctx, *requested); err != nil {
			return nil, err
		}
		value := *requested
		return &value, nil
	}
	if instance.DefaultAccountID != nil && strings.TrimSpace(*instance.DefaultAccountID) != "" {
		if s.accounts == nil {
			return nil, domain.NewError(domain.ErrAccountNotFound, "Account not found")
		}
		if _, err := s.accounts.GetAccount(ctx, *instance.DefaultAccountID); err != nil {
			return nil, err
		}
		value := *instance.DefaultAccountID
		return &value, nil
	}
	if s.accounts == nil {
		return nil, nil
	}
	storedAccounts, err := s.accounts.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	for _, account := range storedAccounts {
		if account.IsDefault {
			value := account.ID
			return &value, nil
		}
	}
	return nil, nil
}

// closeLaunchLog best-effort closes the instance log file opened for a launch.
// Failures only affect cleanup, so they are logged at debug level.
func closeLaunchLog(logFile io.Closer, instanceName string) {
	if err := logFile.Close(); err != nil {
		slog.Debug("could not close the instance log file", "instance", instanceName, "error", err)
	}
}

// clearInjectedCredentials removes the session credentials injected into the
// instance client settings. A failure leaves credentials on disk, so it is
// logged as an error.
func (s *Service) clearInjectedCredentials(cleanup func() error, instance instances.Instance) {
	if err := cleanup(); err != nil {
		slog.Error("could not remove injected credentials", "instance", instance.Name, "error", err)
	}
}

func (s *Service) waitForGame(
	instance instances.Instance,
	process RunningProcess,
	sessionID string,
	startedAt time.Time,
	logFile io.Closer,
	cleanupCredentials func() error,
	stopGameLog func(),
	startupWindow time.Duration,
	releaseMutation func(),
) {
	defer releaseMutation()
	exitCode, waitErr := process.Wait()
	// Let the tailer pick up the lines the process flushed right before
	// exiting, then stop it before the log file is closed.
	stopGameLog()
	if err := logFile.Close(); err != nil {
		slog.Debug("could not close the instance log file", "instance", instance.Name, "error", err)
	}
	s.clearInjectedCredentials(cleanupCredentials, instance)

	durationSeconds := int64(time.Since(startedAt).Seconds())
	crashed := waitErr != nil || exitCode != 0
	if crashed {
		slog.Warn("game exited with an error", "instance", instance.Name, "exitCode", exitCode, "error", waitErr)
	} else {
		slog.Info("game exited", "instance", instance.Name, "exitCode", exitCode, "seconds", durationSeconds)
	}
	// A crash inside the startup window is a failed startup: compare the
	// current configuration with the Last Known Good state and offer a safe
	// recovery. A game that ran past the window (or exited normally) is not a
	// configuration failure, no matter how it ended.
	if crashed && time.Since(startedAt) < startupWindow {
		s.handleFailedLaunch(instance)
	}
	if err := s.sessions.Finish(
		context.Background(),
		sessionID,
		exitCode,
		crashed,
		durationSeconds,
	); err != nil {
		slog.Warn("could not persist the finished session", "instance", instance.Name, "sessionId", sessionID, "error", err)
	}

	instance.Status = instances.StatusReady
	instance.UpdatedAt = time.Now().UTC()
	if err := s.store.SaveInstance(context.Background(), instance); err != nil {
		slog.Warn("could not persist the instance after the game exited", "instance", instance.Name, "error", err)
	}

	s.runningMu.Lock()
	delete(s.running, instance.ID)
	s.runningMu.Unlock()

	s.emit("game:exited", map[string]any{
		"instanceId":      instance.ID,
		"sessionId":       sessionID,
		"exitCode":        exitCode,
		"crashed":         crashed,
		"durationSeconds": durationSeconds,
	})
}
func (s *Service) Stop(ctx context.Context, instanceID string, force bool) error {
	slog.Info("stopping instance", "instance", instanceID, "force", force)
	s.runningMu.Lock()
	r, ok := s.running[instanceID]
	s.runningMu.Unlock()
	if !ok {
		return domain.NewError(domain.ErrValidation, "The instance is not running")
	}
	var e error
	if force {
		e = r.process.Kill()
	} else {
		e = r.process.Stop()
	}
	if e != nil {
		return &domain.AppError{Code: domain.ErrProcessStop, Message: "Failed to stop Vintage Story", Cause: e}
	}
	return nil
}
func (s *Service) RunningInstanceIDs() []string {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()
	ids := make([]string, 0, len(s.running))
	for id := range s.running {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
