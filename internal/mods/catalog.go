package mods

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/snapshots"
)

const (
	maxModDownloadBytes int64 = 512 << 20
)

// CatalogService owns ModDB browsing, catalog downloads with dependency
// resolution, the downloaded-mod cache, local-mod linking, and update
// analysis and application.
type CatalogService struct {
	repository  Repository
	files       FileManager
	catalog     Catalog
	downloads   DownloadedStore
	downloader  Downloader
	versions    VersionReader
	lister      InstalledModLister
	gate        MutationGate
	lock        InstanceLock
	snapshotter snapshots.SafetySnapshotter
	events      Publisher
	telemetry   Telemetry
	tasks       *ModTaskManager
	now         Clock
	newID       IDGenerator
}

// NewCatalogService wires the catalog service with immutable dependencies.
func NewCatalogService(
	repository Repository,
	files FileManager,
	catalog Catalog,
	downloads DownloadedStore,
	downloader Downloader,
	versions VersionReader,
	lister InstalledModLister,
	gate MutationGate,
	lock InstanceLock,
	snapshotter snapshots.SafetySnapshotter,
	events Publisher,
	telemetry Telemetry,
	tasks *ModTaskManager,
	now Clock,
	newID IDGenerator,
) *CatalogService {
	return &CatalogService{
		repository:  repository,
		files:       files,
		catalog:     catalog,
		downloads:   downloads,
		downloader:  downloader,
		versions:    versions,
		lister:      lister,
		gate:        gate,
		lock:        lock,
		snapshotter: snapshotter,
		events:      events,
		telemetry:   telemetry,
		tasks:       tasks,
		now:         now,
		newID:       newID,
	}
}

// SearchMods searches the ModDB catalog and annotates every result with the
// downloaded, installed, and update state from the library.
func (service *CatalogService) SearchMods(ctx context.Context, query ModSearchQuery) (ModSearchResult, error) {
	result, err := service.catalog.Search(ctx, query)
	if err != nil {
		return result, err
	}
	downloaded, _ := service.listDownloadedMods(ctx)
	byID := make(map[string]DownloadedMod, len(downloaded))
	for _, item := range downloaded {
		current, exists := byID[item.ModID]
		if !exists || item.DownloadedAt.After(current.DownloadedAt) {
			byID[item.ModID] = item
		}
	}
	for index := range result.Items {
		if local, ok := byID[result.Items[index].ID]; ok {
			result.Items[index].IsDownloaded = true
			result.Items[index].IsInstalled = len(local.InstalledInstances) > 0
			result.Items[index].UpdateAvailable = local.UpdateAvailable
		}
	}
	return result, nil
}

// ListModTags lists the catalog tags with their mod counts.
func (service *CatalogService) ListModTags(ctx context.Context) ([]ModTag, error) {
	return service.catalog.ListTags(ctx)
}

// GetCatalogMod returns the full catalog record of a mod, enriched with the
// library state and file sizes of versions that are not cached yet.
func (service *CatalogService) GetCatalogMod(ctx context.Context, modID string) (ModDetails, error) {
	details, err := service.catalog.Get(ctx, modID)
	if err != nil {
		return details, err
	}
	downloaded, _ := service.listDownloadedMods(ctx)
	for _, local := range downloaded {
		if local.ModID == details.ID {
			details.IsDownloaded = true
			details.IsInstalled = details.IsInstalled || len(local.InstalledInstances) > 0
			details.UpdateAvailable = details.UpdateAvailable || local.UpdateAvailable
			for index := range details.Versions {
				if details.Versions[index].ID == local.VersionID && local.FileSize > 0 {
					details.Versions[index].FileSize = local.FileSize
				}
			}
		}
	}

	// Fetch file sizes for versions not yet in cache.
	type sizeResult struct {
		index int
		size  int64
	}
	sizeJobs := make(chan int)
	sizeResults := make(chan sizeResult, len(details.Versions))
	workers := 4
	if workers > len(details.Versions) {
		workers = len(details.Versions)
	}
	var sizeWait sync.WaitGroup
	for range workers {
		sizeWait.Add(1)
		go func() {
			defer sizeWait.Done()
			for index := range sizeJobs {
				v := details.Versions[index]
				if v.FileSize != 0 || !strings.HasPrefix(v.DownloadURL, "https://") {
					continue
				}
				headCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				size, err := service.downloader.ContentLength(headCtx, v.DownloadURL)
				cancel()
				if err == nil && size > 0 {
					sizeResults <- sizeResult{index: index, size: size}
				}
			}
		}()
	}
	go func() {
		for index := 0; index < len(details.Versions); index++ {
			sizeJobs <- index
		}
		close(sizeJobs)
		sizeWait.Wait()
		close(sizeResults)
	}()
	for result := range sizeResults {
		details.Versions[result.index].FileSize = result.size
	}

	return details, nil
}

