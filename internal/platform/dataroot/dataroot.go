// Package dataroot is a self-contained library for relocating the launcher's
// data directory. It keeps a fixed launcher home directory (the OS config
// directory) that only holds a pointer and relocation markers, while the heavy
// user data lives in a movable data root that defaults to the home directory.
//
// A relocation is split into two phases so it stays safe on every platform:
//
//  1. Copy: the running launcher copies the data root to the target (with byte
//     progress) while the application is still up.
//  2. Finalize: the launcher restarts, waits for the old process to release the
//     database, hands the database over, points the pointer at the target, and
//     only then rewrites the stored absolute paths and removes the old copy.
//
// Callers interact through the Manager type; the composition root consumes
// PrepareStartup and FinalizePrevious, and the settings controller drives
// StartRelocation.
package dataroot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/atomicfile"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/settings"
)

// Marker phases for a pending data root relocation.
const (
	PhaseCopy     = "copy"
	PhaseFinalize = "finalize"
)

// File names kept inside the fixed launcher home directory. They are never
// moved with the data root and are excluded from the copy.
const (
	pointerFile  = "data-root"
	pendingFile  = "data-root-pending"
	previousFile = "data-root-previous"
	errorFile    = "data-root-error"
	databaseName = "waxlight.db"
	databaseWAL  = "waxlight.db-wal"
	databaseSHM  = "waxlight.db-shm"

	waitForParentTimeout = 30 * time.Second
)

// Marker describes a relocation requested from a running launcher and consumed
// at the next startup. PhaseCopy means the file copy was interrupted and the
// partial target must be discarded; PhaseFinalize means the copy completed and
// the restarted process must finish the move (database handoff and pointer).
type Marker struct {
	From          string `json:"from"`
	To            string `json:"to"`
	CopyTarget    string `json:"copyTarget,omitempty"`
	Phase         string `json:"phase"`
	SourcePID     int    `json:"sourcePid"`
	TargetCreated bool   `json:"targetCreated"`
	Committed     bool   `json:"committed,omitempty"`
	OwnerFile     string `json:"ownerFile,omitempty"`
}

// Manager is the data root library entry point. It owns the fixed home
// directory, the relocation markers, and the copy/move orchestration.
type Manager struct {
	home     string
	relaunch func() error
}

// New creates a Manager rooted at the OS configuration directory.
func New() (*Manager, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &Manager{home: filepath.Join(configDir, "waxlight"), relaunch: RelaunchApplication}, nil
}

// NewWithHome creates a Manager with an explicit home directory (used by tests).
func NewWithHome(home string) *Manager {
	return NewWithHomeAndRelaunch(home, RelaunchApplication)
}

// NewWithHomeAndRelaunch creates a testable Manager without post-construction wiring.
func NewWithHomeAndRelaunch(home string, relaunch func() error) *Manager {
	return &Manager{home: home, relaunch: relaunch}
}

// Home returns the fixed launcher home directory.
func (m *Manager) Home() string {
	return m.home
}

// DatabasePath returns the SQLite database path inside a data root.
func DatabasePath(root string) string {
	return filepath.Join(root, databaseName)
}

// DatabaseFiles returns the database file names that are handled separately
// during a relocation handoff.
func DatabaseFiles() []string {
	return []string{databaseName, databaseWAL, databaseSHM}
}

// Current returns the active data root: the pointer content when a relocation
// happened, otherwise the fixed home directory.
func (m *Manager) Current() (string, error) {
	pointer, err := m.readMarkerFile(pointerFile)
	if err != nil {
		return "", err
	}
	if pointer != "" {
		return pointer, nil
	}
	return m.home, nil
}

