package wails

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/platform/logging"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

func TestFormatSupportLogIncludesSummaryAndLog(t *testing.T) {
	logging.Setup(16)
	defer logging.Clear()
	slog.Info("instance created", "instance", "My world")

	data := supportLogData{
		GeneratedAt: "2026-08-06T00:00:00Z",
		Version:     "0.3.0",
		Platform:    "linux/amd64",
		GoVersion:   "go1.23",
		Versions:    []versions.GameVersion{{ID: "1.20", Name: "1.20"}},
		Instances: []supportLogInstance{
			{Name: "My world", GameVersionID: "1.20", ModCount: 3, EnabledModCount: 2},
		},
	}
	report := formatSupportLog(data, logging.Snapshot())

	for _, want := range []string{
		"Waxlight Launcher support log",
		"Version: 0.3.0",
		"Platform: linux/amd64",
		"Installed game versions:",
		"- 1.20 (1.20)",
		"Instances:",
		"- My world [1.20] mods 3 enabled 2",
		"Launcher log:",
		"instance created instance=My world",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("support log missing %q:\n%s", want, report)
		}
	}
}

func TestFormatSupportLogExcludesSensitiveValues(t *testing.T) {
	logging.Setup(16)
	defer logging.Clear()
	slog.Info("login", "sessionkey", "TOP-SECRET-SESSION", "email", "player@example.com")

	data := supportLogData{GeneratedAt: "now", Version: "0.3.0"}
	report := formatSupportLog(data, logging.Snapshot())
	for _, forbidden := range []string{"TOP-SECRET-SESSION", "player@example.com"} {
		if strings.Contains(report, forbidden) {
			t.Fatalf("support log leaked %q:\n%s", forbidden, report)
		}
	}
}

func TestListLogsReturnsRecentLines(t *testing.T) {
	logging.Setup(16)
	defer logging.Clear()
	slog.Info("first")
	slog.Info("second")
	slog.Info("third")

	controller := &LogController{}
	lines, err := controller.ListLogs(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "second") || !strings.Contains(lines[1], "third") {
		t.Fatalf("unexpected recent lines: %v", lines)
	}

	all, err := controller.ListLogs(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 lines with no limit, got %d", len(all))
	}
}

func TestWriteLogRoutesIntoLoggingPipeline(t *testing.T) {
	logging.Setup(16)
	defer logging.Clear()

	controller := &LogController{}
	if err := controller.WriteLog("warn", "render failed", map[string]string{"component": "Sidebar"}); err != nil {
		t.Fatal(err)
	}

	snapshot := logging.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(snapshot))
	}
	entry := snapshot[0]
	if entry.Level != logging.LevelWarn {
		t.Fatalf("expected WARN level, got %s", entry.Level)
	}
	if !strings.Contains(entry.Message, "render failed") {
		t.Fatalf("message missing: %q", entry.Message)
	}
	for _, want := range []string{"source=frontend", "component=Sidebar"} {
		if !strings.Contains(entry.Message, want) {
			t.Fatalf("entry missing %q: %q", want, entry.Message)
		}
	}
}

func TestWriteLogValidatesInput(t *testing.T) {
	logging.Setup(16)
	defer logging.Clear()

	controller := &LogController{}
	for _, level := range []string{"fatal", "TRACE", "verbose"} {
		if err := controller.WriteLog(level, "message", nil); err == nil {
			t.Fatalf("expected validation error for level %q", level)
		}
	}
	if err := controller.WriteLog("info", "   ", nil); err == nil {
		t.Fatal("expected validation error for empty message")
	}
	if err := controller.WriteLog("info", "ok", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteLogRedactsSensitiveValues(t *testing.T) {
	logging.Setup(16)
	defer logging.Clear()

	controller := &LogController{}
	if err := controller.WriteLog("error", "request failed", map[string]string{"sessionkey": "TOP-SECRET"}); err != nil {
		t.Fatal(err)
	}

	snapshot := logging.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(snapshot))
	}
	if strings.Contains(snapshot[0].Message, "TOP-SECRET") {
		t.Fatalf("sensitive value leaked: %q", snapshot[0].Message)
	}
}
