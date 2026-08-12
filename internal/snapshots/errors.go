package snapshots

// Snapshot error codes. They are feature-owned but remain stable strings so
// the frontend contract never changes.
const (
	// ErrSnapshotNotFound is returned when a snapshot does not exist.
	ErrSnapshotNotFound = "SNAPSHOT_NOT_FOUND"
	// ErrSnapshotInvalid is returned when a snapshot cannot be read or
	// restored because its content is corrupted or incomplete.
	ErrSnapshotInvalid = "SNAPSHOT_INVALID"
	// ErrSnapshotInProgress is returned when an operation needs exclusive
	// access to an instance while a snapshot operation is running.
	ErrSnapshotInProgress = "SNAPSHOT_IN_PROGRESS"
)