// Pending returns the pending relocation marker, or nil when there is none.
func (m *Manager) Pending() (*Marker, error) {
	data, err := os.ReadFile(m.filePath(pendingFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var marker Marker
	if err := json.Unmarshal(data, &marker); err != nil {
		return nil, fmt.Errorf("parse pending data root marker: %w", err)
	}
	return &marker, nil
}

// ValidateTarget checks that a requested data root can be used for relocation.
// The target must differ from the current root, must not nest inside it (or
// contain it), and must be a missing or empty directory.
func (m *Manager) ValidateTarget(target string) error {
	current, err := m.Current()
	if err != nil {
		return err
	}
	return ValidateTarget(current, target)
}

// ValidateTarget checks a target against a current root without a Manager.
func ValidateTarget(current, target string) error {
	if strings.TrimSpace(target) == "" {
		return errors.New("the data folder must not be empty")
	}
	currentAbs, err := filepath.Abs(current)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if filepath.Clean(targetAbs) == filepath.Clean(currentAbs) {
		return errors.New("the data folder must differ from the current one")
	}
	if isSubpath(targetAbs, currentAbs) {
		return errors.New("the data folder must not be inside the current data root")
	}
	if isSubpath(currentAbs, targetAbs) {
		return errors.New("the data folder must not contain the current data root")
	}
	info, err := os.Stat(targetAbs)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("the data folder target is not a directory")
	}
	entries, err := os.ReadDir(targetAbs)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return errors.New("the data folder target must be empty")
	}
	return nil
}

// Relocation is prepared synchronously and run by the application lifecycle.
type Relocation struct {
	manager *Manager
	marker  Marker
}

// CheckTarget verifies that a target can be used as the launcher data folder,
// including write access, without recording a relocation or touching any other
// state. It backs the settings dialog's pre-move warning.
func (m *Manager) CheckTarget(target string) error {
	if err := m.ValidateTarget(target); err != nil {
		return err
	}
	created, err := ensureTargetWritable(target)
	if err != nil {
		return err
	}
	if created {
		return os.Remove(target)
	}
	return nil
}

// PrepareRelocation validates and records a relocation before background work starts.
func (m *Manager) PrepareRelocation(target string) (settings.Relocation, error) {
	current, err := m.Current()
	if err != nil {
		return nil, err
	}
	if err := ValidateTarget(current, target); err != nil {
		return nil, err
	}
	targetCreated, err := ensureTargetWritable(target)
	if err != nil {
		return nil, err
	}
	if targetCreated {
		if err := os.Remove(target); err != nil {
			return nil, err
		}
	}
	copyTarget, err := os.MkdirTemp(filepath.Dir(target), "."+filepath.Base(target)+".waxlight-copy-")
	if err != nil {
		return nil, err
	}
	marker := Marker{
		From: current, To: target, CopyTarget: copyTarget, Phase: PhaseCopy,
		TargetCreated: targetCreated, OwnerFile: ".waxlight-relocation-copy-" + filepath.Base(copyTarget),
	}
	if err := m.writePending(marker); err != nil {
		_ = cleanRelocationTarget(marker)
		return nil, err
	}
	return &Relocation{manager: m, marker: marker}, nil
}

// ensureTargetWritable creates the target directory when it is missing and
// verifies it accepts a file write, cleaning up the created directory on
// failure. It reports whether the target was created so the caller can remove
// it again on success. Creation or write failures map to
// settings.ErrDataFolderNotWritable.
func ensureTargetWritable(target string) (bool, error) {
	created := false
	if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(target, 0o700); err != nil {
			return false, fmt.Errorf("%w: %v", settings.ErrDataFolderNotWritable, err)
		}
		created = true
	} else if err != nil {
		return false, fmt.Errorf("%w: %v", settings.ErrDataFolderNotWritable, err)
	}
	probe := filepath.Join(target, ".waxlight-write-probe")
	if file, err := os.Create(probe); err != nil {
		if created {
			_ = os.RemoveAll(target)
		}
		return false, fmt.Errorf("%w: %v", settings.ErrDataFolderNotWritable, err)
	} else {
		_ = file.Close()
		_ = os.Remove(probe)
	}
	return created, nil
}

