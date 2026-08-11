package mods

import (
	"context"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/downloads"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

// Repository persists installed mod records and resolves instances for mod
// orchestration.
type Repository interface {
	GetInstance(context.Context, string) (InstanceRef, error)
	ListInstances(context.Context) ([]InstanceRef, error)
	ListMods(context.Context, string) ([]InstalledMod, error)
	GetMod(context.Context, string) (InstalledMod, error)
	SaveMod(context.Context, InstalledMod) error
	DeleteMod(context.Context, string) error
}

// DownloadedStore persists the downloaded-mod cache.
type DownloadedStore interface {
	List(context.Context) ([]DownloadedMod, error)
	Get(context.Context, string, string) (DownloadedMod, error)
	Save(context.Context, DownloadedMod) error
	Delete(context.Context, string, string) error
	FilePath(string, string, string) (string, error)
}

// Catalog is the ModDB catalog read surface backed by vintagestory-go.
type Catalog interface {
	List(context.Context) ([]ModSummary, error)
	Search(context.Context, ModSearchQuery) (ModSearchResult, error)
	Get(context.Context, string) (ModDetails, error)
	ListTags(context.Context) ([]ModTag, error)
}

// FileManager owns the instance mod layout on disk.
type FileManager interface {
	EnsureLayout(string) error
	Scan(string) ([]DiscoveredMod, error)
	Install(context.Context, string, string) (string, int64, error)
	InstallOrReplace(context.Context, string, string, string) (string, int64, error)
	SetEnabled(string, string, bool) (string, error)
}

// VersionReader resolves the game version of an instance.
type VersionReader interface {
	Get(context.Context, string) (versions.GameVersion, error)
}

// MutationGate coordinates launcher-wide writes with data-root relocation.
type MutationGate interface {
	Begin() error
	End()
}

// MutationLockMarker is the per-instance busy slot marker held while a
// destructive mod change (or its safety snapshot) is running. It must match
// the marker value used by the launching registry.
const MutationLockMarker = "instance-mutation"

// InstanceLock reserves the per-instance busy slot so destructive operations
// and launches of the same instance never overlap.
type InstanceLock interface {
	Lock(instanceID string, marker string) (func(), error)
}

// SafetySnapshotter creates the automatic snapshot protecting an instance
// before a destructive mod change. A nil error without a snapshot means no
// snapshot was needed; any error must abort the destructive operation.
type SafetySnapshotter interface {
	Create(context.Context, string, domain.SnapshotReason, map[string]string) error
}

// SafetySnapshotterFunc adapts a snapshot-creation function to the
// SafetySnapshotter port.
type SafetySnapshotterFunc func(context.Context, string, domain.SnapshotReason, map[string]string) error

func (fn SafetySnapshotterFunc) Create(ctx context.Context, instanceID string, reason domain.SnapshotReason, context map[string]string) error {
	return fn(ctx, instanceID, reason, context)
}

// Publisher forwards launcher events to the frontend.
type Publisher interface {
	Publish(string, any)
}

// PublishFunc adapts a publish function to the Publisher port.
type PublishFunc func(string, any)

func (publish PublishFunc) Publish(name string, payload any) {
	publish(name, payload)
}

// Telemetry reports allowlisted events and error categories. Both are
// best-effort and must never affect operation outcomes.
type Telemetry interface {
	Event(ctx context.Context, name string)
	Error(ctx context.Context, code, component, operation string)
}

// InstalledModLister lists the reconciled installed mods of an instance. The
// concrete implementation lives in the installed-mod service so the catalog
// service never duplicates discovery or reconciliation.
type InstalledModLister interface {
	ListMods(context.Context, string) ([]InstalledMod, error)
}

// Downloader fetches catalog mod files with progress and checksum
// verification.
type Downloader interface {
	Download(context.Context, downloads.Request, chan<- downloads.Progress) error
	ContentLength(context.Context, string) (int64, error)
}

// Clock returns the current time.
type Clock func() time.Time

// IDGenerator produces unique record identifiers.
type IDGenerator func() string
