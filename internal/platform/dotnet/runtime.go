// Package dotnet detects compatible .NET runtimes installed on Linux.
package dotnet

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

const RequiredMajor = 10

type Runtime struct {
	Root       string
	Version    string
	Source     string
	Executable string
}

type Detector struct {
	GOOS      string
	Getenv    func(string) string
	UserHome  func() (string, error)
	LookPath  func(string) (string, error)
	EvalLinks func(string) (string, error)
	RunDotnet func(string) ([]byte, error)
	MapRoot   func(string) string
}

type RuntimeDetector interface {
	Detect(requiredMajor int) (Runtime, bool)
}

func NewDetector(goos string) Detector {
	return Detector{
		GOOS:      goos,
		Getenv:    os.Getenv,
		UserHome:  os.UserHomeDir,
		LookPath:  exec.LookPath,
		EvalLinks: filepath.EvalSymlinks,
		RunDotnet: func(path string) ([]byte, error) { return exec.Command(path, "--info").Output() },
		MapRoot:   func(path string) string { return path },
	}
}

func (detector Detector) Detect(requiredMajor int) (Runtime, bool) {
	if detector.GOOS != "linux" {
		return Runtime{}, false
	}

	executable := ""
	if path, err := detector.LookPath("dotnet"); err == nil {
		executable = path
	}

	type candidate struct{ root, source string }
	candidates := []candidate{
		{detector.Getenv("DOTNET_ROOT"), "DOTNET_ROOT"},
		{detector.Getenv("DOTNET_INSTALL_DIR"), "DOTNET_INSTALL_DIR"},
	}
	if executable != "" {
		if resolved, err := detector.EvalLinks(executable); err == nil {
			candidates = append(candidates, candidate{filepath.Dir(resolved), "executable"})
		}
		if output, err := detector.RunDotnet(executable); err == nil {
			if root := runtimeRootFromInfo(output); root != "" {
				candidates = append(candidates, candidate{root, "dotnet_info"})
			}
		}
	}
	candidates = append(candidates,
		candidate{"/usr/share/dotnet", "system"},
		candidate{"/usr/lib/dotnet", "system"},
		candidate{"/usr/local/share/dotnet", "system"},
		candidate{"/usr/local/lib/dotnet", "system"},
	)
	if home, err := detector.UserHome(); err == nil && home != "" {
		candidates = append(candidates, candidate{filepath.Join(home, ".dotnet"), "user"})
	}
	candidates = append(candidates, candidate{"/var/snap/dotnet/common/dotnet", "snap"})

	seen := make(map[string]bool)
	for _, candidate := range candidates {
		root := filepath.Clean(strings.TrimSpace(detector.MapRoot(candidate.root)))
		if candidate.root == "" || seen[root] {
			continue
		}
		seen[root] = true
		if version, ok := compatibleVersion(root, requiredMajor); ok {
			return Runtime{Root: root, Version: version, Source: candidate.source, Executable: executable}, true
		}
	}
	return Runtime{Executable: executable}, false
}

func compatibleVersion(root string, requiredMajor int) (string, bool) {
	if !hasVersion(filepath.Join(root, "host", "fxr")) {
		return "", false
	}
	entries, err := os.ReadDir(filepath.Join(root, "shared", "Microsoft.NETCore.App"))
	if err != nil {
		return "", false
	}
	best := ""
	for _, entry := range entries {
		version := entry.Name()
		parsed := "v" + version
		if entry.IsDir() && semver.IsValid(parsed) && semver.Major(parsed) == "v"+strconv.Itoa(requiredMajor) && (best == "" || semver.Compare(parsed, "v"+best) > 0) {
			best = version
		}
	}
	return best, best != ""
}

func hasVersion(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() && semver.IsValid("v"+entry.Name()) {
			return true
		}
	}
	return false
}

func runtimeRootFromInfo(output []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	inRuntimes := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == ".NET runtimes installed:" {
			inRuntimes = true
			continue
		}
		if inRuntimes && strings.HasPrefix(line, "Microsoft.NETCore.App ") {
			open := strings.LastIndex(line, "[")
			if open >= 0 && strings.HasSuffix(line, "]") {
				return filepath.Dir(filepath.Dir(line[open+1 : len(line)-1]))
			}
		}
	}
	return ""
}
