package application

import (
	"context"
	"errors"
	"io"

	"github.com/waxlight/waxlight-launcher/internal/auth"
	"github.com/waxlight/waxlight-launcher/internal/domain"
)

var (
	ErrSecretNotFound   = errors.New("account secret not found")
	ErrStoreLocked      = errors.New("credential store locked")
	ErrStoreUnavailable = errors.New("credential store unavailable")
	ErrPermissionDenied = errors.New("credential store permission denied")
	ErrCorruptSecret    = errors.New("stored account secret is corrupt")
)

type Secret struct {
	SessionKey       string `json:"-"`
	SessionSignature string `json:"-"`
}

type SecretStore interface {
	Get(context.Context, string) (Secret, error)
	Set(context.Context, string, Secret) error
	Delete(context.Context, string) error
}

type PendingSecretStore interface {
	SecretStore
	MarkPending(context.Context, string) error
	ClearPending(context.Context, string) error
}

type AuthClient interface {
	Login(
		context.Context,
		string,
		string,
		string,
		string,
	) (auth.Session, *auth.TOTPChallenge, error)
	Validate(context.Context, string, string) (bool, error)
}

type ClientSettingsPatcher interface {
	Inject(path string, account domain.Account) (func() error, error)
	Clear(path string) error
	Reconcile(path string) error
}

type Store interface {
	Close() error
	ListAccounts(context.Context) ([]domain.Account, error)
	GetAccount(context.Context, string) (domain.Account, error)
	SaveAccount(context.Context, domain.Account) error
	SetDefaultAccount(context.Context, string) error
	DeleteAccount(context.Context, string) error

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
}

// AccountCommitter atomically persists account metadata and its default
// selection. Production metadata stores must implement this boundary.
type AccountCommitter interface {
	SaveAccountAndSelect(context.Context, domain.Account, bool) error
}

type ArchiveInstaller interface {
	Install(
		ctx context.Context,
		sourcePath string,
		targetPath string,
		executableRelativePath string,
		expectedSHA256 string,
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

type EventPublisher interface {
	Publish(name string, payload any)
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
}

type GameVersionCatalog interface {
	List(context.Context) ([]domain.AvailableGameVersion, error)
}

type GamePackageInstaller interface {
	Install(
		ctx context.Context,
		sourcePath string,
		targetPath string,
	) (executablePath string, size int64, err error)
}

type DiskSpaceChecker interface {
	Available(path string) (int64, error)
}

type ModCatalog interface {
	Search(context.Context, domain.ModSearchQuery) (domain.ModSearchResult, error)
	Get(context.Context, string) (domain.ModDetails, error)
}

type DownloadedModStore interface {
	List(context.Context) ([]domain.DownloadedMod, error)
	Get(context.Context, string, string) (domain.DownloadedMod, error)
	Save(context.Context, domain.DownloadedMod) error
	Delete(context.Context, string, string) error
	FilePath(string, string, string) (string, error)
}
