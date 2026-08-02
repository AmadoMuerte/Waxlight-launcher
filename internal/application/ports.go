package application

import (
	"context"
	"errors"
	"io"

	"github.com/waxlight/waxlight-launcher/internal/auth"
	"github.com/waxlight/waxlight-launcher/internal/domain"
)

var ErrSecretNotFound = errors.New("account secret not found")

type Secret struct {
	SessionKey       string `json:"-"`
	SessionSignature string `json:"-"`
}

type SecretStore interface {
	Get(accountID string) (Secret, error)
	Set(accountID string, secret Secret) error
	Delete(accountID string) error
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
	Patch(path string, account domain.Account) error
	Clear(path string) error
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

	GetSettings(context.Context) (domain.Settings, error)
	SaveSettings(context.Context, domain.Settings) error
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
	Install(context.Context, string, string) (string, int64, error)
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
	URL             string
	DestinationPath string
	ExpectedSHA256  string
	Resume          bool
}

type DownloadProgress struct {
	DownloadedBytes int64
	TotalBytes      int64
	BytesPerSecond  int64
}

type Downloader interface {
	Download(context.Context, DownloadRequest, chan<- DownloadProgress) error
}
