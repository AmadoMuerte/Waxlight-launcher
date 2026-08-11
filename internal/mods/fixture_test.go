package mods_test

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/waxlight/waxlight-launcher/internal/snapshots"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/downloads"
	"github.com/waxlight/waxlight-launcher/internal/mods"
	"github.com/waxlight/waxlight-launcher/internal/mutations"
	"github.com/waxlight/waxlight-launcher/internal/operations"
	"github.com/waxlight/waxlight-launcher/internal/platform/filesystem"
	"github.com/waxlight/waxlight-launcher/internal/platform/modstorage"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

type testRepository struct {
	mu        sync.Mutex
	instances map[string]mods.InstanceRef
	mods      map[string]mods.InstalledMod
}

func newTestRepository() *testRepository {
	return &testRepository{
		instances: make(map[string]mods.InstanceRef),
		mods:      make(map[string]mods.InstalledMod),
	}
}

func (repository *testRepository) GetInstance(ctx context.Context, id string) (mods.InstanceRef, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	instance, ok := repository.instances[id]
	if !ok {
		return mods.InstanceRef{}, domain.NewError(domain.ErrValidation, "instance not found")
	}
	return instance, nil
}

func (repository *testRepository) ListInstances(ctx context.Context) ([]mods.InstanceRef, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	result := make([]mods.InstanceRef, 0, len(repository.instances))
	for _, instance := range repository.instances {
		result = append(result, instance)
	}
	return result, nil
}

func (repository *testRepository) ListMods(ctx context.Context, instanceID string) ([]mods.InstalledMod, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	var result []mods.InstalledMod
	for _, mod := range repository.mods {
		if mod.InstanceID == instanceID {
			result = append(result, mod)
		}
	}
	return result, nil
}

func (repository *testRepository) GetMod(ctx context.Context, id string) (mods.InstalledMod, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	mod, ok := repository.mods[id]
	if !ok {
		return mods.InstalledMod{}, domain.NewError(mods.ErrModNotFound, "Mod not found")
	}
	return mod, nil
}

func (repository *testRepository) SaveMod(ctx context.Context, mod mods.InstalledMod) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.mods[mod.ID] = mod
	return nil
}

func (repository *testRepository) DeleteMod(ctx context.Context, id string) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	delete(repository.mods, id)
	return nil
}

func (repository *testRepository) ListOperations(context.Context, int) ([]operations.Operation, error) {
	return nil, nil
}

func (repository *testRepository) SaveOperation(context.Context, operations.Operation) error {
	return nil
}

func (repository *testRepository) ReconcileInterruptedOperations(context.Context, time.Time, string, string) (int64, error) {
	return 0, nil
}

func (repository *testRepository) DeleteFinishedOperation(context.Context, string) error {
	return nil
}

func (repository *testRepository) ClearFinishedOperations(context.Context) (int64, error) {
	return 0, nil
}

type testWorkerOwner struct{}

func (testWorkerOwner) Go(func(context.Context)) bool { return false }

type testInstanceLock struct {
	slot *mutations.Slot
}

func (lock testInstanceLock) Lock(instanceID string, marker string) (func(), error) {
	release, holder := lock.slot.TryAcquire(instanceID, marker)
	if holder != "" {
		return nil, domain.NewError(snapshots.ErrSnapshotInProgress, "Wait for the running operation on this instance to finish")
	}
	return release, nil
}

type recordingSnapshotter struct {
	mu      sync.Mutex
	created int
}

func (snapshotter *recordingSnapshotter) Create(ctx context.Context, instanceID string, reason snapshots.Reason, snapshotContext map[string]string) error {
	snapshotter.mu.Lock()
	defer snapshotter.mu.Unlock()
	snapshotter.created++
	return nil
}

func (snapshotter *recordingSnapshotter) count() int {
	snapshotter.mu.Lock()
	defer snapshotter.mu.Unlock()
	return snapshotter.created
}

type recordedEvent struct {
	name    string
	payload any
}

type recordingEventPublisher struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (publisher *recordingEventPublisher) Publish(name string, payload any) {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	publisher.events = append(publisher.events, recordedEvent{name: name, payload: payload})
}

func (publisher *recordingEventPublisher) last(name string) (any, bool) {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	for index := len(publisher.events) - 1; index >= 0; index-- {
		if publisher.events[index].name == name {
			return publisher.events[index].payload, true
		}
	}
	return nil, false
}

func (publisher *recordingEventPublisher) clear() {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	publisher.events = nil
}

type staticVersionReader struct {
	version versions.GameVersion
}

