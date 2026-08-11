package app_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/app"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/dataroot"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/logging"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/operations"
	"github.com/waxlight/waxlight-launcher/internal/platform/sqlite"
	"github.com/waxlight/waxlight-launcher/internal/sessions"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

// wireHome prepares an isolated launcher home directory for a composition
// test.
func wireHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Cleanup(func() {
		logging.Clear()
		logging.SetFileHeader("")
	})
	return home
}

func seedInterruptedState(t *testing.T, home string) {
	t.Helper()
	store, err := sqlite.Open(dataroot.DatabasePath(home))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	started := now.Add(-30 * time.Minute)
	if err := store.SaveVersion(context.Background(), versions.GameVersion{
		ID:              "1.20",
		Name:            "1.20",
		Channel:         "stable",
		Platform:        "linux",
		Architecture:    "amd64",
		InstallationDir: filepath.Join(home, "versions", "1.20"),
		ExecutablePath:  filepath.Join(home, "versions", "1.20", "Vintagestory"),
		Status:          "installed",
		InstalledAt:     started,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveInstance(context.Background(), instances.Instance{
		ID:            "instance-1",
		Name:          "World",
		GameVersionID: "1.20",
		Directory:     filepath.Join(home, "instances", "world"),
		Status:        "ready",
		CreatedAt:     started,
		UpdatedAt:     started,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSession(context.Background(), sessions.PlaySession{
		ID:         "interrupted-session",
		InstanceID: "instance-1",
		VersionID:  "1.20",
		StartedAt:  started,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveOperation(context.Background(), operations.Operation{
		ID:        "interrupted-operation",
		Type:      "version_install",
		Title:     "Install 1.20",
		Status:    "running",
		CreatedAt: started,
		StartedAt: &started,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestWireConstructsCompleteContainer proves the composition root constructs
// every bound controller and that the bound set matches the checked-in Wails
// API inventory.
func TestWireConstructsCompleteContainer(t *testing.T) {
	home := wireHome(t)
	container, err := app.NewWithHome(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { container.Shutdown(context.Background()) })

	contents, err := os.ReadFile(filepath.Join("..", "..", "docs", "wails-api-inventory.json"))
	if err != nil {
		t.Fatal(err)
	}
	var inventory struct {
		Controllers map[string][]string `json:"controllers"`
	}
	if err := json.Unmarshal(contents, &inventory); err != nil {
		t.Fatal(err)
	}
	bound := map[string]bool{}
	for _, controller := range container.Controllers {
		name := controllerTypeName(controller)
		if _, known := inventory.Controllers[name]; !known {
			t.Fatalf("bound controller %s is missing from the Wails API inventory", name)
		}
		bound[name] = true
	}
	for name := range inventory.Controllers {
		if !bound[name] {
			t.Fatalf("inventory controller %s is not bound by the composition root", name)
		}
	}
}

func controllerTypeName(value any) string {
	// The controller types are exported pointers named <name>Controller; the
	// composition root proves the binding, and the inventory test verifies the
	// set.
	typ := reflect.TypeOf(value)
	if typ.Kind() == reflect.Ptr {
		return typ.Elem().Name()
	}
	return typ.Name()
}

// TestWireRecoveryOrderingProvesInterruptedStateIsRecovered proves the
// construction order reconciles interrupted sessions and operations before
// the container is returned.
func TestWireRecoveryOrderingProvesInterruptedStateIsRecovered(t *testing.T) {
	home := wireHome(t)
	seedInterruptedState(t, home)

	container, err := app.NewWithHome(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { container.Shutdown(context.Background()) })

	store, err := sqlite.Open(dataroot.DatabasePath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	sessions_, err := store.ListSessions(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions_) != 1 {
		t.Fatalf("expected one recovered session, got %d", len(sessions_))
	}
	recovered := sessions_[0]
	if !recovered.Recovered || !recovered.Crashed {
		t.Fatalf("interrupted session was not marked recovered: %+v", recovered)
	}
	if recovered.EndedAt == nil {
		t.Fatal("interrupted session has no finish timestamp")
	}
	if recovered.DurationSec <= 0 {
		t.Fatalf("recovered session playtime was not computed: %+v", recovered)
	}

	operations_, err := store.ListOperations(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(operations_) != 1 {
		t.Fatalf("expected one reconciled operation, got %d", len(operations_))
	}
	reconciled := operations_[0]
	if reconciled.Status != "failed" {
		t.Fatalf("interrupted operation was not reconciled to failed: %+v", reconciled)
	}
	if reconciled.ErrorCode == nil || reconciled.ErrorMessage == nil {
		t.Fatalf("reconciled operation is missing the interrupted-failure message: %+v", reconciled)
	}
	if reconciled.FinishedAt == nil {
		t.Fatal("reconciled operation has no finish timestamp")
	}
}

// TestWireStartupOrderingDerivesLifecycleFromFramework proves Startup wires
// the lifecycle context to the framework context before any worker is
// registered.
func TestWireStartupOrderingDerivesLifecycleFromFramework(t *testing.T) {
	home := wireHome(t)
	container, err := app.NewWithHome(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { container.Shutdown(context.Background()) })

	type contextKey string
	parent := context.WithValue(context.Background(), contextKey("source"), "framework")
	container.Startup(parent)

	if got := container.Lifecycle.Context().Value(contextKey("source")); got != "framework" {
		t.Fatalf("lifecycle context does not derive from the framework context: %v", got)
	}

	ran := make(chan struct{})
	if !container.Lifecycle.Go(func(ctx context.Context) {
		close(ran)
	}) {
		t.Fatal("lifecycle refused a worker before shutdown")
	}
	select {
	case <-ran:
	case <-time.After(5 * time.Second):
		t.Fatal("lifecycle-owned worker did not run")
	}
}

// TestWireShutdownJoinsWorkersAndClosesStore proves Shutdown is deterministic:
// it cancels the lifecycle context, joins every registered worker, and closes
// the shared store.
func TestWireShutdownJoinsWorkersAndClosesStore(t *testing.T) {
	home := wireHome(t)
	container, err := app.NewWithHome(home)
	if err != nil {
		t.Fatal(err)
	}
	container.Startup(context.Background())

	blocked := make(chan struct{})
	released := make(chan struct{})
	if !container.Lifecycle.Go(func(ctx context.Context) {
		close(blocked)
		<-ctx.Done()
		close(released)
	}) {
		t.Fatal("lifecycle refused a worker")
	}
	<-blocked

	shutdownDone := make(chan struct{})
	go func() {
		container.Shutdown(context.Background())
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown did not join the blocked worker")
	}
	select {
	case <-released:
	default:
		t.Fatal("worker was not released by context cancellation")
	}

	// The store is closed and the data root stays consistent for the next run.
	store, err := sqlite.Open(dataroot.DatabasePath(home))
	if err != nil {
		t.Fatalf("store is not consistent after shutdown: %v", err)
	}
	_ = store.Close()
}