// listDownloadedMods returns the cached downloads annotated with the
// instances where each release is installed.
func (service *CatalogService) listDownloadedMods(ctx context.Context) ([]DownloadedMod, error) {
	items, err := service.downloads.List(ctx)
	if err != nil {
		return nil, err
	}
	instances, err := service.repository.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	installedBySource := make(map[string][]InstalledModInstance)
	for _, instance := range instances {
		mods, listErr := service.repository.ListMods(ctx, instance.ID)
		if listErr != nil {
			continue
		}
		for _, installed := range mods {
			modID, versionID, ok := ParseModDBSource(installed.Source)
			if !ok {
				continue
			}
			key := modDownloadKey(modID, versionID)
			installedBySource[key] = append(installedBySource[key], InstalledModInstance{
				InstanceID:   instance.ID,
				InstanceName: instance.Name,
				Version:      installed.Version,
				Enabled:      installed.Enabled,
			})
		}
	}
	for index := range items {
		items[index].InstalledInstances = installedBySource[modDownloadKey(items[index].ModID, items[index].VersionID)]
	}
	sort.Slice(items, func(left, right int) bool {
		return items[left].DownloadedAt.After(items[right].DownloadedAt)
	})
	return items, nil
}

// ListDownloadedMods returns the cached downloads annotated with the
// instances where each release is installed.
func (service *CatalogService) ListDownloadedMods(ctx context.Context) ([]DownloadedMod, error) {
	return service.listDownloadedMods(ctx)
}

// CheckModUpdates refreshes the latest-version and update state of every
// cached release of a mod.
func (service *CatalogService) CheckModUpdates(ctx context.Context, modID string) ([]DownloadedMod, error) {
	if err := service.gate.Begin(); err != nil {
		return nil, err
	}
	defer service.gate.End()
	details, err := service.catalog.Get(ctx, modID)
	if err != nil {
		return nil, err
	}
	items, err := service.downloads.List(ctx)
	if err != nil {
		return nil, err
	}
	for index := range items {
		if items[index].ModID != details.ID {
			continue
		}
		items[index].LatestVersion = details.LatestVersion
		items[index].UpdateAvailable = details.LatestVersion != "" &&
			details.LatestVersion != items[index].DownloadedVersion
		if err := service.downloads.Save(ctx, items[index]); err != nil {
			return nil, err
		}
	}
	return service.listDownloadedMods(ctx)
}

// GetDownloadedMod reads a cached catalog download.
func (service *CatalogService) GetDownloadedMod(ctx context.Context, modID, versionID string) (DownloadedMod, error) {
	return service.downloads.Get(ctx, modID, versionID)
}

// DownloadCatalogMod downloads a catalog mod, resolves its dependencies, and
// installs it into the requested instances.
func (service *CatalogService) DownloadCatalogMod(
	ctx context.Context,
	request DownloadModRequest,
) (ModInstallResult, error) {
	return service.downloadCatalogMod(ctx, request, nil)
}

