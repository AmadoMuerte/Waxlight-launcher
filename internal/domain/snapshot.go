package domain

import "time"

// SnapshotType describes why a snapshot was created. Manual snapshots are
// created on user request; automatic snapshots are created by the launcher
// right before a destructive instance change. The type is part of the
// manifest so snapshots of both kinds stay restorable.
type SnapshotType string

const (
	// SnapshotTypeManual marks a snapshot created by the user.
	SnapshotTypeManual SnapshotType = "manual"
	// SnapshotTypeAutomatic marks a safety snapshot created by the launcher
	// before a destructive operation. Its Reason records the operation.
	SnapshotTypeAutomatic SnapshotType = "automatic"
)

// SnapshotReason describes the destructive operation an automatic snapshot
// was created for. It is stored in the manifest and shown in the Backups UI.
type SnapshotReason string

const (
	// SnapshotReasonBeforeModUpdate protects the state before updating one or
	// more installed mods.
	SnapshotReasonBeforeModUpdate SnapshotReason = "before_mod_update"
	// SnapshotReasonBeforeModRemoval protects the state before removing one or
	// more installed mods.
	SnapshotReasonBeforeModRemoval SnapshotReason = "before_mod_removal"
	// SnapshotReasonBeforeGameVersionChange protects the state before
	// switching an instance to another game version.
	SnapshotReasonBeforeGameVersionChange SnapshotReason = "before_game_version_change"
)

// SnapshotFormatVersion is the version of the snapshot manifest format.
//
// Version 1 snapshots physically contain the instance's Mods directory.
// Version 2 snapshots skip Waxlight-managed mod binaries and instead record
// the exact ModDB release of every managed mod in the manifest. Both versions
// remain restorable.
const SnapshotFormatVersion = 2

// Snapshot format version 1, kept for backward compatibility.
const SnapshotFormatVersion1 = 1

// SnapshotModSource identifies where a snapshotted mod came from.
type SnapshotModSource string

const (
	// SnapshotModSourceModDB marks a mod whose exact release Waxlight can
	// download again from ModDB during restore.
	SnapshotModSourceModDB SnapshotModSource = "moddb"
	// SnapshotModSourceUnknown marks a manually installed mod for which
	// Waxlight has no downloadable source. Its binary is not copied into the
	// snapshot and it cannot be restored automatically.
	SnapshotModSourceUnknown SnapshotModSource = "unknown"
)

// SnapshotMod describes a single mod that was installed in the instance at
// snapshot time. Managed ModDB mods are identified by their exact release so
// restore downloads the same release, never a newer one. The version string is
// kept for validation, UI and diagnostics.
type SnapshotMod struct {
	Source     SnapshotModSource `json:"source"`
	ModID      string            `json:"modId,omitempty"`
	ReleaseID  string            `json:"releaseId,omitempty"`
	Identifier string            `json:"identifier,omitempty"`
	Version    string            `json:"version,omitempty"`
	FileName   string            `json:"fileName,omitempty"`
	SHA256     string            `json:"sha256,omitempty"`
	Enabled    bool              `json:"enabled"`
}

// SnapshotManifest is the persisted metadata of an instance snapshot. It is
// written before a snapshot becomes visible and must never contain secrets or
// temporary session credentials.
type SnapshotManifest struct {
	FormatVersion int               `json:"formatVersion"`
	ID            string            `json:"id"`
	InstanceID    string            `json:"instanceId"`
	InstanceName  string            `json:"instanceName"`
	CreatedAt     time.Time         `json:"createdAt"`
	Type          SnapshotType      `json:"type"`
	Reason        SnapshotReason    `json:"reason,omitempty"`
	Context       map[string]string `json:"context,omitempty"`
	GameVersion   string            `json:"gameVersion"`
	SizeBytes     int64             `json:"sizeBytes"`
	ModCount      int               `json:"modCount,omitempty"`
	WorldCount    int               `json:"worldCount,omitempty"`
	Mods          []SnapshotMod     `json:"mods,omitempty"`
}

// InstanceSnapshot is a snapshot of an instance's user data as presented to
// callers. It carries everything the UI needs to list and manage snapshots.
type InstanceSnapshot struct {
	ID           string            `json:"id"`
	InstanceID   string            `json:"instanceId"`
	InstanceName string            `json:"instanceName"`
	Type         SnapshotType      `json:"type"`
	Reason       SnapshotReason    `json:"reason,omitempty"`
	Context      map[string]string `json:"context,omitempty"`
	GameVersion  string            `json:"gameVersion"`
	CreatedAt    time.Time         `json:"createdAt"`
	SizeBytes    int64             `json:"sizeBytes"`
	ModCount     int               `json:"modCount"`
	WorldCount   int               `json:"worldCount"`
}
