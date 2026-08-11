package versions

import (
	"context"

	"github.com/waxlight/waxlight-launcher/internal/downloads"
)

type Repository interface {
	ListVersions(context.Context) ([]GameVersion, error)
	GetVersion(context.Context, string) (GameVersion, error)
	SaveVersion(context.Context, GameVersion) error
	UpdateVersion(context.Context, GameVersion) error
	DeleteVersion(context.Context, string) error
}

type References interface {
	VersionReference(context.Context, string) (name string, found bool, err error)
}

type Catalog interface {
	List(context.Context) ([]AvailableGameVersion, error)
}

type LocalInstaller interface {
	Install(context.Context, string, string, string, string, func(int64, int64)) (string, int64, error)
	FindExecutable(string, string) (string, error)
}

type PackageInstaller interface {
	Install(context.Context, string, string, func(int64, int64)) (string, int64, error)
}

type DiskSpace interface {
	Available(string) (int64, error)
}

type Filesystem interface {
	DownloadPath(string) string
	VersionPath(string) string
	ExecutableExists(string) bool
	MakeExecutable(string) error
	WriteMarker(string, string) error
	RemoveDownload(string) error
	RemoveInstallTarget(string, string) error
	RemoveVersion(string, string) error
	RemoveVersionsRootIfEmpty(string) error
}

type MutationGate interface {
	Begin() error
	End()
}

type Downloader interface {
	downloads.Downloader
}
