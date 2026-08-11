package application

import (
	"context"

	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/mods"
	"github.com/waxlight/waxlight-launcher/internal/snapshots"
)

// snapshotInstanceAdapter maps the shared store to the snapshot instance
// reader port, converting instance records into the minimal instance view of
// the snapshot feature.
type snapshotInstanceAdapter struct {
	service *Service
}

func (adapter snapshotInstanceAdapter) GetInstance(ctx context.Context, id string) (snapshots.InstanceRef, error) {
	instance, err := adapter.service.GetInstance(ctx, id)
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
	service *Service
}

func (adapter snapshotModStoreAdapter) ListMods(ctx context.Context, instanceID string) ([]snapshots.InstalledMod, error) {
	mods, err := adapter.service.mods.ListMods(ctx, instanceID)
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
	return adapter.service.store.SaveMod(ctx, mods.InstalledMod{
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
	return adapter.service.store.DeleteMod(ctx, id)
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
	service *Service
}

func (adapter snapshotCatalogAdapter) GetDownloadedMod(ctx context.Context, modID, versionID string) (snapshots.DownloadedRelease, error) {
	cached, err := adapter.service.modsCatalog.GetDownloadedMod(ctx, modID, versionID)
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
	downloaded, err := adapter.service.modsCatalog.DownloadRelease(ctx, modID, versionID)
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
type snapshotArchiveInfoAdapter struct {
	service *Service
}

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
	service *Service
}

func (adapter snapshotLKGReferenceAdapter) ClearSnapshotReference(ctx context.Context, instanceID, snapshotID string) {
	if adapter.service.recovery != nil {
		adapter.service.recovery.ClearSnapshotReference(ctx, instanceID, snapshotID)
	}
}

func (adapter snapshotLKGReferenceAdapter) ProtectedSnapshotID(ctx context.Context, instanceID string) string {
	if adapter.service.recovery == nil {
		return ""
	}
	return adapter.service.recovery.ProtectedSnapshotID(ctx, instanceID)
}
