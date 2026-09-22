package launching

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/mutations"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/versions"
)

type reconcileInstances struct{ instances []instances.Instance }

func (repository reconcileInstances) GetInstance(context.Context, string) (instances.Instance, error) {
	return instances.Instance{}, nil
}
func (repository reconcileInstances) ListInstances(context.Context) ([]instances.Instance, error) {
	return repository.instances, nil
}
func (reconcileInstances) SaveInstance(context.Context, instances.Instance) error { return nil }

type reconcileGate struct{}

func (reconcileGate) Begin() error { return nil }
func (reconcileGate) End()         {}

type reconcileLogs struct {
	calls  []string
	errors map[string]error
	order  *[]string
}

func (*reconcileLogs) Open(string) (io.WriteCloser, error) { return nil, nil }
func (logs *reconcileLogs) Harden(path string) error {
	name := filepath.Base(filepath.Dir(path))
	logs.calls = append(logs.calls, name)
	*logs.order = append(*logs.order, "harden "+name)
	return logs.errors[name]
}

type reconcileClientSettings struct {
	calls  []string
	errors map[string]error
	order  *[]string
}

func (*reconcileClientSettings) Inject(string, accounts.Account) (func() error, error) {
	return func() error { return nil }, nil
}
func (*reconcileClientSettings) Clear(string) error { return nil }
func (settings *reconcileClientSettings) Reconcile(path string) error {
	name := filepath.Base(filepath.Dir(path))
	settings.calls = append(settings.calls, name)
	if settings.order != nil {
		*settings.order = append(*settings.order, "reconcile "+name)
	}
	return settings.errors[name]
}

func TestReconcileInjectedCredentialsContinuesAfterFailure(t *testing.T) {
	want := errors.New("permission denied")
	coordinator, logs, settings := newReconcileCoordinator(t, map[string]error{"A": want}, nil)

	err := coordinator.ReconcileInjectedCredentials(context.Background())
	if !errors.Is(err, want) || !strings.Contains(err.Error(), `instance "A": harden logs`) {
		t.Fatalf("ReconcileInjectedCredentials() error = %v", err)
	}
	assertReconcileCalls(t, logs, settings)
}

func TestReconcileInjectedCredentialsJoinsAllFailures(t *testing.T) {
	hardenA := errors.New("harden A")
	reconcileA := errors.New("reconcile A")
	reconcileC := errors.New("reconcile C")
	coordinator, logs, settings := newReconcileCoordinator(
		t,
		map[string]error{"A": hardenA},
		map[string]error{"A": reconcileA, "C": reconcileC},
	)

	err := coordinator.ReconcileInjectedCredentials(context.Background())
	for _, want := range []error{hardenA, reconcileA, reconcileC} {
		if !errors.Is(err, want) {
			t.Fatalf("ReconcileInjectedCredentials() error = %v, missing %v", err, want)
		}
	}
	for _, context := range []string{`instance "A": harden logs`, `instance "A": reconcile client settings`, `instance "C": reconcile client settings`} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("ReconcileInjectedCredentials() error = %v, missing %q", err, context)
		}
	}
	assertReconcileCalls(t, logs, settings)
}

