package wails

import (
	"context"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
)

type instanceListQueries struct{ instances []instances.Instance }

func (queries instanceListQueries) List(context.Context) ([]instances.Instance, error) {
	return queries.instances, nil
}

func (instanceListQueries) Get(context.Context, string) (instances.Instance, error) {
	return instances.Instance{}, nil
}

type storedModCounter struct{ mods []mods.InstalledMod }

func (counter storedModCounter) ListStoredMods(context.Context, string) ([]mods.InstalledMod, error) {
	return counter.mods, nil
}

type instanceListSessions struct{}

func (instanceListSessions) InstancePlaytime(context.Context, string) (int64, error) { return 0, nil }

type instanceListLifecycle struct{}

func (instanceListLifecycle) Context() context.Context      { return context.Background() }
func (instanceListLifecycle) Go(func(context.Context)) bool { return false }

func TestListInstancesUsesStoredModRecordsForCounts(t *testing.T) {
	controller := &InstanceController{
		queries: instanceListQueries{instances: []instances.Instance{{ID: "instance"}}},
		modCounter: storedModCounter{mods: []mods.InstalledMod{
			{Enabled: true}, {Enabled: false},
		}},
		sessions:  instanceListSessions{},
		lifecycle: instanceListLifecycle{},
	}

	result, err := controller.ListInstances()
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].TotalModCount != 2 || result[0].EnabledModCount != 1 {
		t.Fatalf("unexpected mod counts: %#v", result)
	}
}
