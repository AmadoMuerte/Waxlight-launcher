package launching

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
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
}

func (*reconcileLogs) Open(string) (io.WriteCloser, error) { return nil, nil }
func (logs *reconcileLogs) Harden(path string) error {
	name := filepath.Base(filepath.Dir(path))
	logs.calls = append(logs.calls, name)
	return logs.errors[name]
}

type reconcileClientSettings struct {
	calls  []string
	errors map[string]error
}

func (*reconcileClientSettings) Inject(string, accounts.Account) (func() error, error) {
	return func() error { return nil }, nil
}
func (*reconcileClientSettings) Clear(string) error { return nil }
func (settings *reconcileClientSettings) Reconcile(path string) error {
	name := filepath.Base(filepath.Dir(path))
	settings.calls = append(settings.calls, name)
	return settings.errors[name]
}

func TestReconcileInjectedCredentialsContinuesAfterFailure(t *testing.T) {
	want := errors.New("permission denied")
	coordinator, logs, settings := newReconcileCoordinator(map[string]error{"A": want}, nil)

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
	coordinator, logs, settings := newReconcileCoordinator(nil, nil)

	if err := coordinator.ReconcileInjectedCredentials(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertReconcileCalls(t, logs, settings)
}

func newReconcileCoordinator(hardenErrors, reconcileErrors map[string]error) (*Coordinator, *reconcileLogs, *reconcileClientSettings) {
	logs := &reconcileLogs{errors: hardenErrors}
	settings := &reconcileClientSettings{errors: reconcileErrors}
	stored := []instances.Instance{
		{ID: "a", Name: "A", Directory: filepath.Join("instances", "A")},
		{ID: "b", Name: "B", Directory: filepath.Join("instances", "B")},
		{ID: "c", Name: "C", Directory: filepath.Join("instances", "C")},
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
}
