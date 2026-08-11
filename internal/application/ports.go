package application

import (
	"context"
	"io"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/domain"
)

type ClientSettingsPatcher interface {
	Inject(path string, account accounts.Account) (func() error, error)
	Clear(path string) error
	Reconcile(path string) error
}

type Store interface {
	Close() error
	ListVersions(context.Context) ([]domain.GameVersion, error)
	GetVersion(context.Context, string) (domain.GameVersion, error)
	SaveVersion(context.Context, domain.GameVersion) error
	UpdateVersion(context.Context, domain.GameVersion) error
	DeleteVersion(context.Context, string) error

	ListInstances(context.Context) ([]domain.Instance, error)
	GetInstance(context.Context, string) (domain.Instance, error)
	SaveInstance(context.Context, domain.Instance) error
	DeleteInstance(context.Context, string) error
	IsDirectoryUsed(context.Context, string, string) (bool, error)

	ListFavoriteServers(context.Context) ([]domain.FavoriteServer, error)
	GetFavoriteServer(context.Context, string) (domain.FavoriteServer, error)
	SaveFavoriteServer(context.Context, domain.FavoriteServer) error
	DeleteFavoriteServer(context.Context, string) error

	GetLastKnownGood(context.Context, string) (domain.LastKnownGood, error)
	SaveLastKnownGood(context.Context, domain.LastKnownGood) error
	DeleteLastKnownGood(context.Context, string) error

	ListMods(context.Context, string) ([]domain.InstalledMod, error)
	GetMod(context.Context, string) (domain.InstalledMod, error)
	SaveMod(context.Context, domain.InstalledMod) error
	DeleteMod(context.Context, string) error

	SaveSession(context.Context, domain.PlaySession) error
	FinishSession(context.Context, string, int, bool, int64) error
	ListSessions(context.Context, string, int) ([]domain.PlaySession, error)

	ListOperations(context.Context, int) ([]domain.Operation, error)
	SaveOperation(context.Context, domain.Operation) error
	DeleteFinishedOperation(context.Context, string) error
	ClearFinishedOperations(context.Context) (int64, error)

	GetSettings(context.Context) (domain.Settings, error)
	SaveSettings(context.Context, domain.Settings) error

	GetSettingValue(context.Context, string) (string, error)
	SetSettingValue(context.Context, string, string) error
}

type ArchiveInstaller interface {
	Install(
		ctx context.Context,
		sourcePath string,
		targetPath string,
		executableRelativePath string,
		expectedSHA256 string,
		progress func(copied, total int64),
	) (executablePath string, size int64, err error)
	FindExecutable(rootPath string, relativePath string) (string, error)
}

type ModFileManager interface {
	EnsureLayout(string) error
	Scan(string) ([]domain.DiscoveredMod, error)
	Install(context.Context, string, string) (string, int64, error)
	InstallOrReplace(context.Context, string, string, string) (string, int64, error)
	SetEnabled(string, string, bool) (string, error)
}

type RunningProcess interface {
	PID() int
	Wait() (int, error)
	Stop() error
	Kill() error
}

type ProcessLauncher interface {
	Start(
		ctx context.Context,
		executable string,
		args []string,
		workingDir string,
		env map[string]string,
		output io.Writer,
	) (RunningProcess, error)
}

type DownloadRequest struct {
	URL               string
	DestinationPath   string
	ExpectedChecksum  string
	ChecksumAlgorithm string
	Resume            bool
	MaxBytes          int64
}

type DownloadProgress struct {
	DownloadedBytes int64
	TotalBytes      int64
	BytesPerSecond  int64
}

type Downloader interface {
	Download(context.Context, DownloadRequest, chan<- DownloadProgress) error
	ContentLength(ctx context.Context, url string) (int64, error)
}

type LauncherUpdateSource interface {
	Check(context.Context, string, string) (domain.LauncherUpdate, error)
}

type LauncherUpdateInstaller interface {
	Apply(ctx context.Context, installerPath string, currentPID int) error
}

type SignatureVerifier interface {
	Verify(ctx context.Context, executablePath string) error
}

type GameVersionCatalog interface {
	List(context.Context) ([]domain.AvailableGameVersion, error)
}

type PublicServerCatalog interface {
	List(context.Context) ([]domain.PublicServer, error)
}

type GamePackageInstaller interface {
	Install(
		ctx context.Context,
		sourcePath string,
		targetPath string,
		progress func(copied, total int64),
	) (executablePath string, size int64, err error)
}

type DiskSpaceChecker interface {
	Available(path string) (int64, error)
}

type ModCatalog interface {
	List(context.Context) ([]domain.ModSummary, error)
	Search(context.Context, domain.ModSearchQuery) (domain.ModSearchResult, error)
	Get(context.Context, string) (domain.ModDetails, error)
	ListTags(context.Context) ([]domain.ModTag, error)
}

type DownloadedModStore interface {
	List(context.Context) ([]domain.DownloadedMod, error)
	Get(context.Context, string, string) (domain.DownloadedMod, error)
	Save(context.Context, domain.DownloadedMod) error
	Delete(context.Context, string, string) error
	FilePath(string, string, string) (string, error)
}
