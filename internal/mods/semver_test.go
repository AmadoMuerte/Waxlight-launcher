package mods

import "testing"

func TestFindDependencyVersionFallsBackForAnyVersionRequirement(t *testing.T) {
	versions := []ModVersion{
		{ID: "1", Version: "1.2.3", GameVersions: []string{"1.21.0", "1.21.5"}, ReleaseType: "stable", DownloadURL: "https://cdn.test/1.zip"},
		{ID: "2", Version: "1.1.8", GameVersions: []string{"1.21.0"}, ReleaseType: "stable", DownloadURL: "https://cdn.test/2.zip"},
		{ID: "3", Version: "1.0.9", GameVersions: []string{"1.20.12"}, ReleaseType: "stable", DownloadURL: "https://cdn.test/3.zip"},
	}

	// A dependency required as "any version" must not be blocked by stale
	// ModDB release tags: fall back to the best release for the game version.
	v, ok := findDependencyVersion(versions, "*", []string{"1.22.5"}, false)
	if !ok || v.Version != "1.2.3" {
		t.Fatalf("expected best release for any-version dependency, got ok=%v version=%q", ok, v.Version)
	}
	v, ok = findDependencyVersion(versions, "", []string{"1.22.5"}, false)
	if !ok || v.Version != "1.2.3" {
		t.Fatalf("expected best release for empty requirement, got ok=%v version=%q", ok, v.Version)
	}

	// A versioned requirement must still respect game compatibility.
	if _, ok := findDependencyVersion(versions, "1.1.0", []string{"1.22.5"}, false); ok {
		t.Fatal("incompatible versioned requirement must not silently fall back")
	}
	if _, ok := findDependencyVersion(versions, "1.1.0", []string{"1.22.5"}, true); !ok {
		t.Fatal("allowIncompatible must resolve the versioned requirement")
	}

	// Any-version on a compatible game version still picks the best release.
	v, ok = findDependencyVersion(versions, "*", []string{"1.21.5"}, false)
	if !ok || v.Version != "1.2.3" {
		t.Fatalf("expected best compatible release, got ok=%v version=%q", ok, v.Version)
	}
}
