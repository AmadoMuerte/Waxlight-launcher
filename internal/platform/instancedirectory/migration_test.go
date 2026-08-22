package instancedirectory_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/platform/filesystem"
	"github.com/waxlight/waxlight-launcher/internal/platform/instancedirectory"
)

func TestMigrationInspectionAndCopy(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	for _, directory := range []string{"Saves/world-a", "SaveGame/world-b", "Mods", "ModsDisabled", "ModConfig", "Cache", "Logs", "Unknown/nested", target} {
		if err := os.MkdirAll(filepath.Join(source, directory), 0o755); err != nil && directory != target {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"Saves/world-c.vcdbs": "world-c",
		"Mods/a.zip":          "mod-a",
		"Mods/ignored.txt":    "not-a-mod",
		"ModsDisabled/b.dll":  "mod-b",
		"Unknown/nested/data": "unknown",
		"Cache/stale":         "cache",
		"Logs/main.log":       "Vintage Story v1.20.4\n",
		"Logs/second.log":     "Game version 1.20.4\n",
		"clientsettings.json": `{"stringSettings":{"sessionkey":"secret","foo":"bar"},"stringListSettings":{"modPaths":["/old/data/Mods"]}}`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(source, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(source, "Unknown"), filepath.Join(source, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	storage := instancedirectory.NewMigrationStorage(filesystem.SanitizeClientSettings)
	candidate, err := storage.Inspect(source)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.WorldCount != 3 || candidate.ModCount != 2 || !candidate.HasClientSettings || !candidate.HasModConfig {
		t.Fatalf("unexpected inspection: %#v", candidate)
	}
	if candidate.DetectedGameVersion != "1.20.4" || candidate.VersionConfidence != "high" {
		t.Fatalf("unexpected version detection: %#v", candidate)
	}
	if !warningsContain(candidate.Warnings, "Symbolic link") {
		t.Fatalf("missing symlink warning: %v", candidate.Warnings)
	}
	result, err := storage.Copy(context.Background(), source, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !warningsContain(result.Warnings, "symbolic link") {
		t.Fatalf("missing copy warning: %v", result.Warnings)
	}
	if contents, err := os.ReadFile(filepath.Join(target, "Unknown", "nested", "data")); err != nil || string(contents) != "unknown" {
		t.Fatalf("unknown content was not copied: %q, %v", contents, err)
	}
	for _, excluded := range []string{"Cache/stale", "Logs/main.log", "linked"} {
		if _, err := os.Lstat(filepath.Join(target, excluded)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("excluded path was copied: %s", excluded)
		}
	}
	settings, err := os.ReadFile(filepath.Join(target, "clientsettings.json"))
	normalizedSettings := strings.ToLower(string(settings))
	if err != nil || strings.Contains(normalizedSettings, "sessionkey") || strings.Contains(normalizedSettings, "modpaths") || !strings.Contains(normalizedSettings, `"foo": "bar"`) {
		t.Fatalf("settings were not sanitized: %s, %v", settings, err)
	}
}

func TestMigrationInspectionRejectsMissingAndUnrecognizedDirectories(t *testing.T) {
	storage := instancedirectory.NewMigrationStorage(filesystem.SanitizeClientSettings)
	if _, err := storage.Inspect(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing-directory error, got %v", err)
	}
	if _, err := storage.Inspect(t.TempDir()); err == nil {
		t.Fatal("unrecognized directory was accepted")
	}
}

func TestMigrationInspectionDoesNotGuessVersionFromModLogs(t *testing.T) {
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "Logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "Mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "Logs", "main.log"), []byte("Loaded ExampleMod 9.8.7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	candidate, err := instancedirectory.NewMigrationStorage(filesystem.SanitizeClientSettings).Inspect(source)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.DetectedGameVersion != "" || candidate.VersionConfidence != "" {
		t.Fatalf("unexpected version guess: %#v", candidate)
	}
}

func TestMigrationInspectionDoesNotReadSymlinkedSettings(t *testing.T) {
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "Mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "clientsettings.json")
	if err := os.WriteFile(external, []byte(`{"stringSettings":{"sessionkey":"secret"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(source, "clientsettings.json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	candidate, err := instancedirectory.NewMigrationStorage(filesystem.SanitizeClientSettings).Inspect(source)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.HasClientSettings || !warningsContain(candidate.Warnings, "symbolic link") {
		t.Fatalf("symlinked settings were inspected: %#v", candidate)
	}
}

func TestMigrationSkipsMalformedSettings(t *testing.T) {
	source, target := filepath.Join(t.TempDir(), "source"), filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(filepath.Join(source, "Mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "clientsettings.json"), []byte(`{"broken"`), 0o600); err != nil {
		t.Fatal(err)
	}
	storage := instancedirectory.NewMigrationStorage(filesystem.SanitizeClientSettings)
	candidate, err := storage.Inspect(source)
	if err != nil || !warningsContain(candidate.Warnings, "Malformed") {
		t.Fatalf("malformed settings not reported: %#v, %v", candidate, err)
	}
	result, err := storage.Copy(context.Background(), source, target, nil)
	if err != nil || !warningsContain(result.Warnings, "malformed") {
		t.Fatalf("malformed settings not skipped: %#v, %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(target, "clientsettings.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("malformed settings were copied")
	}
}

func TestMigrationCancellationLeavesSourceUnchanged(t *testing.T) {
	source, target := filepath.Join(t.TempDir(), "source"), filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(filepath.Join(source, "Unknown"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("unchanged")
	path := filepath.Join(source, "Unknown", "data")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := instancedirectory.NewMigrationStorage(filesystem.SanitizeClientSettings).Copy(ctx, source, target, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	contents, readErr := os.ReadFile(path)
	if readErr != nil || string(contents) != string(original) {
		t.Fatalf("source changed: %q, %v", contents, readErr)
	}
}

func TestMigrationRejectsOverlappingDestination(t *testing.T) {
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "Mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	storage := instancedirectory.NewMigrationStorage(filesystem.SanitizeClientSettings)
	if err := storage.ValidateTarget(source, filepath.Join(source, "new-instance")); err == nil {
		t.Fatal("destination inside source was accepted")
	}
	if _, err := os.Stat(filepath.Join(source, "new-instance")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("overlap validation changed the source")
	}
}

func warningsContain(warnings []string, part string) bool {
	for _, warning := range warnings {
		if strings.Contains(strings.ToLower(warning), strings.ToLower(part)) {
			return true
		}
	}
	return false
}
