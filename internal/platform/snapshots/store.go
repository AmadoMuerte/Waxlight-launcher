// Package snapshots persists instance snapshots as plain directories under
// <dataRoot>/backups/instances/<instanceID>/<snapshotID>/, each containing a
// manifest.json and a data/ directory with the captured instance files.
//
// Snapshot IDs are validated as single path segments so they can never escape
// the snapshot tree. Manifest files are the source of truth; snapshots without
// a readable manifest are considered incomplete and are skipped by List.
package snapshots

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/platform/atomicfile"
	"github.com/waxlight/waxlight-launcher/internal/snapshots"
)

const (
	manifestFileName = "manifest.json"
	dataDirectory    = "data"
	tempPrefix       = ".tmp-"
)

// Store is the filesystem location of snapshots inside the launcher data root.
type Store struct {
	root string
}

// New creates a Store rooted at the launcher data root. Snapshots live in the
// backups/instances subtree and move together with the data root.
func New(dataRoot string) *Store {
	return &Store{root: filepath.Join(dataRoot, "backups", "instances")}
}

// InstanceDir returns the directory holding every snapshot of an instance,
// creating it when missing.
func (store *Store) InstanceDir(instanceID string) (string, error) {
	dir, err := store.InstancePath(instanceID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// InstancePath returns the directory holding every snapshot of an instance
// without creating it.
func (store *Store) InstancePath(instanceID string) (string, error) {
	instanceID, err := safeSegment(instanceID)
	if err != nil {
		return "", err
	}
	return filepath.Join(store.root, instanceID), nil
}

// SnapshotDir returns the directory of a single snapshot without creating it.
// Both identifiers are validated as single path segments so the result always
// stays inside the backup tree.
func (store *Store) SnapshotDir(instanceID string, snapshotID string) (string, error) {
	instanceID, err := safeSegment(instanceID)
	if err != nil {
		return "", err
	}
	snapshotID, err = safeSegment(snapshotID)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(store.root, instanceID, snapshotID)
	if filepath.Dir(filepath.Dir(dir)) != store.root || filepath.Dir(dir) != filepath.Join(store.root, instanceID) {
		return "", errs.NewError(errs.ErrValidation, "Unsafe snapshot path")
	}
	return dir, nil
}

// DataDir returns the directory inside a snapshot that holds the captured
// instance files.
func (store *Store) DataDir(snapshotDir string) string {
	return filepath.Join(snapshotDir, dataDirectory)
}

// TempDir creates a fresh staging directory as a sibling of the final snapshot
// directory so it can be atomically renamed into place. The staging directory
// is dot-prefixed and therefore never listed as a snapshot.
func (store *Store) TempDir(instanceID string) (string, error) {
	instanceDir, err := store.InstanceDir(instanceID)
	if err != nil {
		return "", err
	}
	temporary, err := os.MkdirTemp(instanceDir, tempPrefix)
	if err != nil {
		return "", err
	}
	return temporary, nil
}

// List returns every readable snapshot of an instance, newest first.
// Directories that cannot be parsed (interrupted creations, corrupted
// manifests) are logged and skipped so one damaged snapshot never breaks the
// whole list.
func (store *Store) List(ctx context.Context, instanceID string) ([]snapshots.InstanceSnapshot, error) {
	instanceDir, err := store.InstancePath(instanceID)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(instanceDir)
	if errors.Is(err, os.ErrNotExist) {
		return []snapshots.InstanceSnapshot{}, nil
	}
	if err != nil {
		return nil, err
	}

	result := []snapshots.InstanceSnapshot{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		manifest, readErr := store.ReadManifest(filepath.Join(instanceDir, name))
		if readErr != nil {
			slog.Warn("skipping an unreadable instance snapshot", "instanceId", instanceID, "snapshotId", name, "error", readErr)
			continue
		}
		if manifest.ID != name {
			slog.Warn("skipping an instance snapshot whose manifest does not match its directory", "instanceId", instanceID, "snapshotId", name)
			continue
		}
		result = append(result, snapshots.InstanceSnapshot{
			ID:           manifest.ID,
			InstanceID:   manifest.InstanceID,
			InstanceName: manifest.InstanceName,
			Type:         manifest.Type,
			Reason:       manifest.Reason,
			Context:      manifest.Context,
			GameVersion:  manifest.GameVersion,
			CreatedAt:    manifest.CreatedAt,
			SizeBytes:    manifest.SizeBytes,
			ModCount:     manifest.ModCount,
			WorldCount:   manifest.WorldCount,
		})
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].CreatedAt.After(result[right].CreatedAt)
	})
	return result, nil
}

// ReadManifest loads and validates the manifest of a snapshot directory.
func (store *Store) ReadManifest(snapshotDir string) (snapshots.Manifest, error) {
	data, err := os.ReadFile(filepath.Join(snapshotDir, manifestFileName))
	if err != nil {
		return snapshots.Manifest{}, err
	}
	var manifest snapshots.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return snapshots.Manifest{}, err
	}
	if manifest.FormatVersion != snapshots.FormatVersion1 &&
		manifest.FormatVersion != snapshots.FormatVersion {
		return snapshots.Manifest{}, errors.New("unsupported snapshot format version")
	}
	if strings.TrimSpace(manifest.ID) == "" || strings.TrimSpace(manifest.InstanceID) == "" {
		return snapshots.Manifest{}, errors.New("snapshot manifest misses its identifiers")
	}
	if manifest.Type != snapshots.TypeManual && manifest.Type != snapshots.TypeAutomatic {
		return snapshots.Manifest{}, errors.New("unsupported snapshot type")
	}
	if manifest.Type == snapshots.TypeAutomatic && manifest.Reason == "" {
		return snapshots.Manifest{}, errors.New("automatic snapshot manifest misses its reason")
	}
	if manifest.CreatedAt.IsZero() {
		return snapshots.Manifest{}, errors.New("snapshot manifest misses its creation time")
	}
	return manifest, nil
}

// WriteManifest persists a manifest into a snapshot staging directory.
func (store *Store) WriteManifest(snapshotDir string, manifest snapshots.Manifest) error {
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(filepath.Join(snapshotDir, manifestFileName), append(encoded, '\n'), 0o600)
}

// Remove deletes a snapshot directory. It never touches anything outside the
// snapshot tree.
func (store *Store) Remove(instanceID string, snapshotID string) error {
	dir, err := store.SnapshotDir(instanceID, snapshotID)
	if err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return errs.NewError(snapshots.ErrSnapshotNotFound, "Snapshot not found")
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errs.NewError(snapshots.ErrSnapshotInvalid, "Snapshot path is not a directory")
	}
	return os.RemoveAll(dir)
}

// Size returns the number of bytes stored in the data directory of a snapshot.
func (store *Store) Size(snapshotDir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(store.DataDir(snapshotDir), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	return total, err
}

// safeSegment rejects identifiers that could escape the snapshot tree.
func safeSegment(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == ".." || filepath.Base(value) != value || strings.ContainsAny(value, `/\\`) {
		return "", errs.NewError(errs.ErrValidation, "Invalid snapshot identifier")
	}
	return value, nil
}
