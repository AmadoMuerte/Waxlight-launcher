// Package gamelog owns game-log tailing, crash indicators, and launcher-side
// crash-report coordination. It forwards error-classified game output and new
// crash reports through the launcher log pipeline while a game runs; it never
// copies the full game output into launcher logs or support exports.
package gamelog

import (
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Game errors captured from a running game are forwarded through the regular
// slog pipeline exactly like launcher errors, so they reach the console, the
// per-session log files, and the support export. They never enter telemetry:
// the telemetry taxonomy has no entries for game output.
const (
	// maxLineLength caps a single forwarded error line.
	maxLineLength = 2000
	// maxBuffer caps how much unbroken game output is kept while waiting for
	// a newline.
	maxBuffer = 128 * 1024
	// maxErrorLinesPerSecond throttles forwarded lines so a noisy game cannot
	// flood the log pipeline.
	maxErrorLinesPerSecond = 50
	// mainLogName is the game's own log file inside the instance Logs
	// directory, followed on a par with the captured stdout stream.
	mainLogName = "main.log"
	// crashReportGlob matches Vintage Story crash report files inside the
	// instance Logs directory.
	crashReportGlob = "crashreport-*.txt"
)

// pollInterval is how often the game output files are polled for new bytes.
// It is a variable so tests can shorten the interval.
var pollInterval = 500 * time.Millisecond

// errorLineRe classifies a game output line as an error: bracketed severity
// markers used by Vintage Story, standalone error phrases, .NET exception type
// names, and continuation lines of a .NET stack trace.
var errorLineRe = regexp.MustCompile(
	`(?i)\[(error|fatal|critical|exception|stacktrace|stack trace|crash)\]` +
		`|\b(?:unhandled exception|stack trace|fatal error|crash report|exception thrown|crashed?)\b` +
		`|\b[A-Z][A-Za-z0-9_.]+Exception\b` +
		`|^\s+at [a-z]` +
		`|(^|[^a-z])error:`,
)

// isErrorLine reports whether a game output line looks like an error.
func isErrorLine(line string) bool {
	return errorLineRe.MatchString(line)
}

// Watch tails the captured game output, the game's own main.log, and any newly
// created crash report inside the instance Logs directory, and forwards error
// lines through the launcher log pipeline while the game runs. It returns a
// stop function that blocks until the tailer has drained the remaining output;
// call it once the game exits.
func Watch(instanceName string, logPath string) func() {
	tailer := newTailer(instanceName, logPath)
	stop := make(chan struct{})
	var once sync.Once
	done := make(chan struct{})
	go func() {
		defer close(done)
		tailer.run(stop)
	}()
	return func() {
		once.Do(func() {
			close(stop)
			<-done
		})
	}
}

// tailer keeps the incremental read state of every followed game output file
// inside one instance Logs directory.
type tailer struct {
	instanceName string
	logPath      string
	dir          string
	files        map[string]*logFile
	seen         map[string]bool
	window       time.Time
	lines        int
}

// logFile is the incremental read state of one followed file. raw files (crash
// reports) are forwarded whole, without severity filtering.
type logFile struct {
	file    *os.File
	offset  int64
	pending string
	raw     bool
}

func newTailer(instanceName string, logPath string) *tailer {
	tailer := &tailer{
		instanceName: instanceName,
		logPath:      logPath,
		dir:          filepath.Dir(logPath),
		files:        map[string]*logFile{},
		seen:         map[string]bool{},
	}
	// The captured stdout stream belongs to this session: forward it from the
	// beginning.
	if state := openLogFile(logPath, false, false); state != nil {
		tailer.files[logPath] = state
	} else {
		slog.Debug("could not open the game output for error capture", "instance", instanceName, "path", logPath)
	}
	// A main.log that already exists belongs to a previous session: only new
	// appends are forwarded.
	mainLog := filepath.Join(tailer.dir, mainLogName)
	if state := openLogFile(mainLog, true, false); state != nil {
		tailer.files[mainLog] = state
	}
	// Crash reports that already exist are stale: never forward them.
	for _, path := range globCrashReports(tailer.dir) {
		tailer.seen[path] = true
	}
	return tailer
}

func (tailer *tailer) run(stop <-chan struct{}) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			tailer.tick(true)
			return
		case <-ticker.C:
		}
		tailer.tick(false)
	}
}