// Run copies the data and relaunches the application in the caller-owned worker.
func (relocation *Relocation) Run(ctx context.Context, progress func(copied, total int64)) error {
	marker := relocation.marker
	if err := CopyDataContext(ctx, marker.From, marker.copyTarget(), progress); err != nil {
		cleanupFailedRelocation(relocation.manager, marker, err)
		return err
	}
	if err := ctx.Err(); err != nil {
		cleanupFailedRelocation(relocation.manager, marker, err)
		return err
	}
	if err := validateUnchangedTarget(marker); err != nil {
		cleanupFailedRelocation(relocation.manager, marker, err)
		return err
	}
	ownerPath := filepath.Join(marker.copyTarget(), marker.OwnerFile)
	if _, err := os.Stat(ownerPath); err == nil {
		err = errors.New("the data folder contains a reserved relocation file")
		cleanupFailedRelocation(relocation.manager, marker, err)
		return err
	} else if !errors.Is(err, os.ErrNotExist) {
		cleanupFailedRelocation(relocation.manager, marker, err)
		return err
	}
	if err := os.WriteFile(ownerPath, []byte(marker.CopyTarget), 0o600); err != nil {
		cleanupFailedRelocation(relocation.manager, marker, err)
		return err
	}
	marker.Phase = PhaseFinalize
	marker.SourcePID = os.Getpid()
	if err := relocation.manager.writePending(marker); err != nil {
		cleanupFailedRelocation(relocation.manager, marker, fmt.Errorf("could not record the data folder move: %w", err))
		return err
	}
	if err := ctx.Err(); err != nil {
		cleanupFailedRelocation(relocation.manager, marker, err)
		return err
	}
	if err := relocation.manager.relaunch(); err != nil {
		cleanupFailedRelocation(relocation.manager, marker, fmt.Errorf("could not restart the launcher: %w", err))
		return err
	}
	return nil
}

// PrepareStartup resolves the data root to use for this startup, handling any
// pending relocation from a previous run. It is called before the database is
// opened.
//
//   - An interrupted copy discards the partial target and keeps the old root.
//   - A completed copy is finalized: the old process is waited for, the closed
//     database files are handed over, the pointer moves, and the old root is
//     recorded for path rewriting and cleanup.
//   - A failed finalize keeps the old root, records the error, and discards the
//     partial target.
func (m *Manager) PrepareStartup() (string, error) {
	if err := os.MkdirAll(m.home, 0o700); err != nil {
		return "", err
	}
	marker, err := m.Pending()
	if err != nil {
		return "", err
	}
	current, err := m.Current()
	if err != nil {
		return "", err
	}
	if marker == nil {
		return m.validatePointerTarget(current)
	}
	switch marker.Phase {
	case PhaseCopy:
		if err := cleanRelocationTarget(*marker); err != nil {
			return "", err
		}
		if err := m.clearPending(); err != nil {
			return "", err
		}
		return m.validatePointerTarget(current)
	case PhaseFinalize:
		if err := m.finishPending(marker); err != nil {
			if marker.Committed {
				_ = m.writeError(fmt.Sprintf("data folder relocation failed: %v", err))
				return "", err
			}
			if cleanupErr := cleanRelocationTarget(*marker); cleanupErr == nil {
				_ = m.clearPending()
			}
			_ = m.writeError(fmt.Sprintf("data folder relocation failed: %v", err))
			return m.validatePointerTarget(current)
		}
		return marker.To, nil
	default:
		_ = m.clearPending()
		return m.validatePointerTarget(current)
	}
}

