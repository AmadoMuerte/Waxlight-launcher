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
	"strings"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/downloads"
	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/events"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/launching"
	"github.com/waxlight/waxlight-launcher/internal/mods"
	"github.com/waxlight/waxlight-launcher/internal/mutations"
	"github.com/waxlight/waxlight-launcher/internal/operations"
	"github.com/waxlight/waxlight-launcher/internal/recovery"
	"github.com/waxlight/waxlight-launcher/internal/sessions"
	settingscore "github.com/waxlight/waxlight-launcher/internal/settings"
	"github.com/waxlight/waxlight-launcher/internal/snapshots"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

type Service struct {
	store           Store
	clientSettings  ClientSettingsPatcher
	downloader      downloads.Downloader
	diskSpace       DiskSpaceChecker
	versions        VersionCapabilities
	dataRoot        string
	snapshots       *snapshots.Service
	recovery        *recovery.Service
	events          events.Publisher
	telemetry       mods.Telemetry
	mutationGate    *mutations.Gate
	settings        *settingscore.Reader
	operations      *operations.Manager
	sessions        *sessions.Service
	instanceQueries *instances.QueryService
	instanceCreator *instances.CreateService
	instanceUpdater *instances.UpdateService
	instanceDeleter *instances.DeleteService
	instanceCloner  *instances.CloneService
	instanceSlot    *mutations.Slot
	launchRegistry  *launching.Registry
	mods            *mods.Service
	modsCatalog     *mods.CatalogService
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
	modFiles mods.FileManager,
	dataRoot string,
	snapshotStorage snapshots.Storage,
	totalSize snapshots.TotalSizeFunc,
	sanitizeSettings snapshots.ClientSettingsSanitizer,
	hardenLogs snapshots.LogsHardener,
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
	modCatalog mods.Catalog,
	modDownloads mods.DownloadedStore,
	publisher events.Publisher,
	telemetryService mods.Telemetry,
) *Service {
	service := &Service{
		store:           store,
		dataRoot:        dataRoot,
		operations:      operationManager,
		sessions:        sessionService,
		instanceQueries: instanceQueries,
		instanceCreator: instanceCreator,
		versions:        versionService,
		downloader:      downloader,
		diskSpace:       diskSpace,
		mutationGate:    mutationGate,
		settings:        settingsReader,
		instanceSlot:    instanceSlot,
		launchRegistry:  launchRegistry,
	}
	clearClientSettings := func(path string) error {
		if service.clientSettings == nil {
			return nil
		}
		return service.clientSettings.Clear(path)
	}
	service.snapshots = snapshots.NewService(
		snapshotStorage,
		snapshotInstanceAdapter{service: service},
		versionService,
		snapshotModStoreAdapter{service: service},
		snapshotCatalogAdapter{service: service},
		snapshotArchiveInfoAdapter{service: service},
		settingsReader,
		operationManager,
		mutationGate,
		instanceSlot,
		launchRegistry,
		diskSpace,
		totalSize,
		sanitizeSettings,
		hardenLogs,
		clearClientSettings,
		func(path string) error { return safeRemoveAll(path, dataRoot, ".waxlight-instance") },
		snapshotLKGReferenceAdapter{service: service},
		dataRoot,
		time.Now,
		newID,
	)
	service.recovery = recovery.NewService(
		store,
		service.snapshots,
		service.snapshots,
		store,
		mutationGate,
		publisher,
		time.Now,
	)
	snapshotter := snapshots.SafetySnapshotterFunc(func(ctx context.Context, instanceID string, reason snapshots.Reason, snapshotContext map[string]string) error {
		_, err := service.snapshots.CreateSafety(ctx, instanceID, reason, snapshotContext)
		return err
	})
	repository := modsStoreAdapter{store: store}
	service.mods = mods.NewService(
		repository,
		modFiles,
		modCatalog,
		modDownloads,
		operationManager,
		mutationGate,
		launchRegistry,
		snapshotter,
		mods.PublishFunc(func(name string, payload any) {
			if publisher != nil {
				publisher.Publish(name, payload)
			}
		}),
		telemetryService,
		time.Now,
		newID,
	)
	service.modsCatalog = mods.NewCatalogService(
		repository,
		modFiles,
		modCatalog,
		modDownloads,
		downloader,
		versionService,
		service.mods,
		mutationGate,
		launchRegistry,
		snapshotter,
		mods.PublishFunc(func(name string, payload any) {
			if publisher != nil {
				publisher.Publish(name, payload)
			}
		}),
		telemetryService,
		mods.NewModTaskManager(mods.PublishFunc(func(name string, payload any) {
			if publisher != nil {
				publisher.Publish(name, payload)
			}
		})),
		time.Now,
		newID,
	)
	service.instanceUpdater = instances.NewUpdateService(
		store,
		versionService,
		mutationGate,
		launchRegistry,
		snapshotter,
		clearClientSettings,
		instances.PublishFunc(service.emit),
		time.Now,
	)
	reportEvent := func(ctx context.Context, name string) {
		if telemetryService != nil {
			telemetryService.Event(ctx, name)
		}
	}
	service.instanceDeleter = instances.NewDeleteService(
		store,
		mutationGate,
		launchRegistry,
		func(path string) error { return safeRemoveAll(path, dataRoot, ".waxlight-instance") },
		clearClientSettings,
		store.DeleteLastKnownGood,
		instances.PublishFunc(service.emit),
		reportEvent,
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

// Mods returns the installed-mod service owned by the application layer.
func (s *Service) Mods() *mods.Service {
	return s.mods
}

// ModsCatalog returns the catalog service owned by the application layer.
func (s *Service) ModsCatalog() *mods.CatalogService {
	return s.modsCatalog
}

// Snapshots returns the snapshot service owned by the application layer.
func (s *Service) Snapshots() *snapshots.Service {
	return s.snapshots
}

// Recovery returns the Last Known Good recovery service owned by the
// application layer.
func (s *Service) Recovery() *recovery.Service {
	return s.recovery
}

func (s *Service) SetEventPublisher(publisher events.Publisher) {
	s.events = publisher
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
		return "", errs.NewError(errs.ErrValidation, "Name cannot be empty")
	}
	if len([]rune(v)) > 80 {
		return "", errs.NewError(errs.ErrValidation, "Name cannot exceed 80 characters")
	}
	return v, nil
}

func isAppErrorCode(err error, code string) bool {
	var appError *errs.AppError
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
		return errs.NewError(errs.ErrValidation, "Unsafe deletion path")
	}
	if _, e = os.Stat(filepath.Join(abs, marker)); e != nil {
		return errs.NewError(errs.ErrValidation, "The directory is not managed by Waxlight; no files were deleted")
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
