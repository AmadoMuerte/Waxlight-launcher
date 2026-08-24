// Package recovery owns Last Known Good state, startup reconciliation,
// failed-launch analysis, recovery suggestions, and restore coordination. It
// reads snapshot capabilities through narrow ports and never touches storage
// or filesystem adapters directly.
package recovery

import (
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/snapshots"
)

// LastKnownGood records the most recent instance configuration that Waxlight
// considers successfully launched. It deliberately reuses the snapshot mod
// representation so change detection compares the same exact-release identity
// the snapshot system uses, never filenames alone.
//
// SnapshotID references an existing snapshot that captures this configuration
// when one exists; it is empty when no restorable snapshot matches. The
// reference is best-effort: when the snapshot is deleted later, only the
// reference is cleared and the metadata stays usable for change comparison.
type LastKnownGood struct {
	InstanceID  string
	RecordedAt  time.Time
	GameVersion string
	SnapshotID  string
	Mods        []snapshots.Mod
}

// ModChange describes a single mod whose presence or version differs between
// the Last Known Good state and the current instance state. From/To hold the
// version strings; for added mods only To is set, for removed mods only From.
type ModChange struct {
	Name string `json:"name"`
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

// ConfigurationChanges is the comparison between the Last Known Good state and
// the current instance state. It reports facts only: which game version and
// which mods changed, without any causality claim. The three mod lists are
// always serialized (never omitted), so consumers can rely on them being
// arrays even when nothing changed.
type ConfigurationChanges struct {
	GameVersionFrom string      `json:"gameVersionFrom,omitempty"`
	GameVersionTo   string      `json:"gameVersionTo,omitempty"`
	Updated         []ModChange `json:"updated"`
	Added           []ModChange `json:"added"`
	Removed         []ModChange `json:"removed"`
}

// Count returns the number of reported changes.
func (c ConfigurationChanges) Count() int {
	changes := len(c.Updated) + len(c.Added) + len(c.Removed)
	if c.GameVersionFrom != "" && c.GameVersionFrom != c.GameVersionTo {
		changes++
	}
	return changes
}

// Empty reports whether nothing changed since the Last Known Good state.
func (c ConfigurationChanges) Empty() bool {
	return c.Count() == 0
}

// LastKnownGoodStatus is the read model served to the instance page: the Last
// Known Good marker itself, whether the current configuration still matches
// it, and the live availability of the recovery snapshot.
type LastKnownGoodStatus struct {
	RecordedAt     time.Time
	GameVersion    string
	ModCount       int
	SnapshotID     string
	SnapshotExists bool
	MatchesCurrent bool
	Changes        ConfigurationChanges
}

// RecoverySuggestion is emitted after a failed startup when the current
// configuration differs from the Last Known Good state. It never claims a
// specific mod caused the failure; it only reports what changed. SnapshotID is
// set when a restorable snapshot captures the Last Known Good state, and the
// frontend must only offer a one-click restore when it is present.
type RecoverySuggestion struct {
	InstanceID string    `json:"instanceId"`
	RecordedAt time.Time `json:"recordedAt"`
	SnapshotID string    `json:"snapshotId,omitempty"`
	// SnapshotExists is true when a restorable snapshot captures the Last
	// Known Good state; the frontend must only offer a one-click restore when
	// SnapshotID is present alongside it.
	SnapshotExists bool                 `json:"snapshotExists"`
	Changes        ConfigurationChanges `json:"changes"`
	// StateSignature identifies the current (failed) configuration. The
	// frontend can suppress repeat prompts for the same state with it.
	StateSignature string `json:"stateSignature"`
}
