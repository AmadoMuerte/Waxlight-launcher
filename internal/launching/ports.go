package launching

import (
	"context"
	"io"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/mutations"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/operations"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/process"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/sessions"
	settingscore "github.com/AmadoMuerte/Waxlight-launcher/internal/settings"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/versions"
)

type InstanceReader interface {
	GetInstance(context.Context, string) (instances.Instance, error)
	ListInstances(context.Context) ([]instances.Instance, error)
	SaveInstance(context.Context, instances.Instance) error
}

type VersionReader interface {
	Get(context.Context, string) (versions.GameVersion, error)
	ResolveExecutable(context.Context, string) (versions.GameVersion, error)
}

type AccountReader interface {
	GetAccount(context.Context, string) (accounts.Account, error)
	ListAccounts(context.Context) ([]accounts.Account, error)
	ValidateAuthorizedAccount(context.Context, string) (accounts.Account, error)
}

type ClientSettingsPatcher interface {
	Inject(path string, account accounts.Account) (func() error, error)
	Clear(path string) error
	Reconcile(path string) error
}

type SettingsReader interface {
	Get(context.Context) (settingscore.Settings, error)
}

type ModLayout interface {
	EnsureLayout(string) error
}

type OptimumTarget struct {
	Executable       string
	WorkingDirectory string
	Exclusive        bool
}

type OptimumResolver interface {
	Resolve(configuredPath, vanillaDirectory string) (OptimumTarget, error)
}

type EnabledModChecker interface {
	HasEnabledMod(instanceDirectory, modID string) (bool, error)
}

type SessionRecorder interface {
	Create(context.Context, sessions.PlaySession) error
	Finish(context.Context, string, int, bool, int64) error
}

type ProcessLauncher interface {
	Start(
		ctx context.Context,
		executable string,
		args []string,
		workingDir string,
		env map[string]string,
		output io.Writer,
	) (process.Running, error)
}

// LaunchLogs opens and hardens launcher-owned instance log files.
type LaunchLogs interface {
	Open(path string) (io.WriteCloser, error)
	Harden(directory string) error
}

// TelemetryReporter forwards allowlisted launch events and error categories.
// Implementations are strictly best-effort and never affect launch outcomes.
type TelemetryReporter interface {
	Event(context.Context, string)
	Error(context.Context, string, string, string)
}

// LaunchRecovery records the Last Known Good state of a successful launch and
// assesses failed startups. It stays a narrow port until the recovery feature
// owns the orchestration.
type LaunchRecovery interface {
	RecordLastKnownGood(context.Context, instances.Instance)
	HandleFailedLaunch(instances.Instance)
}

// WorkerGroup starts work derived from the application lifecycle context.
type WorkerGroup interface {
	Go(func(context.Context)) bool
}

// OperationLister lists tracked operations for the data folder relocation
// busy check.
type OperationLister interface {
	ListLimit(context.Context, int) ([]operations.Operation, error)
}

// InstanceLock coordinates per-instance launches, destructive operations, and
// snapshots. Marker identifies the operation holding the slot.
type InstanceLock interface {
	Guard(instanceID string, marker string, runningMessage string) (func(), error)
	Lock(instanceID string, marker string) (func(), error)
	Busy(instanceID string) bool
	Running(instanceID string) bool
}

var _ InstanceLock = (*Registry)(nil)

type Publisher interface {
	Publish(string, any)
}

type MutationGate interface {
	Begin() error
	End()
}

type IDGenerator func() string

type Clock func() time.Time

// InstanceSlot exposes the shared per-instance busy slot so snapshot creation
// can mark itself while the caller already holds a reservation.
type InstanceSlot interface {
	TryAcquire(instanceID string, marker string) (func(), string)
	IsBusy(instanceID string) bool
	BusyMarker(instanceID string) string
}

var _ InstanceSlot = (*mutations.Slot)(nil)
