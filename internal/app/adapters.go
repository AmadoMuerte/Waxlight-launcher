package app

import (
	"context"
	"strings"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/launching"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
	optimumfeature "github.com/AmadoMuerte/Waxlight-launcher/internal/optimum"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/discord"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/filesystem"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/logging"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/sqlite"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/presence"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/recovery"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/snapshots"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/supportreports"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/telemetry"
)

type optimumLaunchAdapter struct {
	service *optimumfeature.Service
}

type discordPresenceClient struct {
	client *discord.Client
}

func (adapter discordPresenceClient) Connected() bool { return adapter.client.Connected() }

func (adapter discordPresenceClient) SetActivity(activity presence.Activity) error {
	return adapter.client.SetActivity(discord.Activity{
		State:          activity.State,
		Details:        activity.Details,
		LargeImageKey:  activity.LargeImageKey,
		LargeImageText: activity.LargeImageText,
		SmallImageKey:  activity.SmallImageKey,
		SmallImageText: activity.SmallImageText,
		StartTimestamp: activity.StartTimestamp,
	})
}

func (adapter discordPresenceClient) ClearActivity() error { return adapter.client.ClearActivity() }
func (adapter discordPresenceClient) Close()               { adapter.client.Close() }

type supportLogAdapter struct{}

func (supportLogAdapter) Lines() []string {
	entries := logging.Snapshot()
	lines := make([]string, len(entries))
	for i, entry := range entries {
		lines[i] = entry.Plain()
	}
	return lines
}

type supportRecoveryAdapter struct {
	recovery  *recovery.Service
	snapshots *snapshots.Service
}

func (adapter supportRecoveryAdapter) Summary(ctx context.Context, instanceID string) (bool, int) {
	status, err := adapter.recovery.Status(ctx, instanceID)
	exists := err == nil && !status.RecordedAt.IsZero()
	listed, err := adapter.snapshots.List(ctx, instanceID)
	if err != nil {
		return exists, 0
	}
	return exists, len(listed)
}

type supportSenderAdapter struct{ client *telemetry.Client }

func (adapter supportSenderAdapter) SendSupportReport(ctx context.Context, report supportreports.Report) (supportreports.Result, error) {
	result, err := adapter.client.SendSupportReport(ctx, report)
	return supportreports.Result{ReportID: result.ReportID, Status: result.Status}, err
}

func (adapter optimumLaunchAdapter) Resolve(configuredPath, vanillaDirectory string) (launching.OptimumTarget, error) {
	installation, err := adapter.service.Resolve(configuredPath, vanillaDirectory)
	return launching.OptimumTarget{
		Executable: installation.Executable, WorkingDirectory: installation.WorkingDirectory,
		Exclusive: installation.Exclusive,
	}, err
}

type enabledModAdapter struct {
	files filesystem.ModFileManager
}

func (adapter enabledModAdapter) HasEnabledMod(instanceDirectory, modID string) (bool, error) {
	found, err := adapter.files.Scan(instanceDirectory)
	if err != nil {
		return false, err
	}
	for _, mod := range found {
		if mod.Enabled && strings.EqualFold(strings.TrimSpace(mod.ModID), modID) {
			return true, nil
		}
	}
	return false, nil
}

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