// validatePointerTarget refuses to start on a relocated data root that does
// not exist. Without this check a missing drive or a mistyped pointer would
// silently boot the launcher with a fresh empty database, looking like data
// loss. The default home directory (no pointer) is created by the caller.
func (m *Manager) validatePointerTarget(current string) (string, error) {
	pointer, err := m.readMarkerFile(pointerFile)
	if err != nil {
		return "", err
	}
	if pointer == "" {
		return current, nil
	}
	if _, err := os.Stat(pointer); errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("the data folder %q does not exist; restore it or correct the %q file in %q", pointer, pointerFile, m.home)
	} else if err != nil {
		return "", err
	}
	return current, nil
}

// FinalizePrevious rewrites the stored absolute paths from a previous data root
// to the current one through relocatePaths, then removes the previous data
// copy. It is called after the database has been opened from the current root.
// It is a no-op when no relocation left a previous root behind.
func (m *Manager) FinalizePrevious(relocatePaths func(oldRoot, newRoot string) error) error {
	previous, err := m.readMarkerFile(previousFile)
	if err != nil {
		return err
	}
	if previous == "" {
		return nil
	}
	current, err := m.Current()
	if err != nil {
		return err
	}
	if filepath.Clean(previous) == filepath.Clean(current) {
		_ = m.clearMarker(previousFile)
		_ = m.clearError()
		return nil
	}
	if relocatePaths != nil {
		if err := relocatePaths(previous, current); err != nil {
			_ = m.writeError(fmt.Sprintf("could not rewrite stored data paths: %v", err))
			return err
		}
	}
	if err := m.removeOldRoot(previous); err != nil {
		_ = m.writeError(fmt.Sprintf("could not remove the previous data folder: %v", err))
		return err
	}
	_ = m.clearMarker(previousFile)
	_ = m.clearError()
	return nil
}

// ReadError returns the last recorded relocation error, or "".
func (m *Manager) ReadError() (string, error) {
	return m.readMarkerFile(errorFile)
}

// ClearError clears the recorded relocation error.
func (m *Manager) ClearError() error {
	return m.clearMarker(errorFile)
}

// finishPending completes a finalized relocation before the database is opened.
func (m *Manager) finishPending(marker *Marker) error {
	if marker.SourcePID > 0 {
		waitForProcessExit(marker.SourcePID, waitForParentTimeout)
	}
	if _, err := os.Stat(marker.From); err != nil {
		return err
	}
	copyTarget := marker.copyTarget()
	if marker.CopyTarget != "" {
		if _, err := os.Stat(copyTarget); errors.Is(err, os.ErrNotExist) {
			owned, ownerErr := relocationTargetOwned(marker.To, marker.OwnerFile, marker.CopyTarget)
			if ownerErr != nil {
				return ownerErr
			}
			current, currentErr := m.Current()
			if currentErr != nil {
				return currentErr
			}
			if !owned && filepath.Clean(current) != filepath.Clean(marker.To) {
				return errors.New("the staged data folder copy is missing")
			}
			copyTarget = marker.To
			marker.Committed = true
		} else if err != nil {
			return err
		}
	}
	if _, err := os.Stat(filepath.Join(marker.From, databaseName)); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("the database is missing in the previous data folder %q; the move was cancelled to avoid data loss", marker.From)
	}
	for _, name := range DatabaseFiles() {
		source := filepath.Join(marker.From, name)
		if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := copyFile(source, filepath.Join(copyTarget, name)); err != nil {
			return err
		}
	}
	if filepath.Clean(copyTarget) != filepath.Clean(marker.To) {
		if err := os.Remove(marker.To); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Rename(copyTarget, marker.To); err != nil {
			if !marker.TargetCreated {
				_ = os.MkdirAll(marker.To, 0o700)
			}
			return err
		}
		marker.Committed = true
		if err := m.writePending(*marker); err != nil {
			return err
		}
	}
	if err := m.writeMarker(pointerFile, marker.To); err != nil {
		return err
	}
	if err := m.writeMarker(previousFile, marker.From); err != nil {
		return err
	}
	if marker.OwnerFile != "" {
		if err := os.Remove(filepath.Join(marker.To, marker.OwnerFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("could not remove the data folder relocation owner marker", "error", err)
		}
	}
	return m.clearPending()
}

