package recovery

import (
	"context"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/snapshots"
)

// Repository persists the Last Known Good markers of instances.
type Repository interface {
	GetLastKnownGood(context.Context, string) (LastKnownGood, error)
	SaveLastKnownGood(context.Context, LastKnownGood) error
	DeleteLastKnownGood(context.Context, string) error
}

// SnapshotReader exposes the snapshot capabilities the recovery analysis
// needs. It is implemented by the snapshots feature.
type SnapshotReader interface {
	List(context.Context, string) ([]snapshots.InstanceSnapshot, error)
	ReadManifest(context.Context, string, string) (snapshots.Manifest, error)
	IsRestorable(context.Context, string, string) bool
}

// ModConfiguration resolves the current installed-mod configuration of an
// instance into the snapshot manifest representation. It is implemented by
// the snapshots feature.
type ModConfiguration interface {
	ListInstalledMods(context.Context, string) ([]snapshots.InstalledMod, error)
	ModManifest(context.Context, []snapshots.InstalledMod) []snapshots.Mod
	GameVersionName(context.Context, string) string
}

// InstanceReader resolves instance records for status reads.
type InstanceReader interface {
	GetInstance(context.Context, string) (instances.Instance, error)
}

// MutationGate coordinates launcher-wide writes with data-root relocation.
type MutationGate interface {
	Begin() error
	End()
}

// Publisher forwards launcher events to the frontend.
type Publisher interface {
	Publish(string, any)
}

// Clock returns the current time.
type Clock func() time.Time
