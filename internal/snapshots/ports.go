package snapshots

import (
	"context"
	"time"

	settingscore "github.com/waxlight/waxlight-launcher/internal/settings"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

// InstanceRef is the minimal instance view the snapshot feature reads. It
// keeps snapshots independent of the instances feature.
type InstanceRef struct {
	ID            string
	Name          string
	Directory     string
	GameVersionID string
}

// InstanceReader resolves instance records for snapshot orchestration.
type InstanceReader interface {
	GetInstance(context.Context, string) (InstanceRef, error)
}

// VersionReader resolves game versions so manifests store the display name.
type VersionReader interface {
	Get(context.Context, string) (versions.GameVersion, error)
}

// InstalledMod is the minimal installed-mod view the snapshot feature reads
// and writes. It keeps snapshots independent of the mods feature.
type InstalledMod struct {
	ID          string
	InstanceID  string
	Name        string
	Version     string
	FileName    string
	FilePath    string
	Enabled     bool
	Managed     bool
	Source      string
	SizeBytes   int64
	InstalledAt time.Time
	UpdatedAt   time.Time
}

// ModStore persists installed-mod records during exact restore.
type ModStore interface {
	ListMods(context.Context, string) ([]InstalledMod, error)
	SaveMod(context.Context, InstalledMod) error
	DeleteMod(context.Context, string) error
}

// DownloadedRelease is the subset of the catalog downloaded-mod cache the
// snapshot feature consumes. It keeps snapshots independent of the mods
// feature.
type DownloadedRelease struct {
	FilePath string
	Checksum string
	Name     string
	Slug     string
	Version  string
	FileName string
}

// Catalog downloads and inspects catalog releases for manifest enrichment and
// exact restore.
type Catalog interface {
	GetDownloadedMod(context.Context, string, string) (DownloadedRelease, error)
	DownloadRelease(context.Context, string, string) (DownloadedRelease, error)
}

// ArchiveInfo is the modinfo.json metadata of a restored mod archive.
type ArchiveInfo struct {
	ModID   string
	Version string
}

// ArchiveInfoReader inspects downloaded mod archives for identity validation.
type ArchiveInfoReader interface {
	ReadArchiveInfo(string) (ArchiveInfo, error)
}

// SettingsReader reads launcher preferences for the automatic safety-snapshot
// toggle.
type SettingsReader interface {
	Get(context.Context) (settingscore.Settings, error)
}

// MutationGate coordinates launcher-wide writes with data-root relocation.
type MutationGate interface {
	Begin() error
	End()
}

// DiskSpaceChecker verifies that the data volume can hold a snapshot or a
// restore.
type DiskSpaceChecker interface {
	Available(string) (int64, error)
}

// InstanceLock coordinates per-instance launches, destructive operations, and
// snapshots. Guard rejects the instance while its game is running; Lock
// reserves the slot without a running check. Marker identifies the operation.
type InstanceLock interface {
	Guard(instanceID string, marker string, runningMessage string) (func(), error)
	Lock(instanceID string, marker string) (func(), error)
	Running(instanceID string) bool
	Busy(instanceID string) bool
}

// InstanceSlot exposes the shared per-instance busy slot so snapshot creation
// can mark itself while the caller already holds a reservation.
type InstanceSlot interface {
	TryAcquire(instanceID string, marker string) (func(), string)
	Set(instanceID string, marker string) func()
	IsBusy(instanceID string) bool
}

// LastKnownGoodReference resolves and clears the snapshot reference of the
// Last Known Good marker. It is implemented by the recovery feature.
type LastKnownGoodReference interface {
	// ClearSnapshotReference drops the marker's snapshot reference after the
	// referenced snapshot is deleted.
	ClearSnapshotReference(context.Context, string, string)
	// ProtectedSnapshotID returns the snapshot referenced by the Last Known
	// Good marker so automatic retention never removes the active recovery
	// snapshot.
	ProtectedSnapshotID(context.Context, string) string
}

// SafetySnapshotter creates the automatic snapshot that protects an instance
// before a destructive change. A nil operation with no error means no snapshot
// was needed; any error must abort the destructive operation.
type SafetySnapshotter interface {
	Create(context.Context, string, Reason, map[string]string) error
}

// SafetySnapshotterFunc adapts a snapshot-creation function to the
// SafetySnapshotter port.
type SafetySnapshotterFunc func(context.Context, string, Reason, map[string]string) error

// Create implements SafetySnapshotter.
func (fn SafetySnapshotterFunc) Create(ctx context.Context, instanceID string, reason Reason, snapshotContext map[string]string) error {
	return fn(ctx, instanceID, reason, snapshotContext)
}

// ClientSettingsSanitizer removes temporary authentication and machine
// specific properties from a clientsettings.json document.
type ClientSettingsSanitizer func([]byte) ([]byte, error)

// ClientSettingsClearer removes temporary authentication properties from a
// clientsettings.json file on disk.
type ClientSettingsClearer func(string) error

// LogsHardener applies the launcher's log-directory hardening policy.
type LogsHardener func(string) error

// TotalSizeFunc enumerates the total size of a directory tree, honoring
// cancellation.
type TotalSizeFunc func(context.Context, string) (int64, error)

// DirectoryRemover deletes an owned directory, refusing unsafe paths.
type DirectoryRemover func(string) error

// Storage persists snapshots as directories with a manifest and a data tree.
// The concrete adapter lives under internal/platform/snapshots.
type Storage interface {
	List(context.Context, string) ([]InstanceSnapshot, error)
	ReadManifest(string) (Manifest, error)
	WriteManifest(string, Manifest) error
	SnapshotDir(string, string) (string, error)
	DataDir(string) string
	TempDir(string) (string, error)
	Remove(string, string) error
	Size(string) (int64, error)
}

// Publisher forwards launcher events to the frontend.
type Publisher interface {
	Publish(string, any)
}

// Clock returns the current time.
type Clock func() time.Time

// IDGenerator produces unique operation and record identifiers.
type IDGenerator func() string

// Marker constants must match the values used by the launching registry.
const (
	// MutationLockMarker is the per-instance marker held while a destructive
	// mutation (or its safety snapshot) is running.
	MutationLockMarker = "instance-mutation"
	// ReservationMarker is held while a snapshot operation reserves the
	// instance slot.
	ReservationMarker = "snapshot-reservation"
)

var _ SafetySnapshotter = SafetySnapshotterFunc(nil)

// InstanceReaderFunc adapts an instance-read function to the InstanceReader
// port.
type InstanceReaderFunc func(context.Context, string) (InstanceRef, error)

// GetInstance implements InstanceReader.
func (fn InstanceReaderFunc) GetInstance(ctx context.Context, instanceID string) (InstanceRef, error) {
	return fn(ctx, instanceID)
}
