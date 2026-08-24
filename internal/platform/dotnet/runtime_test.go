package dotnet

import (
	"os"
	"path/filepath"
	"testing"
)

type detectionTest struct {
	name        string
	environment map[string]string
	home        bool
	paths       map[string][]string
	wantSource  string
	wantVersion string
}

func TestDetectCompatibleRuntime(t *testing.T) {
	tests := []detectionTest{
		{name: "existing DOTNET_ROOT", environment: map[string]string{"DOTNET_ROOT": "env"}, paths: map[string][]string{"env": {"10.0.11"}}, wantSource: "DOTNET_ROOT", wantVersion: "10.0.11"},
		{name: "snap", paths: map[string][]string{"snap": {"10.0.11"}}, wantSource: "snap", wantVersion: "10.0.11"},
		{name: "user", home: true, paths: map[string][]string{"user": {"10.0.8"}}, wantSource: "user", wantVersion: "10.0.8"},
		{name: "system", paths: map[string][]string{"system": {"10.0.7"}}, wantSource: "system", wantVersion: "10.0.7"},
		{name: "newest compatible", environment: map[string]string{"DOTNET_ROOT": "env"}, paths: map[string][]string{"env": {"10.0.3", "10.0.11", "10.1.0"}}, wantSource: "DOTNET_ROOT", wantVersion: "10.1.0"},
		{name: "invalid env continues", environment: map[string]string{"DOTNET_ROOT": "broken"}, paths: map[string][]string{"system": {"10.0.11"}}, wantSource: "system", wantVersion: "10.0.11"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			locations := map[string]string{
				"env":    filepath.Join(root, "custom", "dotnet"),
				"broken": filepath.Join(root, "broken"),
				"user":   filepath.Join(root, "home", ".dotnet"),
				"system": filepath.Join(root, "usr", "lib", "dotnet"),
				"snap":   filepath.Join(root, "var", "snap", "dotnet", "common", "dotnet"),
			}
			for name, versions := range test.paths {
				makeRuntime(t, locations[name], versions...)
			}
			detector := testDetector(root, test.environment, test.home)
			got, ok := detector.Detect(RequiredMajor)
			if !ok || got.Root != locations[test.wantSourceKey()] || got.Source != test.wantSource || got.Version != test.wantVersion {
				t.Fatalf("Detect() = %#v, %v", got, ok)
			}
		})
	}
}

func TestDetectRejectsWrongOrBrokenRuntime(t *testing.T) {
	root := t.TempDir()
	makeRuntime(t, filepath.Join(root, "usr", "lib", "dotnet"), "8.0.12")
	if err := os.MkdirAll(filepath.Join(root, "var", "snap", "dotnet", "common", "dotnet", "host", "fxr", "10.0.11"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := testDetector(root, nil, false).Detect(RequiredMajor); ok {
		t.Fatalf("Detect() = %#v, want not found", got)
	}
}

func TestDetectUsesDotnetInfoRuntimeRoot(t *testing.T) {
	root := t.TempDir()
	runtimeRoot := filepath.Join(root, "snap-runtime")
	makeRuntime(t, runtimeRoot, "10.0.11")
	detector := NewDetector("linux")
	detector.Getenv = func(string) string { return "" }
	detector.UserHome = func() (string, error) { return "", nil }
	detector.LookPath = func(string) (string, error) { return "/snap/bin/dotnet", nil }
	detector.EvalLinks = func(string) (string, error) { return "/snap/dotnet/current/dotnet", nil }
	detector.RunDotnet = func(string) ([]byte, error) {
		return []byte(".NET runtimes installed:\nMicrosoft.NETCore.App 10.0.11 [" + filepath.Join(runtimeRoot, "shared", "Microsoft.NETCore.App") + "]\n"), nil
	}
	detector.MapRoot = func(path string) string {
		if path == runtimeRoot {
			return path
		}
		return filepath.Join(root, "missing", filepath.Base(path))
	}

	got, ok := detector.Detect(RequiredMajor)
	if !ok || got.Root != runtimeRoot || got.Source != "dotnet_info" || got.Executable != "/snap/bin/dotnet" {
		t.Fatalf("Detect() = %#v, %v", got, ok)
	}
}

func (test detectionTest) wantSourceKey() string {
	switch test.wantSource {
	case "DOTNET_ROOT":
		return "env"
	default:
		return test.wantSource
	}
}

func testDetector(root string, environment map[string]string, home bool) Detector {
	detector := NewDetector("linux")
	detector.Getenv = func(key string) string {
		value := environment[key]
		if value == "" {
			return ""
		}
		return value
	}
	detector.UserHome = func() (string, error) {
		if home {
			return filepath.Join(root, "home"), nil
		}
		return "", nil
	}
	detector.LookPath = func(string) (string, error) { return "", os.ErrNotExist }
	detector.RunDotnet = func(string) ([]byte, error) { return nil, os.ErrNotExist }
	detector.EvalLinks = filepath.EvalSymlinks
	detectorRoots := map[string]string{
		"env":                            filepath.Join(root, "custom", "dotnet"),
		"broken":                         filepath.Join(root, "broken"),
		"/usr/share/dotnet":              filepath.Join(root, "usr", "share", "dotnet"),
		"/usr/lib/dotnet":                filepath.Join(root, "usr", "lib", "dotnet"),
		"/usr/local/share/dotnet":        filepath.Join(root, "usr", "local", "share", "dotnet"),
		"/usr/local/lib/dotnet":          filepath.Join(root, "usr", "local", "lib", "dotnet"),
		"/var/snap/dotnet/common/dotnet": filepath.Join(root, "var", "snap", "dotnet", "common", "dotnet"),
	}
	detector.MapRoot = func(path string) string {
		if mapped := detectorRoots[path]; mapped != "" {
			return mapped
		}
		return path
	}
	return detector
}

func makeRuntime(t *testing.T, root string, versions ...string) {
	t.Helper()
	for _, path := range append([]string{filepath.Join(root, "host", "fxr", "10.0.11")}, runtimePaths(root, versions...)...) {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func runtimePaths(root string, versions ...string) []string {
	paths := make([]string, 0, len(versions))
	for _, version := range versions {
		paths = append(paths, filepath.Join(root, "shared", "Microsoft.NETCore.App", version))
	}
	return paths
}