// removeOldRoot deletes the previous data root. The launcher home itself is
// never deleted; only its data subdirectories and database files are removed.
func (m *Manager) removeOldRoot(previous string) error {
	if filepath.Clean(previous) == filepath.Clean(m.home) {
		for _, name := range []string{
			"versions", "instances", "downloads", "cache", "security", "updates", "logs", "backups",
		} {
			if err := os.RemoveAll(filepath.Join(m.home, name)); err != nil {
				return err
			}
		}
		for _, name := range DatabaseFiles() {
			if err := os.RemoveAll(filepath.Join(m.home, name)); err != nil {
				return err
			}
		}
		return nil
	}
	return os.RemoveAll(previous)
}

func (m *Manager) filePath(name string) string {
	return filepath.Join(m.home, name)
}

func (m *Manager) readMarkerFile(name string) (string, error) {
	data, err := os.ReadFile(m.filePath(name))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (m *Manager) writeMarker(name, value string) error {
	return atomicfile.Write(m.filePath(name), []byte(value), 0o600)
}

func (m *Manager) clearMarker(name string) error {
	err := os.Remove(m.filePath(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (m *Manager) writePending(marker Marker) error {
	data, err := json.Marshal(marker)
	if err != nil {
		return err
	}
	return atomicfile.Write(m.filePath(pendingFile), data, 0o600)
}

func (m *Manager) clearPending() error {
	if err := m.clearMarker(pendingFile); err != nil {
		slog.Warn("could not clear the relocation marker", "error", err)
		return err
	}
	return nil
}

func (m *Manager) clearError() error {
	return m.clearMarker(errorFile)
}

func (m *Manager) writeError(message string) error {
	if err := m.writeMarker(errorFile, message); err != nil {
		slog.Warn("could not persist the data folder relocation error", "error", err)
		return err
	}
	return nil
}

// cleanupFailedRelocation undoes the partial relocation state after a failed
// copy. The copy error itself is delivered through the relocation error file
// and the completion callback, so failures here are logged only.
func cleanupFailedRelocation(m *Manager, marker Marker, cause error) {
	if err := cleanRelocationTarget(marker); err != nil {
		slog.Warn("could not remove the partial relocation target", "target", marker.copyTarget(), "error", err)
	} else if err := m.clearPending(); err != nil {
		slog.Warn("could not clear the pending relocation marker", "error", err)
	}
	if err := m.writeError(fmt.Sprintf("data folder relocation failed: %v", cause)); err != nil {
		slog.Warn("could not record the data folder copy error", "error", err)
	}
}

func cleanRelocationTarget(marker Marker) error {
	target := marker.copyTarget()
	if marker.CopyTarget != "" {
		if err := os.RemoveAll(target); err != nil {
			return err
		}
		return nil
	}
	if marker.TargetCreated {
		return os.RemoveAll(target)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(target, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (marker Marker) copyTarget() string {
	if marker.CopyTarget != "" {
		return marker.CopyTarget
	}
	return marker.To
}

func validateUnchangedTarget(marker Marker) error {
	entries, err := os.ReadDir(marker.To)
	if marker.TargetCreated && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("the data folder target changed during relocation")
	}
	return nil
}

func relocationTargetOwned(target, ownerFile, owner string) (bool, error) {
	if ownerFile == "" {
		return false, nil
	}
	data, err := os.ReadFile(filepath.Join(target, ownerFile))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return string(data) == owner, nil
}

// RelaunchApplication spawns a fresh copy of the current executable so the old
// process can hand over the data root and exit.
func RelaunchApplication() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	command := exec.Command(executable)
	command.Dir = filepath.Dir(executable)
	return command.Start()
}

func isSubpath(child, base string) bool {
	rel, err := filepath.Rel(base, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
