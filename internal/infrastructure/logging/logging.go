// Package logging is a standalone, dependency-free logging facility for the
// Waxlight launcher. It keeps a bounded in-memory history of recent entries so
// the UI can render a live console and export the latest lines for support,
// and it can mirror every line to a rolling set of files under a directory so
// logs survive process restarts.
//
// The package knows nothing about Wails, the domain model, or the application
// layer. External integration happens through three seams:
//
//   - Setup installs a slog handler as the process default, so every
//     slog.Info/Warn/Error call anywhere in the codebase is captured here.
//   - SetLogDirectory enables writing to per-session files in a directory,
//     keeping only the newest maxFiles files.
//   - SetEmitter lets the host register a callback (for example one that
//     publishes Wails events) that receives every new entry as it happens.
package logging

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// logFilePrefix is the prefix of per-session log files inside the configured
// log directory.
const logFilePrefix = "launcher-"

// DefaultMaxLogFiles is how many per-session log files are kept on disk.
const DefaultMaxLogFiles = 5

// Level is the human readable severity of a log entry.
type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

// Entry is a single captured log line.
type Entry struct {
	Time    time.Time `json:"time"`
	Level   Level     `json:"level"`
	Message string    `json:"message"`
}

// Line renders the entry as a single terminal line with ANSI colors. The
// resulting string is safe to feed to an xterm-compatible console.
func (entry Entry) Line() string {
	stamp := entry.Time.Format("2006-01-02 15:04:05.000")
	code := ansiCode(entry.Level)
	return fmt.Sprintf("\x1b[90m%s\x1b[0m \x1b[%dm%-5s\x1b[0m %s", stamp, code, entry.Level, entry.Message)
}

// Plain renders the entry without ANSI colors for log files.
func (entry Entry) Plain() string {
	return fmt.Sprintf(
		"%s | %-5s | %s",
		entry.Time.Format("2006-01-02 15:04:05.000"),
		entry.Level,
		entry.Message,
	)
}

func ansiCode(level Level) int {
	switch level {
	case LevelDebug:
		return 90
	case LevelInfo:
		return 37
	case LevelWarn:
		return 33
	default:
		return 31
	}
}

var (
	setupMu   sync.RWMutex
	buffer    *RingBuffer
	emitter   func(Entry)
	emitterMu sync.RWMutex

	fileMu      sync.Mutex
	logDir      string
	logDirSet   bool
	maxLogFiles int
	logFile     *os.File
	fileHeader  string
)

// Setup initializes the shared logger with a ring capacity of at least
// capacity entries and installs its handler as slog's process default. Calling
// Setup more than once replaces the previous buffer. It is safe to call before
// any other logging function.
func Setup(capacity int) {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	setupMu.Lock()
	defer setupMu.Unlock()
	buffer = NewRingBuffer(capacity)
	slog.SetDefault(slog.New(&handler{}))
}

// SetEmitter registers a callback invoked for every new entry after it has
// been stored. A nil callback disables emission. This is the only seam used
// to push log lines to the UI.
func SetEmitter(fn func(Entry)) {
	emitterMu.Lock()
	defer emitterMu.Unlock()
	emitter = fn
}

// SetLogDirectory enables writing every new entry to a per-session file inside
// dir. A fresh file is opened on the first write after the call; only the
// newest maxFiles session files are kept and older ones are pruned. Passing an
// empty dir disables file logging. Calling it again re-targets the sink (for
// example after the data folder is relocated).
func SetLogDirectory(dir string, maxFiles int) {
	fileMu.Lock()
	defer fileMu.Unlock()
	if maxFiles < 1 {
		maxFiles = DefaultMaxLogFiles
	}
	logDir = dir
	logDirSet = dir != ""
	maxLogFiles = maxFiles
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
}

// LogDirectory returns the directory configured for file logging, or "" when
// file logging is disabled.
func LogDirectory() string {
	fileMu.Lock()
	defer fileMu.Unlock()
	return logDir
}

// SetFileHeader sets a multi-line preamble (launcher version, platform, and so
// on) written at the top of every newly opened session log file. It must be
// called before the first entry is written to affect the current session.
func SetFileHeader(header string) {
	fileMu.Lock()
	defer fileMu.Unlock()
	fileHeader = header
}

// CloseFileLog closes the currently open session log file. The next entry
// opens a new session file.
func CloseFileLog() {
	fileMu.Lock()
	defer fileMu.Unlock()
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
}

// appendEntry stores an entry, notifies the external emitter, mirrors the line
// to the session log file, and writes it to stderr so terminal-based runs
// still see the log.
func appendEntry(entry Entry) {
	setupMu.RLock()
	ring := buffer
	setupMu.RUnlock()
	if ring != nil {
		ring.Append(entry)
	}
	emitterMu.RLock()
	notify := emitter
	emitterMu.RUnlock()
	if notify != nil {
		notify(entry)
	}
	writeToFile(entry)
	logRedactedLine(entry)
}

// writeToFile appends the entry to the current session log file, opening and
// pruning one on first use. Failures are silent so logging never breaks the
// application.
func writeToFile(entry Entry) {
	fileMu.Lock()
	defer fileMu.Unlock()
	if !logDirSet {
		return
	}
	if logFile == nil {
		if err := openLogFile(); err != nil {
			logDirSet = false
			return
		}
	}
	_, _ = fmt.Fprintln(logFile, entry.Plain())
}

func openLogFile() error {
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return err
	}
	name := logFilePrefix + time.Now().UTC().Format("20060102-150405") + ".log"
	file, err := os.OpenFile(filepath.Join(logDir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	logFile = file
	if fileHeader != "" {
		_, _ = fmt.Fprintln(file, fileHeader)
		_, _ = fmt.Fprintln(file)
	}
	pruneLogFiles(logDir, maxLogFiles)
	return nil
}

// pruneLogFiles removes the oldest session log files so at most keep remain.
func pruneLogFiles(dir string, keep int) {
	matches, err := filepath.Glob(filepath.Join(dir, logFilePrefix+"*.log"))
	if err != nil || len(matches) <= keep {
		return
	}
	sort.Slice(matches, func(i, j int) bool {
		return fileModTime(matches[i]).After(fileModTime(matches[j]))
	})
	for _, path := range matches[keep:] {
		_ = os.Remove(path)
	}
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// Snapshot returns every stored entry in chronological order. It returns nil
// when Setup has not been called yet.
func Snapshot() []Entry {
	setupMu.RLock()
	defer setupMu.RUnlock()
	if buffer == nil {
		return nil
	}
	return buffer.Snapshot()
}

// SnapshotLines returns the stored entries rendered as colored terminal lines.
func SnapshotLines() []string {
	entries := Snapshot()
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, entry.Line())
	}
	return lines
}

// Clear empties the in-memory history.
func Clear() {
	setupMu.RLock()
	defer setupMu.RUnlock()
	if buffer != nil {
		buffer.Clear()
	}
}

// Fatal logs an error and terminates the process with a non-zero exit code. It
// replaces bare log.Fatal calls so fatal startup failures still reach the
// in-memory history and stderr through the shared logger.
func Fatal(message string, err error) {
	slog.Error(message, "error", err)
	os.Exit(1)
}

// ContainsSensitiveKey reports whether text references a sensitive field name.
func ContainsSensitiveKey(text string) bool {
	return containsSensitiveKey(text)
}
