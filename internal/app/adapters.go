package app

import (
	"context"

	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/mods"
	"github.com/waxlight/waxlight-launcher/internal/platform/sqlite"
	"github.com/waxlight/waxlight-launcher/internal/recovery"
	"github.com/waxlight/waxlight-launcher/internal/snapshots"
)

// modsStoreAdapter maps the shared store to the mods repository port,
// converting instance records into the minimal instance view of the mods
// feature.
type modsStoreAdapter struct {
	store *sqlite.SQLiteStore
}

func (adapter modsStoreAdapter) GetInstance(ctx context.Context, id string) (mods.InstanceRef, error) {
	instance, err := adapter.store.GetInstance(ctx, id)
	return modsInstanceRef(instance), err
}

func (adapter modsStoreAdapter) ListInstances(ctx context.Context) ([]mods.InstanceRef, error) {
	stored, err := adapter.store.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]mods.InstanceRef, 0, len(stored))
	for _, instance := range stored {
		result = append(result, modsInstanceRef(instance))
	}
	return result, nil
}

func modsInstanceRef(instance instances.Instance) mods.InstanceRef {
	return mods.InstanceRef{
		ID:            instance.ID,
		Name:          instance.Name,
		Directory:     instance.Directory,
		GameVersionID: instance.GameVersionID,
	}
}

func (adapter modsStoreAdapter) ListMods(ctx context.Context, instanceID string) ([]mods.InstalledMod, error) {
	return adapter.store.ListMods(ctx, instanceID)
}

func (adapter modsStoreAdapter) GetMod(ctx context.Context, id string) (mods.InstalledMod, error) {
	return adapter.store.GetMod(ctx, id)
}

func (adapter modsStoreAdapter) SaveMod(ctx context.Context, mod mods.InstalledMod) error {
	return adapter.store.SaveMod(ctx, mod)
}

func (adapter modsStoreAdapter) DeleteMod(ctx context.Context, id string) error {
	return adapter.store.DeleteMod(ctx, id)
}

// snapshotInstanceAdapter maps the shared store to the snapshot instance
// reader port, converting instance records into the minimal instance view of
// the snapshot feature.
type snapshotInstanceAdapter struct {
	store *sqlite.SQLiteStore
}

func (adapter snapshotInstanceAdapter) GetInstance(ctx context.Context, id string) (snapshots.InstanceRef, error) {
	instance, err := adapter.store.GetInstance(ctx, id)
	return snapshotInstanceRef(instance), err
}

func snapshotInstanceRef(instance instances.Instance) snapshots.InstanceRef {
	return snapshots.InstanceRef{
		ID:            instance.ID,
		Name:          instance.Name,
		Directory:     instance.Directory,
		GameVersionID: instance.GameVersionID,
	}
}

// snapshotModStoreAdapter maps the installed-mod service and the shared store
// to the snapshot mod-store port. Listing goes through the reconciling mods
// service so disk state is imported exactly like the rest of the launcher;
// saving and deleting touch the raw store because restore rebuilds records
// without touching the restored files.
type snapshotModStoreAdapter struct {
	store *sqlite.SQLiteStore
	mods  func() *mods.Service
}

func (adapter snapshotModStoreAdapter) ListMods(ctx context.Context, instanceID string) ([]snapshots.InstalledMod, error) {
	mods, err := adapter.mods().ListMods(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	result := make([]snapshots.InstalledMod, 0, len(mods))
	for _, mod := range mods {
		result = append(result, snapshotInstalledMod(mod))
	}
	return result, nil
}

func (adapter snapshotModStoreAdapter) SaveMod(ctx context.Context, mod snapshots.InstalledMod) error {
	return adapter.store.SaveMod(ctx, mods.InstalledMod{
		ID:          mod.ID,
		InstanceID:  mod.InstanceID,
		Name:        mod.Name,
		Version:     mod.Version,
		FileName:    mod.FileName,
		FilePath:    mod.FilePath,
		Enabled:     mod.Enabled,
		Managed:     mod.Managed,
		Source:      mod.Source,
		SizeBytes:   mod.SizeBytes,
		InstalledAt: mod.InstalledAt,
		UpdatedAt:   mod.UpdatedAt,
	})
}

func (adapter snapshotModStoreAdapter) DeleteMod(ctx context.Context, id string) error {
	return adapter.store.DeleteMod(ctx, id)
}

func snapshotInstalledMod(mod mods.InstalledMod) snapshots.InstalledMod {
	return snapshots.InstalledMod{
		ID:          mod.ID,
		InstanceID:  mod.InstanceID,
		Name:        mod.Name,
		Version:     mod.Version,
		FileName:    mod.FileName,
		FilePath:    mod.FilePath,
		Enabled:     mod.Enabled,
		Managed:     mod.Managed,
		Source:      mod.Source,
		SizeBytes:   mod.SizeBytes,
		InstalledAt: mod.InstalledAt,
		UpdatedAt:   mod.UpdatedAt,
	}
}

// snapshotCatalogAdapter maps the catalog service to the snapshot catalog
// port, converting downloaded releases between their mods and snapshot
// representations.
type snapshotCatalogAdapter struct {
	catalog func() *mods.CatalogService
}

func (adapter snapshotCatalogAdapter) GetDownloadedMod(ctx context.Context, modID, versionID string) (snapshots.DownloadedRelease, error) {
	cached, err := adapter.catalog().GetDownloadedMod(ctx, modID, versionID)
	if err != nil {
		return snapshots.DownloadedRelease{}, err
	}
	return snapshots.DownloadedRelease{
		FilePath: cached.FilePath,
		Checksum: cached.Checksum,
		Name:     cached.Name,
		Slug:     cached.Slug,
		Version:  cached.DownloadedVersion,
		FileName: cached.FileName,
	}, nil
}

func (adapter snapshotCatalogAdapter) DownloadRelease(ctx context.Context, modID, versionID string) (snapshots.DownloadedRelease, error) {
	downloaded, err := adapter.catalog().DownloadRelease(ctx, modID, versionID)
	if err != nil {
		return snapshots.DownloadedRelease{}, err
	}
	return snapshots.DownloadedRelease{
		FilePath: downloaded.FilePath,
		Checksum: downloaded.Checksum,
		Name:     downloaded.Name,
	}, nil
}

// snapshotArchiveInfoAdapter maps the mods archive inspection to the snapshot
// archive-info port.
type snapshotArchiveInfoAdapter struct{}

func (adapter snapshotArchiveInfoAdapter) ReadArchiveInfo(filePath string) (snapshots.ArchiveInfo, error) {
	info, err := mods.ReadModArchiveInfo(filePath)
	if err != nil {
		return snapshots.ArchiveInfo{}, err
	}
	return snapshots.ArchiveInfo{ModID: info.ModID, Version: info.Version}, nil
}

// snapshotLKGReferenceAdapter maps the recovery feature to the snapshot
// Last-Known-Good reference port. The recovery service is wired after the
// snapshot service, so the lazy lookup is guarded.
type snapshotLKGReferenceAdapter struct {
	recovery func() *recovery.Service
}

func (adapter snapshotLKGReferenceAdapter) ClearSnapshotReference(ctx context.Context, instanceID, snapshotID string) {
	if service := adapter.recovery(); service != nil {
		service.ClearSnapshotReference(ctx, instanceID, snapshotID)
	}
}

func (adapter snapshotLKGReferenceAdapter) ProtectedSnapshotID(ctx context.Context, instanceID string) string {
	if service := adapter.recovery(); service != nil {
		return service.ProtectedSnapshotID(ctx, instanceID)
	}
	return ""
}
