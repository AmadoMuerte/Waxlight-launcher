package application

import (
	"context"
	"io"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/mods"
	"github.com/waxlight/waxlight-launcher/internal/recovery"
	"github.com/waxlight/waxlight-launcher/internal/servers"
)

type ClientSettingsPatcher interface {
	Inject(path string, account accounts.Account) (func() error, error)
	Clear(path string) error
	Reconcile(path string) error
}

type Store interface {
	instances.Repository
	servers.Repository
	Close() error

	GetLastKnownGood(context.Context, string) (recovery.LastKnownGood, error)
	SaveLastKnownGood(context.Context, recovery.LastKnownGood) error
	DeleteLastKnownGood(context.Context, string) error

	ListMods(context.Context, string) ([]mods.InstalledMod, error)
	GetMod(context.Context, string) (mods.InstalledMod, error)
	SaveMod(context.Context, mods.InstalledMod) error
	DeleteMod(context.Context, string) error
}

// modsStoreAdapter maps the shared store to the mods repository port,
// converting instance records into the minimal instance view of the mods
// feature.
type modsStoreAdapter struct {
	store Store
}

func (adapter modsStoreAdapter) GetInstance(ctx context.Context, id string) (mods.InstanceRef, error) {
	instance, err := adapter.store.GetInstance(ctx, id)
	return instanceRef(instance), err
}

func (adapter modsStoreAdapter) ListInstances(ctx context.Context) ([]mods.InstanceRef, error) {
	instances, err := adapter.store.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]mods.InstanceRef, 0, len(instances))
	for _, instance := range instances {
		result = append(result, instanceRef(instance))
	}
	return result, nil
}

func instanceRef(instance instances.Instance) mods.InstanceRef {
	return mods.InstanceRef{
		ID:            instance.ID,
		Name:          instance.Name,
		Directory:     instance.Directory,
		GameVersionID: instance.GameVersionID,
	}
}

func (adapter modsStoreAdapter) ListMods(ctx context.Context, instanceID string) ([]mods.InstalledMod, error) {
	return adapter.store.ListMods(ctx, instanceID)
}

func (adapter modsStoreAdapter) GetMod(ctx context.Context, id string) (mods.InstalledMod, error) {
	return adapter.store.GetMod(ctx, id)
}

func (adapter modsStoreAdapter) SaveMod(ctx context.Context, mod mods.InstalledMod) error {
	return adapter.store.SaveMod(ctx, mod)
}

func (adapter modsStoreAdapter) DeleteMod(ctx context.Context, id string) error {
	return adapter.store.DeleteMod(ctx, id)
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

type DiskSpaceChecker interface {
	Available(path string) (int64, error)
}
