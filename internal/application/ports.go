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

type LauncherUpdateSource interface {
	Check(context.Context, string, string) (domain.LauncherUpdate, error)
}

type LauncherUpdateInstaller interface {
	Apply(ctx context.Context, installerPath string, currentPID int) error
}

type SignatureVerifier interface {
	Verify(ctx context.Context, executablePath string) error
}

type PublicServerCatalog interface {
	List(context.Context) ([]domain.PublicServer, error)
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