// downloadCatalogMod is the shared download implementation. lockedInstances
// lists instances whose mutation slot is already held by the caller.
func (service *CatalogService) downloadCatalogMod(
	ctx context.Context,
	request DownloadModRequest,
	lockedInstances map[string]struct{},
) (ModInstallResult, error) {
	if err := service.gate.Begin(); err != nil {
		return ModInstallResult{}, err
	}
	defer service.gate.End()

	details, err := service.catalog.Get(ctx, request.ModID)
	if err != nil {
		return ModInstallResult{}, err
	}
	selected, ok := FindModVersion(details.Versions, request.VersionID)
	if !ok {
		return ModInstallResult{}, errs.NewError(ErrModVersionNotFound, "Mod version not found")
	}
	if !strings.HasPrefix(selected.DownloadURL, "https://") {
		return ModInstallResult{}, errs.NewError(ErrInvalidModFile, "The catalog returned an unsafe download URL")
	}
	slog.Info("downloading catalog mod", "mod", details.Name, "version", selected.Version, "instances", len(request.InstanceIDs))

	targetInstances := make([]InstanceRef, 0, len(request.InstanceIDs))
	gameVersions := make([]string, 0, len(request.InstanceIDs))
	seenInstances := make(map[string]struct{}, len(request.InstanceIDs))
	seenGameVersions := make(map[string]struct{}, len(request.InstanceIDs))
	for _, instanceID := range request.InstanceIDs {
		if _, duplicate := seenInstances[instanceID]; duplicate {
			continue
		}
		seenInstances[instanceID] = struct{}{}

		instance, getErr := service.repository.GetInstance(ctx, instanceID)
		if getErr != nil {
			return ModInstallResult{}, getErr
		}
		version, versionErr := service.versions.Get(ctx, instance.GameVersionID)
		if versionErr != nil {
			return ModInstallResult{}, versionErr
		}
		gameVersion := version.Name
		if gameVersion == "" {
			gameVersion = version.ID
		}
		if !request.AllowIncompatible && !ModSupportsVersion(selected.GameVersions, gameVersion) {
			return ModInstallResult{}, errs.NewError(
				ErrModIncompatible,
				fmt.Sprintf("%s does not list support for Vintage Story %s", details.Name, gameVersion),
			)
		}
		if _, seen := seenGameVersions[gameVersion]; !seen {
			seenGameVersions[gameVersion] = struct{}{}
			gameVersions = append(gameVersions, gameVersion)
		}
		targetInstances = append(targetInstances, instance)
	}

	taskID := service.newID()
	downloadCtx, existingTask, err := service.tasks.Begin(ctx, taskID, details.ID, selected.ID)
	if err != nil {
		return ModInstallResult{TaskID: existingTask}, err
	}
	defer service.tasks.Release(taskID)

	plan := make([]modInstallPlanItem, 0, 4)
	resolved := make(map[string]ModVersion)
	visiting := make(map[string]struct{})
	downloaded, err := service.resolveAndDownloadCatalogMod(
		downloadCtx,
		taskID,
		details,
		selected,
		gameVersions,
		request.AllowIncompatible,
		true,
		resolved,
		visiting,
		&plan,
	)
	if err != nil {
		service.tasks.EmitProgress(taskID, details.ID, phaseFailed, 0, 0, 0, dependencyFailureMessage(err))
		return ModInstallResult{TaskID: taskID}, err
	}

	result := ModInstallResult{TaskID: taskID, Downloaded: downloaded}
	defer service.tasks.EmitDownloadsChanged(taskID, details.ID, newlyDownloadedDependencies(plan))
	if request.DownloadOnly || len(targetInstances) == 0 {
		service.tasks.EmitProgress(taskID, details.ID, phaseComplete, downloaded.FileSize, downloaded.FileSize, 1, messageDownload)
		return result, nil
	}

	allInstalled := true
	for _, instance := range targetInstances {
		var instanceRelease func()
		if _, alreadyLocked := lockedInstances[instance.ID]; !alreadyLocked {
			instanceRelease, err = service.lockInstanceMutations(instance.ID)
			if err != nil {
				return result, err
			}
		}
		installation := service.installModPlan(downloadCtx, plan, instance)
		if instanceRelease != nil {
			instanceRelease()
		}
		result.Installations = append(result.Installations, installation)
		allInstalled = allInstalled && installation.Installed
	}
	if !allInstalled {
		service.tasks.EmitProgress(taskID, details.ID, phaseFailed, downloaded.FileSize, downloaded.FileSize, 1, messageInstallErr)
		return result, nil
	}
	service.tasks.EmitProgress(taskID, details.ID, phaseComplete, downloaded.FileSize, downloaded.FileSize, 1, messageInstall)
	return result, nil
}

