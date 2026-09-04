package gamelog

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestIsGameErrorLine(t *testing.T) {
	t.Parallel()

	cases := []struct {
		line string
		want bool
	}{
		{"[Error] Could not load texture pack", true},
		{"[Fatal] Out of memory", true},
		{"[Critical] Something broke", true},
		{"[Exception] System.NullReferenceException", true},
		{"[StackTrace] at VintageStory.Program.Main", true},
		{"   at VintageStory.Game.OnClientStart()", true},
		{"An unhandled exception occurred", true},
		{"Fatal error: video driver crashed", true},
		{"error: failed to parse settings", true},
		{"System.NullReferenceException: Object reference not set to an instance of an object", true},
		{"IOException while reading the save", true},
		{"The game crashed during world generation", true},
		{"The game has crashed, sorry about that", true},
		{"[Info] World loaded in 4.2 seconds", false},
		{"[Warning] Texture size is unusual", false},
		{"[Debug] network tick 42", false},
		{"[Notification] Achievement unlocked", false},
		{"Loaded 1280 blocks", false},
		{"Saving world...", false},
		{"No exception was raised during shutdown", false},
		{"[Info] Handled exception during autosave", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.line, func(t *testing.T) {
			t.Parallel()
			if got := isErrorLine(tc.line); got != tc.want {
				t.Errorf("isErrorLine(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

func TestWatchGameLogLogsOnlyErrorLines(t *testing.T) {
	oldInterval := pollInterval
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = oldInterval })

	logPath := filepath.Join(t.TempDir(), "instance.log")
	if err := os.WriteFile(logPath, []byte("[Info] booting\n[Error] first problem\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	records := captureSlog(t)

	instance := "Test World"
	stop := Watch(instance, logPath)

	appendToFile(t, logPath, "[Warning] something odd\n[Error] second problem\n")
	waitFor(t, func() bool { return len(records()) >= 1 })

	stop()
	appendToFile(t, logPath, "   at VintageStory.Game.OnClientStart()\n")
	waitFor(t, func() bool { return len(records()) >= 2 })

	messages := recordMessages(records())
	if len(messages) != 2 {
		t.Fatalf("logged %d game error lines, want 2: %+v", len(messages), messages)
	}
	if messages[0] != "Test World: [Error] first problem" {
		t.Errorf("unexpected first record: %q", messages[0])
	}
	if messages[1] != "Test World: [Error] second problem" {
		t.Errorf("unexpected second record: %q", messages[1])
	}
}

func TestWatchGameLogHandlesTruncation(t *testing.T) {
	oldInterval := pollInterval
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = oldInterval })

	logPath := filepath.Join(t.TempDir(), "instance.log")
	if err := os.WriteFile(logPath, []byte("[Error] before restart\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	records := captureSlog(t)

	stop := Watch("W", logPath)
	waitFor(t, func() bool { return len(records()) >= 1 })

	// Simulate a crash restart that replaces the file: smaller and new content.
	if err := os.WriteFile(logPath, []byte("[Error] after restart\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(records()) >= 2 })
	stop()

	messages := recordMessages(records())
	if messages[1] != "W: [Error] after restart" {
		t.Errorf("truncation was not handled: %+v", messages)
	}
}

func TestWatchGameLogCapturesMainGameLogAppends(t *testing.T) {
	oldInterval := pollInterval
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = oldInterval })

	dir := t.TempDir()
	logPath := filepath.Join(dir, "instance.log")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	mainLog := filepath.Join(dir, "main.log")
	// Content that predates this session must not be replayed.
	if err := os.WriteFile(mainLog, []byte("[Error] stale previous session\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	records := captureSlog(t)
	stop := Watch("W", logPath)

	appendToFile(t, mainLog, "[Error] fresh main.log error\n")
	waitFor(t, func() bool { return len(records()) >= 1 })
	stop()

	messages := recordMessages(records())
	if len(messages) != 1 || messages[0] != "W: [Error] fresh main.log error" {
		t.Fatalf("unexpected records: %+v", messages)
	}
}

func TestWatchGameLogReleasesFilesOnStop(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "instance.log")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	stop := Watch("W", logPath)
	stop()

	if err := os.Rename(dir, filepath.Join(filepath.Dir(dir), filepath.Base(dir)+"-renamed")); err != nil {
		t.Fatalf("instance directory remained locked after stopping log watcher: %v", err)
	}
}

func TestWatchGameLogForwardsNewCrashReportWhole(t *testing.T) {
	oldInterval := pollInterval
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = oldInterval })

	dir := t.TempDir()
	logPath := filepath.Join(dir, "instance.log")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	records := captureSlog(t)
	stop := Watch("W", logPath)

	// A crash report is written after the game starts; every line is an error,
	// even without severity markers.
	report := filepath.Join(dir, "crashreport-2026-01-01-12-00-00.txt")
	if err := os.WriteFile(
		report,
		[]byte("--- Crash report ---\nSystem.NullReferenceException: boom\n   at Game.Update()\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(records()) >= 3 })
	stop()

	messages := recordMessages(records())
	if len(messages) != 3 {
		t.Fatalf("forwarded %d crash report lines, want 3: %+v", len(messages), messages)
	}
	if messages[0] != "W: --- Crash report ---" {
		t.Errorf("unexpected first line: %q", messages[0])
	}
	if messages[1] != "W: System.NullReferenceException: boom" {
		t.Errorf("unexpected second line: %q", messages[1])
	}
	if messages[2] != "W: at Game.Update()" {
		t.Errorf("unexpected third line: %q", messages[2])
	}
}

func TestWatchGameLogIgnoresStaleCrashReport(t *testing.T) {
	oldInterval := pollInterval
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = oldInterval })

	dir := t.TempDir()
	logPath := filepath.Join(dir, "instance.log")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dir, "crashreport-stale.txt")
	if err := os.WriteFile(stale, []byte("[Error] stale crash\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	records := captureSlog(t)
	stop := Watch("W", logPath)
	time.Sleep(60 * time.Millisecond)
	stop()

	if messages := recordMessages(records()); len(messages) != 0 {
		t.Fatalf("stale crash report was forwarded: %+v", messages)
	}
}

func recordMessages(records []slog.Record) []string {
	messages := make([]string, 0, len(records))
	for _, record := range records {
		messages = append(messages, record.Message)
	}
	return messages
}

// slogRecorder captures records produced through the default slog handler.
type slogRecorder struct {
	mu      sync.Mutex
	records []slog.Record
}

func (recorder *slogRecorder) Enabled(context.Context, slog.Level) bool { return true }

func (recorder *slogRecorder) Handle(_ context.Context, record slog.Record) error {
	recorder.mu.Lock()
	recorder.records = append(recorder.records, record)
	recorder.mu.Unlock()
	return nil
}

func (recorder *slogRecorder) WithAttrs(attrs []slog.Attr) slog.Handler { return recorder }

func (recorder *slogRecorder) WithGroup(string) slog.Handler { return recorder }

// captureSlog installs a capturing slog handler and returns a snapshot
// function. The previous default handler is restored when the test ends.
func captureSlog(t *testing.T) func() []slog.Record {
	t.Helper()
	recorder := &slogRecorder{}
	original := slog.Default()
	slog.SetDefault(slog.New(recorder))
	t.Cleanup(func() { slog.SetDefault(original) })
	return func() []slog.Record {
		recorder.mu.Lock()
		defer recorder.mu.Unlock()
		return append([]slog.Record(nil), recorder.records...)
	}
}

func appendToFile(t *testing.T, path string, contents string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(contents); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}