func TestReconcileInjectedCredentialsSucceedsForAllInstances(t *testing.T) {
	coordinator, logs, settings := newReconcileCoordinator(t, nil, nil)

	if err := coordinator.ReconcileInjectedCredentials(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertReconcileCalls(t, logs, settings)
}

type propagationInstances struct{ instance instances.Instance }

func (repository propagationInstances) GetInstance(context.Context, string) (instances.Instance, error) {
	return repository.instance, nil
}
func (repository propagationInstances) ListInstances(context.Context) ([]instances.Instance, error) {
	return nil, nil
}
func (propagationInstances) SaveInstance(context.Context, instances.Instance) error { return nil }

type propagationVersions struct{ version versions.GameVersion }

func (repository propagationVersions) Get(context.Context, string) (versions.GameVersion, error) {
	return repository.version, nil
}
func (repository propagationVersions) ResolveExecutable(context.Context, string) (versions.GameVersion, error) {
	return repository.version, nil
}

type propagationAccounts struct{ err error }

func (repository propagationAccounts) GetAccount(context.Context, string) (accounts.Account, error) {
	return accounts.Account{ID: "account", Status: accounts.StatusValid}, nil
}
func (propagationAccounts) ListAccounts(context.Context) ([]accounts.Account, error) { return nil, nil }
func (repository propagationAccounts) ValidateAuthorizedAccount(context.Context, string) (accounts.Account, error) {
	return accounts.Account{}, repository.err
}

func TestLaunchPropagatesAuthorizedAccountCredentialError(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "Vintagestory")
	if err := os.WriteFile(executable, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	want := &errs.AppError{Code: errs.ErrSecretStorage, Message: "safe credential-store error", Retryable: true}
	accountID := "account"
	coordinator := &Coordinator{
		registry:       NewRegistry(mutations.NewSlot()),
		gate:           reconcileGate{},
		instances:      propagationInstances{instance: instances.Instance{ID: "instance", Directory: t.TempDir(), GameVersionID: "version"}},
		versions:       propagationVersions{version: versions.GameVersion{ID: "version", ExecutablePath: executable}},
		accounts:       propagationAccounts{err: want},
		clientSettings: &reconcileClientSettings{},
	}

	_, err := coordinator.Launch(context.Background(), "instance", &accountID)

	if err != want {
		t.Fatalf("Launch() error = %v, want unchanged authorized-account error", err)
	}
}

func TestLaunchCommandLogOmitsArgumentValues(t *testing.T) {
	var log strings.Builder
	secret := "--token=launch-argument-secret"
	if err := writeLaunchCommand(&log, "/game/Vintagestory", []string{secret, "--dataPath", "/private/instance"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(log.String(), secret) || strings.Contains(log.String(), "/private/instance") {
		t.Fatalf("launch command log exposed argument values: %q", log.String())
	}
}

func TestReconcileInjectedCredentialsSkipsMissingInstanceDirectory(t *testing.T) {
	order := []string{}
	logs := &reconcileLogs{order: &order}
	settings := &reconcileClientSettings{order: &order}
	coordinator := &Coordinator{
		gate: reconcileGate{},
		instances: reconcileInstances{instances: []instances.Instance{
			{ID: "missing", Name: "Missing", Directory: filepath.Join(t.TempDir(), "gone")},
		}},
		logs:           logs,
		clientSettings: settings,
	}

	if err := coordinator.ReconcileInjectedCredentials(context.Background()); err != nil {
		t.Fatalf("ReconcileInjectedCredentials() error = %v, want nil for a missing instance directory", err)
	}
	if len(logs.calls) != 0 || len(settings.calls) != 0 {
		t.Fatalf("reconciled a skipped instance: harden=%v reconcile=%v", logs.calls, settings.calls)
	}
}

func newReconcileCoordinator(t *testing.T, hardenErrors, reconcileErrors map[string]error) (*Coordinator, *reconcileLogs, *reconcileClientSettings) {
	t.Helper()
	order := []string{}
	logs := &reconcileLogs{errors: hardenErrors, order: &order}
	settings := &reconcileClientSettings{errors: reconcileErrors, order: &order}
	root := t.TempDir()
	stored := []instances.Instance{
		{ID: "a", Name: "A", Directory: filepath.Join(root, "A")},
		{ID: "b", Name: "B", Directory: filepath.Join(root, "B")},
		{ID: "c", Name: "C", Directory: filepath.Join(root, "C")},
	}
	for _, instance := range stored {
		if err := os.MkdirAll(instance.Directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &Coordinator{
		gate:           reconcileGate{},
		instances:      reconcileInstances{instances: stored},
		logs:           logs,
		clientSettings: settings,
	}, logs, settings
}

func assertReconcileCalls(t *testing.T, logs *reconcileLogs, settings *reconcileClientSettings) {
	t.Helper()
	want := []string{"A", "B", "C"}
	if !reflect.DeepEqual(logs.calls, want) {
		t.Fatalf("Harden calls = %v, want %v", logs.calls, want)
	}
	if !reflect.DeepEqual(settings.calls, want) {
		t.Fatalf("Reconcile calls = %v, want %v", settings.calls, want)
	}
	wantOrder := []string{"reconcile A", "reconcile B", "reconcile C", "harden A", "harden B", "harden C"}
	if !reflect.DeepEqual(*logs.order, wantOrder) {
		t.Fatalf("reconciliation order = %v, want %v", *logs.order, wantOrder)
	}
}
