// Package modpack analyzes a build (сборка) of mods installed in a Waxlight
// instance and reports which of the installed mods can be updated through the
// mod catalog and which cannot, together with game-version compatibility and
// dependency information for each candidate update.
//
// The package is deliberately self-contained: it does not import any other
// Waxlight package. It defines its own input types and a single Catalog port,
// so it can be exercised in isolation with a fake catalog and reused
// independently of the launcher's database, application services, or user
// interface. Analyze never downloads or installs anything; it only produces
// information. Callers apply the reported target versions themselves.
package modpack

import "context"

// Build describes the mods currently installed in an instance together with
// the game version the instance runs.
type Build struct {
	// GameVersion is the installed Vintage Story version, for example
	// "1.19.8". It decides whether candidate update versions are compatible
	// with the instance.
	GameVersion string
	// Mods lists the mods installed in the instance.
	Mods []ModInstall
}

// ModInstall describes a single installed mod.
type ModInstall struct {
	// ModID is the catalog identifier of the mod. It is empty for mods that
	// were installed manually and therefore cannot be updated through the
	// catalog.
	ModID string
	// Name is the display name of the mod.
	Name string
	// Version is the installed version string, for example "1.2.3".
	Version string
	// Managed reports whether the mod was installed from the catalog and can
	// be resolved for updates. User-added local mods are not managed.
	Managed bool
	// FileName is the mod archive or script file name.
	FileName string
	// Enabled reports whether the mod is enabled in the instance.
	Enabled bool
	// Dependencies lists the catalog mod IDs the installed version declares in
	// its modinfo.json. It is used to detect dependencies that a newer version
	// no longer requires.
	Dependencies []string
}

// Dependency is a mod dependency declared by a mod version.
type Dependency struct {
	// ModID is the catalog identifier of the dependency.
	ModID string
	// Name is the display name of the dependency.
	Name string
	// Requirement is the version requirement declared by the depending mod,
	// for example ">=1.4.0". It may be empty when the requirement is unknown.
	Requirement string
}

// ModStatus reports the update state of a single installed mod.
type ModStatus string

const (
	// StatusUpToDate means the installed version is the newest catalog version.
	StatusUpToDate ModStatus = "up_to_date"
	// StatusUpdateAvailable means a newer catalog version exists.
	StatusUpdateAvailable ModStatus = "update_available"
	// StatusNotUpdatable means the mod cannot be updated through the catalog
	// for the reason recorded in ModUpdate.Reason.
	StatusNotUpdatable ModStatus = "not_updatable"
	// StatusUnknown means the update state could not be determined.
	StatusUnknown ModStatus = "unknown"
)

// NotUpdatableReason explains why a mod cannot be updated.
type NotUpdatableReason string

const (
	// ReasonNone is used when the mod is updatable or up to date.
	ReasonNone NotUpdatableReason = ""
	// ReasonLocalMod means the mod was installed manually and has no catalog
	// identifier.
	ReasonLocalMod NotUpdatableReason = "local_mod"
	// ReasonNotInCatalog means the catalog has no entry for the mod.
	ReasonNotInCatalog NotUpdatableReason = "not_in_catalog"
	// ReasonCatalogError means the catalog could not be queried for the mod.
	ReasonCatalogError NotUpdatableReason = "catalog_error"
)

// ModUpdate is the per-mod result of Analyze.
type ModUpdate struct {
	// ModID is the catalog identifier of the mod.
	ModID string
	// Name is the display name of the mod.
	Name string
	// InstalledVersion is the version currently installed in the build.
	InstalledVersion string
	// TargetVersionID is the catalog version ID to install to update the mod.
	// It is empty when the mod cannot be updated.
	TargetVersionID string
	// TargetVersion is the version string of the update candidate.
	TargetVersion string
	// Status is the update state of the mod.
	Status ModStatus
	// Reason explains StatusNotUpdatable.
	Reason NotUpdatableReason
	// Changelog is the release changelog of the update candidate.
	Changelog string
	// Compatible reports whether the update candidate supports the build's
	// game version.
	Compatible bool
	// Prerelease reports whether the update candidate is not a stable release.
	Prerelease bool
	// AddedDeps are dependencies the update candidate requires that are not
	// installed in the build.
	AddedDeps []Dependency
	// RemovedDeps are dependencies the installed version declared that the
	// update candidate no longer requires and that are installed in the build.
	RemovedDeps []Dependency
}

// Summary aggregates the per-mod results of Analyze.
type Summary struct {
	// TotalMods is the number of mods in the build.
	TotalMods int
	// UpToDate is the number of mods already at the newest catalog version.
	UpToDate int
	// UpdatesAvailable is the number of mods with an available update.
	UpdatesAvailable int
	// NotUpdatableLocal is the number of manually installed mods.
	NotUpdatableLocal int
	// NotUpdatableAbsent is the number of mods missing from the catalog.
	NotUpdatableAbsent int
	// NotUpdatableCatalogError is the number of mods whose catalog entry could
	// not be loaded.
	NotUpdatableCatalogError int
	// Incompatible is the number of mods with an available update whose
	// candidate does not support the build's game version.
	Incompatible int
}

// Report is the result of Analyze.
type Report struct {
	// Build is the build that was analyzed.
	Build Build
	// Mods holds one entry per mod in the build.
	Mods []ModUpdate
	// Summary aggregates the mod results.
	Summary Summary
}

// Analyze inspects every mod in build and reports which of them have catalog
// updates. It never modifies the build and never installs anything; callers
// apply the reported target versions themselves. A catalog error for a single
// mod is recorded as StatusNotUpdatable with ReasonCatalogError and does not
// abort the whole report. A canceled context aborts analysis and returns the
// context error.
func Analyze(ctx context.Context, build Build, catalog Catalog) (Report, error) {
	report := Report{Build: build, Mods: make([]ModUpdate, 0, len(build.Mods))}
	installed := make(map[string]struct{}, len(build.Mods))
	for _, mod := range build.Mods {
		if mod.ModID != "" {
			installed[mod.ModID] = struct{}{}
		}
	}
	for _, mod := range build.Mods {
		if err := ctx.Err(); err != nil {
			return Report{}, err
		}
		report.Mods = append(report.Mods, analyzeMod(ctx, mod, build.GameVersion, installed, catalog))
	}
	report.Summary = summarize(report.Mods)
	return report, nil
}
