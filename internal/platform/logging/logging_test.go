package logging

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSlogRecordsLandInBuffer(t *testing.T) {
	Setup(64)
	slog.Info("hello world")
	slog.Warn("careful now", "key", "value")
	slog.Error("boom", "error", "connection refused")

	snapshot := Snapshot()
	if len(snapshot) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(snapshot))
	}
	if snapshot[0].Message != "hello world" || snapshot[0].Level != LevelInfo {
		t.Fatalf("unexpected first entry: %+v", snapshot[0])
	}
	if snapshot[1].Level != LevelWarn || !strings.Contains(snapshot[1].Message, "key=value") {
		t.Fatalf("unexpected warn entry: %+v", snapshot[1])
	}
	if snapshot[2].Level != LevelError || snapshot[2].Message != "boom error=connection refused" {
		t.Fatalf("unexpected error entry: %+v", snapshot[2])
	}
}

func TestSlogRedactsSensitiveAttributes(t *testing.T) {
	Setup(16)
	slog.Info("login", "sessionkey", "TOP-SECRET-VALUE", "playername", "Ada")

	snapshot := Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(snapshot))
	}
	if strings.Contains(snapshot[0].Message, "TOP-SECRET-VALUE") {
		t.Fatalf("sensitive attribute leaked: %q", snapshot[0].Message)
	}
	if !strings.Contains(snapshot[0].Message, "playername=Ada") {
		t.Fatalf("non-sensitive attribute was lost: %q", snapshot[0].Message)
	}
}

func TestEmitterReceivesEntries(t *testing.T) {
	Setup(16)
	var received atomic.Int32
	SetEmitter(func(Entry) { received.Add(1) })
	defer SetEmitter(nil)

	slog.Info("one")
	slog.Info("two")
	if received.Load() != 2 {
		t.Fatalf("expected 2 emitted entries, got %d", received.Load())
	}
}

func TestEntryLineIsAnsiColored(t *testing.T) {
	entry := Entry{Message: "ping"}
	line := entry.Line()
	if !strings.Contains(line, "\x1b[") || !strings.Contains(line, "ping") {
		t.Fatalf("expected colored line, got %q", line)
	}
}

func TestClearEmptiesHistory(t *testing.T) {
	Setup(16)
	slog.Info("first")
	Clear()
	if snapshot := Snapshot(); len(snapshot) != 0 {
		t.Fatalf("expected empty history, got %d entries", len(snapshot))
	}
}

func TestWithAttrsKeepsWorking(t *testing.T) {
	Setup(16)
	logger := slog.Default().With("instance", "alpha")
	logger.Info("started")
	snapshot := Snapshot()
	if len(snapshot) != 1 || !strings.Contains(snapshot[0].Message, "instance=alpha") {
		t.Fatalf("With attrs not captured: %+v", snapshot)
	}
}

var _ = context.Background

func TestSetLogDirectoryWritesSessionFile(t *testing.T) {
	Setup(16)
	dir := t.TempDir()
	SetLogDirectory(dir, 3)
	defer SetLogDirectory("", 0)
	slog.Info("written to file")

	matches, err := filepath.Glob(filepath.Join(dir, logFilePrefix+"*.log"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one session log file, got %d", len(matches))
	}
	contents, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "written to file") {
		t.Fatalf("session log file is missing the entry: %q", contents)
	}
	if strings.Contains(string(contents), "\x1b[") {
		t.Fatalf("session log file must not contain ANSI codes: %q", contents)
	}
}

func TestSetLogDirectoryPrunesOldFiles(t *testing.T) {
	Setup(16)
	dir := t.TempDir()
	base := time.Now().Add(-time.Hour)
	for index := 0; index < 3; index++ {
		path := filepath.Join(dir, logFilePrefix+"old-"+string(rune('a'+index))+".log")
		if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
		stamp := base.Add(time.Duration(index) * time.Minute)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	SetLogDirectory(dir, 3)
	defer SetLogDirectory("", 0)
	slog.Info("new session line")

	matches, err := filepath.Glob(filepath.Join(dir, logFilePrefix+"*.log"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 3 {
		t.Fatalf("expected 3 kept log files, got %d", len(matches))
	}
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.ModTime().Equal(base) {
			t.Fatalf("the oldest log file was not pruned: %s", path)
		}
	}
}

func TestLogDirectoryReportsConfiguredDir(t *testing.T) {
	Setup(16)
	dir := t.TempDir()
	SetLogDirectory(dir, 5)
	defer SetLogDirectory("", 0)
	if LogDirectory() != dir {
		t.Fatalf("expected %q, got %q", dir, LogDirectory())
	}
	SetLogDirectory("", 0)
	if LogDirectory() != "" {
		t.Fatalf("expected disabled file logging, got %q", LogDirectory())
	}
}

func TestSessionFileStartsWithHeader(t *testing.T) {
	Setup(16)
	dir := t.TempDir()
	SetFileHeader("Waxlight Launcher 0.3.0\nPlatform: linux/amd64\nStarted: now")
	SetLogDirectory(dir, 3)
	defer SetLogDirectory("", 0)
	slog.Info("first line after header")

	matches, err := filepath.Glob(filepath.Join(dir, logFilePrefix+"*.log"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one session log file, got %d", len(matches))
	}
	contents, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	if !strings.HasPrefix(text, "Waxlight Launcher 0.3.0\nPlatform: linux/amd64\nStarted: now\n\n") {
		t.Fatalf("session file must start with the header, got:\n%s", text)
	}
	if !strings.Contains(text, "first line after header") {
		t.Fatalf("session file is missing the log line:\n%s", text)
	}
}
