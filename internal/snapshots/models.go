// Package snapshots owns instance backup and restore: snapshot models,
// reasons, retention policy, filesystem policy, and the manual and automatic
// snapshot capabilities. The concrete storage adapter lives under
// internal/platform/snapshots; this package only depends on narrow ports.
package snapshots

import "time"

// Type describes why a snapshot was created. Manual snapshots are created on
// user request; automatic snapshots are created by the launcher right before a
// destructive instance change. The type is part of the manifest so snapshots
// of both kinds stay restorable.
type Type string

const (
	// TypeManual marks a snapshot created by the user.
	TypeManual Type = "manual"
	// TypeAutomatic marks a safety snapshot created by the launcher before a
	// destructive operation. Its Reason records the operation.
	TypeAutomatic Type = "automatic"
)

// Reason describes the destructive operation an automatic snapshot was
// created for. It is stored in the manifest and shown in the Backups UI.
type Reason string

const (
	// ReasonBeforeModUpdate protects the state before updating one or more
	// installed mods.
	ReasonBeforeModUpdate Reason = "before_mod_update"
	// ReasonBeforeModRemoval protects the state before removing one or more
	// installed mods.
	ReasonBeforeModRemoval Reason = "before_mod_removal"
	// ReasonBeforeGameVersionChange protects the state before switching an
	// instance to another game version.
	ReasonBeforeGameVersionChange Reason = "before_game_version_change"
)

// FormatVersion is the version of the snapshot manifest format.
//
// Version 1 snapshots physically contain the instance's Mods directory.
// Version 2 snapshots skip Waxlight-managed mod binaries and instead record
// the exact ModDB release of every managed mod in the manifest. Both versions
// remain restorable.
const FormatVersion = 2

// FormatVersion1 is the legacy snapshot manifest format, kept for backward
// compatibility.
const FormatVersion1 = 1

// ModSource identifies where a snapshotted mod came from.
type ModSource string

const (
	// ModSourceModDB marks a mod whose exact release Waxlight can download
	// again from ModDB during restore.
	ModSourceModDB ModSource = "moddb"
	// ModSourceUnknown marks a manually installed mod for which Waxlight has
	// no downloadable source. Its binary is not copied into the snapshot and
	// it cannot be restored automatically.
	ModSourceUnknown ModSource = "unknown"
)

// Mod describes a single mod that was installed in the instance at snapshot
// time. Managed ModDB mods are identified by their exact release so restore
// downloads the same release, never a newer one. The version string is kept
// for validation, UI and diagnostics.
type Mod struct {
	Source     ModSource `json:"source"`
	ModID      string    `json:"modId,omitempty"`
	ReleaseID  string    `json:"releaseId,omitempty"`
	Identifier string    `json:"identifier,omitempty"`
	Version    string    `json:"version,omitempty"`
	FileName   string    `json:"fileName,omitempty"`
	SHA256     string    `json:"sha256,omitempty"`
	Enabled    bool      `json:"enabled"`
}

// Manifest is the persisted metadata of an instance snapshot. It is written
// before a snapshot becomes visible and must never contain secrets or
// temporary session credentials.
type Manifest struct {
	FormatVersion int               `json:"formatVersion"`
	ID            string            `json:"id"`
	InstanceID    string            `json:"instanceId"`
	InstanceName  string            `json:"instanceName"`
	CreatedAt     time.Time         `json:"createdAt"`
	Type          Type              `json:"type"`
	Reason        Reason            `json:"reason,omitempty"`
	Context       map[string]string `json:"context,omitempty"`
	GameVersion   string            `json:"gameVersion"`
	SizeBytes     int64             `json:"sizeBytes"`
	ModCount      int               `json:"modCount,omitempty"`
	WorldCount    int               `json:"worldCount,omitempty"`
	Mods          []Mod             `json:"mods,omitempty"`
}

// InstanceSnapshot is a snapshot of an instance's user data as presented to
// callers. It carries everything the UI needs to list and manage snapshots.
type InstanceSnapshot struct {
	ID           string
	InstanceID   string
	InstanceName string
	Type         Type
	Reason       Reason
	Context      map[string]string
	GameVersion  string
	CreatedAt    time.Time
	SizeBytes    int64
	ModCount     int
	WorldCount   int
}