// DownloadCatalogModsBatch continues after individual target failures so users
// receive an outcome for every catalog mod selected for one instance.
func (service *CatalogService) DownloadCatalogModsBatch(
	ctx context.Context,
	request BatchDownloadModsRequest,
) []BatchModInstallResult {
	if err := service.gate.Begin(); err != nil {
		results := make([]BatchModInstallResult, 0, len(request.Targets))
		for _, target := range request.Targets {
			results = append(results, BatchModInstallResult{ModID: target.ModID, VersionID: target.VersionID, Error: err.Error()})
		}
		return results
	}
	defer service.gate.End()
	results := make([]BatchModInstallResult, 0, len(request.Targets))
	for _, target := range request.Targets {
		result, err := service.DownloadCatalogMod(ctx, DownloadModRequest{
			ModID:             target.ModID,
			VersionID:         target.VersionID,
			InstanceIDs:       []string{request.InstanceID},
			AllowIncompatible: true,
		})
		item := BatchModInstallResult{
			ModID:     target.ModID,
			VersionID: target.VersionID,
			Result:    result,
		}
		if err != nil {
			item.Error = err.Error()
		}
		results = append(results, item)
	}
	return results
}

// InstallDownloadedMod installs an already cached catalog mod release into the
// requested instances.
func (service *CatalogService) InstallDownloadedMod(
	ctx context.Context,
	modID string,
	versionID string,
	instanceIDs []string,
	allowIncompatible bool,
) (ModInstallResult, error) {
	return service.DownloadCatalogMod(ctx, DownloadModRequest{
		ModID:             modID,
		VersionID:         versionID,
		InstanceIDs:       instanceIDs,
		AllowIncompatible: allowIncompatible,
	})
}

// CancelModTask cancels the ModDB download task with the given ID.
func (service *CatalogService) CancelModTask(taskID string) error {
	return service.tasks.Cancel(taskID)
}

// removeCatalogModFromInstallPlan drops the plan items of a mod whose release
// was replaced by a newer shared-dependency version.
func removeCatalogModFromInstallPlan(plan *[]modInstallPlanItem, modID string) {
	items := (*plan)[:0]
	for _, item := range *plan {
		if strings.EqualFold(item.Downloaded.ModID, modID) {
			continue
		}
		items = append(items, item)
	}
	*plan = items
}

// newlyDownloadedDependencies collects the non-root releases downloaded by a
// task for the mods:downloads-changed event.
func newlyDownloadedDependencies(plan []modInstallPlanItem) []modDownloadedDependencyEvent {
	dependencies := make([]modDownloadedDependencyEvent, 0)
	for _, item := range plan {
		if item.Root || !item.DownloadedNow {
			continue
		}
		dependencies = append(dependencies, modDownloadedDependencyEvent{
			ModID:   item.Downloaded.ModID,
			Name:    item.Downloaded.Name,
			Version: item.Downloaded.DownloadedVersion,
		})
	}
	return dependencies
}

// removeSupersededCacheVersion deletes a cached mod version that was replaced
// by an update, unless another instance still has that version installed.
func (service *CatalogService) removeSupersededCacheVersion(ctx context.Context, modID, versionID string) {
	source := ModDBSource(modID, versionID)
	instances, err := service.repository.ListInstances(ctx)
	if err != nil {
		slog.Warn("could not check superseded cache version", "modId", modID, "versionId", versionID, "error", err)
		return
	}
	for _, instance := range instances {
		mods, err := service.repository.ListMods(ctx, instance.ID)
		if err != nil {
			slog.Warn("could not check superseded cache version", "instance", instance.Name, "error", err)
			return
		}
		for _, mod := range mods {
			if mod.Source == source {
				return
			}
		}
	}
	slog.Info("superseded cached mod version removed", "modId", modID, "versionId", versionID)
	if err := service.downloads.Delete(ctx, modID, versionID); err != nil {
		slog.Warn("could not delete the superseded cached mod version", "modId", modID, "versionId", versionID, "error", err)
	}
}