func (reader staticVersionReader) Get(ctx context.Context, id string) (versions.GameVersion, error) {
	return reader.version, nil
}

type switchingDownloader struct {
	mu      sync.RWMutex
	current downloads.Downloader
}

func (downloader *switchingDownloader) Set(current downloads.Downloader) {
	downloader.mu.Lock()
	downloader.current = current
	downloader.mu.Unlock()
}

func (downloader *switchingDownloader) Download(ctx context.Context, request downloads.Request, progress chan<- downloads.Progress) error {
	downloader.mu.RLock()
	current := downloader.current
	downloader.mu.RUnlock()
	return current.Download(ctx, request, progress)
}

func (downloader *switchingDownloader) ContentLength(ctx context.Context, url string) (int64, error) {
	downloader.mu.RLock()
	current := downloader.current
	downloader.mu.RUnlock()
	return current.ContentLength(ctx, url)
}

type recordingDownloader struct{}

func (downloader recordingDownloader) Download(
	ctx context.Context,
	request downloads.Request,
	progress chan<- downloads.Progress,
) error {
	if err := os.MkdirAll(filepath.Dir(request.DestinationPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(request.DestinationPath, []byte("package"), 0o644); err != nil {
		return err
	}
	progress <- downloads.Progress{
		DownloadedBytes: 7,
		TotalBytes:      7,
		BytesPerSecond:  7,
	}
	return nil
}

func (downloader recordingDownloader) ContentLength(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

type modArchiveDownloader struct {
	manifests map[string]map[string]any
}

func (downloader modArchiveDownloader) Download(
	_ context.Context,
	request downloads.Request,
	progress chan<- downloads.Progress,
) error {
	if err := os.MkdirAll(filepath.Dir(request.DestinationPath), 0o755); err != nil {
		return err
	}
	file, err := os.Create(request.DestinationPath)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(file)
	modInfo, err := archive.Create("modinfo.json")
	if err != nil {
		_ = file.Close()
		return err
	}
	manifest := downloader.manifests[request.URL]
	if manifest == nil {
		manifest = map[string]any{
			"modid":        "testmod",
			"name":         "Test mod",
			"version":      "1.0.0",
			"dependencies": map[string]string{},
		}
	}
	if err := json.NewEncoder(modInfo).Encode(manifest); err != nil {
		_ = archive.Close()
		_ = file.Close()
		return err
	}
	if err := archive.Close(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	info, err := os.Stat(request.DestinationPath)
	if err != nil {
		return err
	}
	progress <- downloads.Progress{
		DownloadedBytes: info.Size(),
		TotalBytes:      info.Size(),
		BytesPerSecond:  info.Size(),
	}
	return nil
}

func (downloader modArchiveDownloader) ContentLength(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

type testFixture struct {
	repository     *testRepository
	modsService    *mods.Service
	catalogService *mods.CatalogService
	downloads      *modstorage.Store
	downloader     *switchingDownloader
	events         *recordingEventPublisher
	snapshots      *recordingSnapshotter
	lock           testInstanceLock
	root           string
	version        versions.GameVersion
}

func newTestFixture(t *testing.T) testFixture {
	return newTestFixtureWithDeps(t, nil, recordingDownloader{})
}

func newTestFixtureWithDeps(
	t *testing.T,
	catalog mods.Catalog,
	downloader downloads.Downloader,
) testFixture {
	t.Helper()
	root := t.TempDir()
	repository := newTestRepository()
	events := &recordingEventPublisher{}
	snapshots := &recordingSnapshotter{}
	slot := mutations.NewSlot()
	lock := testInstanceLock{slot: slot}
	gate := &mutations.Gate{}
	downloadedStore := modstorage.New(root)
	downloadSwitch := &switchingDownloader{current: downloader}
	version := versions.GameVersion{ID: "1.20", Name: "1.20", Status: "installed"}
	operationsManager := operations.NewManager(repository, testWorkerOwner{}, nil)
	installedService := mods.NewService(
		repository,
		filesystem.ModFileManager{},
		catalog,
		downloadedStore,
		operationsManager,
		gate,
		lock,
		snapshots,
		events,
		nil,
		time.Now,
		newTestID,
	)
	catalogService := mods.NewCatalogService(
		repository,
		filesystem.ModFileManager{},
		catalog,
		downloadedStore,
		downloadSwitch,
		staticVersionReader{version: version},
		installedService,
		gate,
		lock,
		snapshots,
		events,
		nil,
		mods.NewModTaskManager(events),
		time.Now,
		newTestID,
	)
	return testFixture{
		repository:     repository,
		modsService:    installedService,
		catalogService: catalogService,
		downloads:      downloadedStore,
		downloader:     downloadSwitch,
		events:         events,
		snapshots:      snapshots,
		lock:           lock,
		root:           root,
		version:        version,
	}
}

func newTestID() string {
	return fmt.Sprintf("id-%d", time.Now().UnixNano())
}

// createTestInstance creates an instance directory with the standard mod
// layout and registers it in the repository.
func (fixture testFixture) createTestInstance(t *testing.T, name string) mods.InstanceRef {
	t.Helper()
	instance := mods.InstanceRef{
		ID:            fmt.Sprintf("instance-%d", time.Now().UnixNano()),
		Name:          name,
		Directory:     filepath.Join(fixture.root, "instances", name),
		GameVersionID: "1.20",
	}
	if err := os.MkdirAll(filepath.Join(instance.Directory, "Mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(instance.Directory, "ModsDisabled"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.SaveMod(context.Background(), mods.InstalledMod{}); err == nil {
		// no-op to keep the repository exercised; instances are saved below
	}
	fixture.repository.instances[instance.ID] = instance
	return instance
}

// writeVintageStoryMod writes a mod archive with the given modinfo.json
// content.
func writeVintageStoryMod(t *testing.T, path, metadata string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create("modinfo.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(metadata)); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func installedModByName(items []mods.InstalledMod, name string) mods.InstalledMod {
	for _, mod := range items {
		if mod.Name == name {
			return mod
		}
	}
	return mods.InstalledMod{}
}

// staticModCatalog serves fixed catalog details.
type staticModCatalog struct {
	details     mods.ModDetails
	detailsByID map[string]mods.ModDetails
}

func (catalog staticModCatalog) List(context.Context) ([]mods.ModSummary, error) {
	if len(catalog.detailsByID) > 0 {
		items := make([]mods.ModSummary, 0, len(catalog.detailsByID))
		for _, details := range catalog.detailsByID {
			items = append(items, details.ModSummary)
		}
		return items, nil
	}
	return []mods.ModSummary{catalog.details.ModSummary}, nil
}

func (catalog staticModCatalog) Search(
	context.Context,
	mods.ModSearchQuery,
) (mods.ModSearchResult, error) {
	return mods.ModSearchResult{Items: []mods.ModSummary{catalog.details.ModSummary}, Page: 1, PageSize: 24, TotalItems: 1}, nil
}

func (catalog staticModCatalog) Get(_ context.Context, modID string) (mods.ModDetails, error) {
	if len(catalog.detailsByID) == 0 {
		return catalog.details, nil
	}
	if details, ok := catalog.detailsByID[modID]; ok {
		return details, nil
	}
	for _, details := range catalog.detailsByID {
		if details.ID == modID || details.Slug == modID {
			return details, nil
		}
	}
	return mods.ModDetails{}, domain.NewError(mods.ErrModNotFound, "Mod not found")
}

func (catalog staticModCatalog) ListTags(context.Context) ([]mods.ModTag, error) {
	return []mods.ModTag{}, nil
}

// twoReleaseMod builds catalog details for a mod with an old and a new
// release.
func twoReleaseMod(modID string, oldRelease, oldVersion, newRelease, newVersion string) mods.ModDetails {
	return mods.ModDetails{
		ModSummary: mods.ModSummary{
			ID:            modID,
			Name:          "Mod " + modID,
			LatestVersion: newVersion,
		},
		Versions: []mods.ModVersion{
			{ID: oldRelease, Version: oldVersion, FileName: modID + "_" + oldVersion + ".zip", DownloadURL: "https://mods.example/" + modID + "_" + oldVersion + ".zip"},
			{ID: newRelease, Version: newVersion, FileName: modID + "_" + newVersion + ".zip", DownloadURL: "https://mods.example/" + modID + "_" + newVersion + ".zip"},
		},
	}
}

// corpseCatalog builds the catalog entry of the Player Corpse mod.
func corpseCatalog() staticModCatalog {
	return staticModCatalog{details: mods.ModDetails{
		ModSummary: mods.ModSummary{
			ID:            "51",
			Slug:          "playercorpse",
			Name:          "Player Corpse",
			AuthorName:    "Ada",
			ModIDStrings:  []string{"playercorpse"},
			LatestVersion: "2.0.0",
		},
		Versions: []mods.ModVersion{{
			ID:           "7",
			Version:      "2.0.0",
			GameVersions: []string{"1.20"},
			ReleaseType:  "stable",
			FileName:     "playercorpse_2.0.0.zip",
			DownloadURL:  "https://mods.example/playercorpse_2.0.0.zip",
		}},
	}}
}
