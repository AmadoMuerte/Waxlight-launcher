package launching

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/mutations"
	settingscore "github.com/waxlight/waxlight-launcher/internal/settings"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

type optimumSettings struct{ path string }

func (settings optimumSettings) Get(context.Context) (settingscore.Settings, error) {
	return settingscore.Settings{OptimumPath: settings.path}, nil
}

type optimumResolver struct {
	target OptimumTarget
	err    error
	path   string
	dir    string
}

func (resolver *optimumResolver) Resolve(path, directory string) (OptimumTarget, error) {
	resolver.path, resolver.dir = path, directory
	return resolver.target, resolver.err
}

func TestResolveLaunchTargetKeepsVanillaExecutable(t *testing.T) {
	coordinator := &Coordinator{registry: NewRegistry(mutations.NewSlot())}
	target, err := coordinator.resolveLaunchTarget(context.Background(), instances.Instance{}, versions.GameVersion{
		ExecutablePath: "/game/Vintagestory", InstallationDir: "/game",
	})
	if err != nil {
		t.Fatal(err)
	}
	if target.Executable != "/game/Vintagestory" || target.WorkingDirectory != "/game" {
		t.Fatalf("vanilla target = %#v", target)
	}
}

func TestResolveLaunchTargetUsesOptimum(t *testing.T) {
	resolver := &optimumResolver{target: OptimumTarget{Executable: "/optimum/run.sh", WorkingDirectory: "/optimum"}}
	coordinator := &Coordinator{
		registry: NewRegistry(mutations.NewSlot()), settings: optimumSettings{path: "/configured"}, optimum: resolver,
	}
	target, err := coordinator.resolveLaunchTarget(context.Background(), instances.Instance{GameClient: instances.GameClientOptimum}, versions.GameVersion{
		InstallationDir: "/managed/1.22.5", ExecutablePath: "/game/vintagestory/Vintagestory",
	})
	if err != nil {
		t.Fatal(err)
	}
	if target.Executable != "/optimum/run.sh" || resolver.path != "/configured" || resolver.dir != "/game/vintagestory" {
		t.Fatalf("optimum target = %#v, resolver = %#v", target, resolver)
	}
}

func TestResolveLaunchTargetDoesNotFallBackWhenOptimumIsMissing(t *testing.T) {
	resolver := &optimumResolver{err: errors.New("missing Optimum")}
	coordinator := &Coordinator{registry: NewRegistry(mutations.NewSlot()), settings: optimumSettings{}, optimum: resolver}
	_, err := coordinator.resolveLaunchTarget(context.Background(), instances.Instance{GameClient: instances.GameClientOptimum}, versions.GameVersion{ExecutablePath: "/game/Vintagestory"})
	if err == nil {
		t.Fatal("missing Optimum unexpectedly fell back to Vanilla")
	}
}

func TestResolveLaunchTargetRejectsSecondExclusiveOptimum(t *testing.T) {
	registry := NewRegistry(mutations.NewSlot())
	registry.Start("running", runningGame{client: instances.GameClientOptimum})
	resolver := &optimumResolver{target: OptimumTarget{
		Executable: "/optimum/Optimum.exe", WorkingDirectory: "/optimum", Exclusive: true,
	}}
	coordinator := &Coordinator{registry: registry, settings: optimumSettings{}, optimum: resolver}
	_, err := coordinator.resolveLaunchTarget(context.Background(), instances.Instance{GameClient: instances.GameClientOptimum}, versions.GameVersion{})
	if err == nil {
		t.Fatal("second exclusive Optimum launch was accepted")
	}
}

func TestBuildLaunchArgumentsForOptimumKeepsManagedArguments(t *testing.T) {
	got := buildLaunchArguments([]string{"--global"}, []string{"--instance"}, "/instances/world", "example.test:42420")
	want := []string{"--global", "--instance", "--dataPath", "/instances/world", "--connect", "example.test:42420"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("arguments = %#v, want %#v", got, want)
	}
}
