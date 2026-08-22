package instances

import (
	"context"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/mods"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

type QueryRepository interface {
	ListInstances(context.Context) ([]Instance, error)
	GetInstance(context.Context, string) (Instance, error)
}

type CreateRepository interface {
	ListInstances(context.Context) ([]Instance, error)
	SaveInstance(context.Context, Instance) error
	IsDirectoryUsed(context.Context, string, string) (bool, error)
}

type UpdateRepository interface {
	GetInstance(context.Context, string) (Instance, error)
	SaveInstance(context.Context, Instance) error
}

type DeleteRepository interface {
	GetInstance(context.Context, string) (Instance, error)
	DeleteInstance(context.Context, string) error
}

type CloneRepository interface {
	GetInstance(context.Context, string) (Instance, error)
	SaveInstance(context.Context, Instance) error
	DeleteInstance(context.Context, string) error
}

type CloneModRepository interface {
	ListMods(context.Context, string) ([]mods.InstalledMod, error)
	SaveMod(context.Context, mods.InstalledMod) error
}

type Repository interface {
	QueryRepository
	CreateRepository
	DeleteInstance(context.Context, string) error
}

type VersionReader interface {
	Get(context.Context, string) (versions.GameVersion, error)
}

type AccountReader interface {
	GetAccount(context.Context, string) (accounts.Account, error)
}

type MutationGate interface {
	Begin() error
	End()
}

type DirectoryStorage interface {
	Allocate(directory, instanceID string) (DirectoryAllocation, error)
}

type DirectoryAllocation interface {
	Directory() string
	Commit()
	Rollback() error
}

type InstanceCreator interface {
	Create(context.Context, CreateInput) (Instance, error)
}

type CloneStorage interface {
	Copy(context.Context, string, string) error
	CopiedPath(string, string, string) (string, bool)
}

type Publisher interface {
	Publish(string, any)
}

type PublishFunc func(string, any)

func (publish PublishFunc) Publish(name string, payload any) {
	publish(name, payload)
}

type VersionChangePreparer func(context.Context, Instance, Instance) (func(), error)
type ClientSettingsClearer func(string) error
type DeleteGuard func(string) (func(), error)
type CloneGuard func(string) (func(), error)
type DirectoryRemover func(string) error
type DirectoryRemovalStager func(string) (restore func() error, remove func() error, err error)
type RecoveryCleaner func(context.Context, string) error
type LanguageFunc func(context.Context) (string, error)
type TelemetryFunc func(context.Context, string)
type Clock func() time.Time
type IDGenerator func() string

// MutationMarker is the per-instance busy slot marker held while a destructive
// instance mutation (or its safety snapshot) is running.
const MutationMarker = "instance-mutation"

// MutationLock coordinates per-instance launches, destructive operations, and
// snapshots. Guard rejects the instance while its game is running; Lock
// reserves the slot without a running check. Marker identifies the operation.
type MutationLock interface {
	Guard(instanceID string, marker string, runningMessage string) (func(), error)
	Lock(instanceID string, marker string) (func(), error)
}

// PackageRepository is the instance persistence surface used by package
// import and export.
type PackageRepository interface {
	GetInstance(context.Context, string) (Instance, error)
	SaveInstance(context.Context, Instance) error
	DeleteInstance(context.Context, string) error
}

// PackageVersionReader resolves game versions for package inspection and
// import.
type PackageVersionReader interface {
	Get(context.Context, string) (versions.GameVersion, error)
	List(context.Context) ([]versions.GameVersion, error)
	ListAvailable(context.Context) ([]versions.AvailableGameVersion, error)
	InstallCatalogAndWait(context.Context, string) (versions.GameVersion, error)
}

// PackageModStore persists installed mod records during package import.
type PackageModStore interface {
	ListMods(context.Context, string) ([]mods.InstalledMod, error)
	SaveMod(context.Context, mods.InstalledMod) error
}

// PackageCatalog resolves catalog mod details for package inspection.
type PackageCatalog interface {
	Get(context.Context, string) (mods.ModDetails, error)
}

// PackageDownloadedMods reads cached catalog mod downloads for package
// export enrichment.
type PackageDownloadedMods interface {
	Get(context.Context, string, string) (mods.DownloadedMod, error)
}

// CatalogModInstaller installs catalog mods into instances and toggles enable
// state. It stays a transitional delegate until the mods feature owns it.
type CatalogModInstaller interface {
	DownloadCatalogMod(context.Context, mods.DownloadModRequest) (mods.ModInstallResult, error)
	RemoveDownloadedModsIfUnused(context.Context, []mods.DownloadedMod) error
	SetModEnabled(context.Context, string, bool) (mods.InstalledMod, error)
}

// PackageIO reads and writes portable instance packages. The infrastructure
// adapter owns the archive format; the feature stays format-independent.
type PackageIO interface {
	Open(path string) (PackageArchive, error)
	Write(context.Context, string, PackageWriteSource) error
}

// PackageArchive exposes the contents of an opened package without leaking the
// archive implementation.
type PackageArchive interface {
	Manifest() PackageManifest
	TotalSize() int64
	ExtractConfigs(context.Context, string) error
	ExtractEmbeddedMod(context.Context, string, string) error
	ExtractIcon(context.Context, string) error
}

// PackageWriteSource is the feature-owned input for package writing.
type PackageWriteSource struct {
	Manifest     PackageManifest
	InstanceDir  string
	EmbeddedMods map[string]string
	IconPath     string
}