// RemoveDownloadedMod removes one cached mod release from the library.
func (service *CatalogService) RemoveDownloadedMod(ctx context.Context, modID, versionID string) error {
	if err := service.gate.Begin(); err != nil {
		return err
	}
	defer service.gate.End()
	slog.Info("cached mod removed", "modId", modID, "versionId", versionID)
	if err := service.downloads.Delete(ctx, modID, versionID); err != nil {
		return err
	}
	service.reportEvent(ctx, EventModRemoved)
	return nil
}

// PreviewUnusedDownloadedMods reports how much space cleaning unused cached
// mods would free.
func (service *CatalogService) PreviewUnusedDownloadedMods(ctx context.Context) (DownloadedModCleanupResult, error) {
	items, err := service.unusedDownloadedMods(ctx)
	if err != nil {
		return DownloadedModCleanupResult{}, err
	}
	return downloadedModCleanupResult(items), nil
}

// RemoveUnusedDownloadedMods deletes every cached mod release that is not
// installed anywhere and not downloading.
func (service *CatalogService) RemoveUnusedDownloadedMods(ctx context.Context) (DownloadedModCleanupResult, error) {
	if err := service.gate.Begin(); err != nil {
		return DownloadedModCleanupResult{}, err
	}
	defer service.gate.End()
	items, err := service.unusedDownloadedMods(ctx)
	if err != nil {
		return DownloadedModCleanupResult{}, err
	}
	for _, item := range items {
		if err := service.downloads.Delete(ctx, item.ModID, item.VersionID); err != nil {
			return DownloadedModCleanupResult{}, err
		}
		service.reportEvent(ctx, EventModRemoved)
	}
	return downloadedModCleanupResult(items), nil
}

// unusedDownloadedMods lists cached releases that are neither installed nor
// downloading.
func (service *CatalogService) unusedDownloadedMods(ctx context.Context) ([]DownloadedMod, error) {
	items, err := service.downloads.List(ctx)
	if err != nil {
		return nil, err
	}
	instances, err := service.repository.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	installed := make(map[string]struct{})
	for _, instance := range instances {
		mods, listErr := service.repository.ListMods(ctx, instance.ID)
		if listErr != nil {
			return nil, listErr
		}
		for _, mod := range mods {
			modID, versionID, ok := ParseModDBSource(mod.Source)
			if ok {
				installed[modDownloadKey(modID, versionID)] = struct{}{}
			}
		}
	}
	unused := make([]DownloadedMod, 0, len(items))
	for _, item := range items {
		if _, used := installed[modDownloadKey(item.ModID, item.VersionID)]; used {
			continue
		}
		if service.tasks.IsDownloading(item.ModID, item.VersionID) {
			continue
		}
		unused = append(unused, item)
	}
	return unused, nil
}

func downloadedModCleanupResult(items []DownloadedMod) DownloadedModCleanupResult {
	result := DownloadedModCleanupResult{RemovedCount: len(items)}
	for _, item := range items {
		result.FreedBytes += item.FileSize
	}
	return result
}

// lockInstanceMutations reserves the per-instance mutation slot. The returned
// release function must be called exactly once when the operation finishes.
func (service *CatalogService) lockInstanceMutations(instanceID string) (func(), error) {
	return service.lock.Lock(instanceID, MutationLockMarker)
}

func (service *CatalogService) reportEvent(ctx context.Context, name string) {
	if service.telemetry != nil {
		service.telemetry.Event(ctx, name)
	}
}

func (service *CatalogService) reportError(ctx context.Context, code, component, operation string) {
	if service.telemetry != nil {
		service.telemetry.Error(ctx, code, component, operation)
	}
}

// reportModDownloadError classifies a mod download failure into a structured
// telemetry error. The downloader reports HTTP statuses as
// "download returned HTTP <code>"; only the category is transmitted.
func (service *CatalogService) reportModDownloadError(err error) {
	code := ErrorModDownloadFailed
	if status := downloadHTTPStatus(err); status == 404 {
		code = ErrorModDownloadHTTP404
	}
	service.reportError(context.Background(), code, ComponentModDownloader, OperationDownloadMod)
}

func downloadHTTPStatus(err error) int {
	message := err.Error()
	marker := "HTTP "
	if index := strings.LastIndex(message, marker); index >= 0 {
		if status, parseErr := strconv.Atoi(strings.TrimSpace(message[index+len(marker):])); parseErr == nil {
			return status
		}
	}
	return 0
}