// tick scans for files that appeared since the last poll and reads the bytes
// appended to every followed file. final drains partial lines and skips the
// rate limit so the last lines of a crash are not lost.
func (tailer *tailer) tick(final bool) {
	tailer.scanForNewFiles()
	for path, state := range tailer.files {
		tailer.tickFile(path, state, final)
	}
}

// scanForNewFiles opens a main.log that appeared after the tailer started and
// crash reports created since the last scan.
func (tailer *tailer) scanForNewFiles() {
	if tailer.files[tailer.logPath] == nil {
		if state := openLogFile(tailer.logPath, false, false); state != nil {
			tailer.files[tailer.logPath] = state
		}
	}
	// A main.log created while the game runs belongs to the current session:
	// forward it from the beginning.
	mainLog := filepath.Join(tailer.dir, mainLogName)
	if tailer.files[mainLog] == nil {
		if state := openLogFile(mainLog, false, false); state != nil {
			tailer.files[mainLog] = state
		}
	}
	for _, path := range globCrashReports(tailer.dir) {
		if tailer.seen[path] {
			continue
		}
		tailer.seen[path] = true
		if state := openLogFile(path, false, true); state != nil {
			tailer.files[path] = state
		}
	}
}

func (tailer *tailer) tickFile(path string, state *logFile, final bool) {
	if state.file == nil {
		return
	}
	chunk, err := readChunk(state)
	if err != nil {
		slog.Debug("could not read the game output", "instance", tailer.instanceName, "path", path, "error", err)
		return
	}
	if len(chunk) == 0 && state.pending == "" {
		return
	}
	state.pending += string(chunk)
	if len(state.pending) > maxBuffer {
		// The game printed an unbroken blob; drop it rather than buffer it.
		state.pending = ""
		return
	}
	for {
		newline := strings.IndexByte(state.pending, '\n')
		if newline < 0 {
			break
		}
		tailer.forward(state, state.pending[:newline], final)
		state.pending = state.pending[newline+1:]
	}
	if final && state.pending != "" {
		tailer.forward(state, state.pending, true)
		state.pending = ""
	}
}

func readChunk(state *logFile) ([]byte, error) {
	info, err := state.file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() < state.offset {
		// The file was truncated or replaced after a crash restart.
		state.offset = 0
		state.pending = ""
	}
	if info.Size() == state.offset {
		return nil, nil
	}
	remaining := info.Size() - state.offset
	if remaining > 64*1024 {
		remaining = 64 * 1024
	}
	buffer := make([]byte, remaining)
	read, err := state.file.ReadAt(buffer, state.offset)
	state.offset += int64(read)
	if err != nil && read == 0 {
		// The file shrank between Stat and ReadAt.
		state.offset = 0
		return nil, nil
	}
	return buffer[:read], nil
}

func (tailer *tailer) forward(state *logFile, line string, final bool) {
	line = strings.TrimSpace(line)
	if line == "" || (!state.raw && !isErrorLine(line)) {
		return
	}
	if !final && !tailer.allowLine() {
		return
	}
	if runes := []rune(line); len(runes) > maxLineLength {
		line = string(runes[:maxLineLength]) + "…"
	}
	// Logged through the shared slog pipeline so the line reaches the console,
	// the session log files, and the support export like any other error. The
	// instance name is prefixed so output from parallel instances stays
	// distinguishable.
	slog.Error(tailer.instanceName + ": " + line)
}

// allowLine enforces a simple per-second cap on forwarded lines.
func (tailer *tailer) allowLine() bool {
	now := time.Now()
	if now.Sub(tailer.window) >= time.Second {
		tailer.window = now
		tailer.lines = 0
	}
	if tailer.lines >= maxErrorLinesPerSecond {
		return false
	}
	tailer.lines++
	return true
}

func openLogFile(path string, fromEnd bool, raw bool) *logFile {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	state := &logFile{file: file, raw: raw}
	if fromEnd {
		if info, err := file.Stat(); err == nil {
			state.offset = info.Size()
		}
	}
	return state
}

func globCrashReports(dir string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, crashReportGlob))
	return matches
}
