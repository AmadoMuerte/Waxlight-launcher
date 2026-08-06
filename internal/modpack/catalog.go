package modpack

import "context"

// Catalog is the read-only source of mod metadata used by Analyze. Implement
// it against the mod catalog the launcher already queries; the interface
// keeps this package independent of any concrete catalog implementation.
type Catalog interface {
	// Get returns catalog information for a mod. Get must return a ModInfo
	// with an empty ID when the mod is unknown to the catalog. Any other error
	// is treated as a catalog failure for that single mod.
	Get(ctx context.Context, modID string) (ModInfo, error)
}

// ModInfo is the catalog information about a mod.
type ModInfo struct {
	// ID is the catalog identifier of the mod.
	ID string
	// LatestVersion is the newest published version string, for example
	// "1.2.3". It may be empty when the catalog does not publish one.
	LatestVersion string
	// Versions lists the published versions of the mod.
	Versions []ModVersion
}

// ModVersion is a single published version of a mod.
type ModVersion struct {
	// ID is the catalog identifier of this version.
	ID string
	// Version is the version string.
	Version string
	// ReleaseType is the release channel, for example "stable" or "prerelease".
	ReleaseType string
	// GameVersions lists the Vintage Story versions this version supports.
	GameVersions []string
	// Changelog describes what changed in this version.
	Changelog string
	// Dependencies are the mods this version requires.
	Dependencies []Dependency
}
