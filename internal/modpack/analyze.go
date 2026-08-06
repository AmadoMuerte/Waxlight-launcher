package modpack

import (
	"context"
	"strings"
)

func analyzeMod(
	ctx context.Context,
	mod ModInstall,
	gameVersion string,
	installed map[string]struct{},
	catalog Catalog,
) ModUpdate {
	result := ModUpdate{
		ModID:            mod.ModID,
		Name:             mod.Name,
		InstalledVersion: mod.Version,
		Status:           StatusUnknown,
	}
	if !mod.Managed || mod.ModID == "" {
		result.Status = StatusNotUpdatable
		result.Reason = ReasonLocalMod
		return result
	}
	info, err := catalog.Get(ctx, mod.ModID)
	if err != nil {
		result.Status = StatusNotUpdatable
		result.Reason = ReasonCatalogError
		return result
	}
	if info.ID == "" {
		result.Status = StatusNotUpdatable
		result.Reason = ReasonNotInCatalog
		return result
	}
	target, ok := selectTargetVersion(info, mod.Version)
	if !ok {
		result.Status = StatusUpToDate
		return result
	}
	result.Status = StatusUpdateAvailable
	result.TargetVersionID = target.ID
	result.TargetVersion = target.Version
	result.Changelog = target.Changelog
	result.Compatible = supportsGameVersion(target.GameVersions, gameVersion)
	result.Prerelease = !isStableRelease(target.ReleaseType)
	result.AddedDeps, result.RemovedDeps = dependencyDiff(mod.Dependencies, target.Dependencies, installed)
	return result
}

// selectTargetVersion picks the update candidate for an installed version. It
// prefers the version the catalog marks as latest, falls back to the newest
// stable version, and finally to any newest version. An installed version is
// returned as "no update" when it matches the candidate.
func selectTargetVersion(info ModInfo, installedVersion string) (ModVersion, bool) {
	if info.LatestVersion != "" {
		if VersionEquals(info.LatestVersion, installedVersion) {
			return ModVersion{}, false
		}
		if version, ok := findVersion(info.Versions, info.LatestVersion); ok {
			return version, true
		}
	}
	if version, ok := newestVersion(info.Versions, true); ok {
		if VersionEquals(version.Version, installedVersion) {
			return ModVersion{}, false
		}
		return version, true
	}
	version, ok := newestVersion(info.Versions, false)
	if !ok || VersionEquals(version.Version, installedVersion) {
		return ModVersion{}, false
	}
	return version, true
}

func findVersion(versions []ModVersion, version string) (ModVersion, bool) {
	for _, candidate := range versions {
		if VersionEquals(candidate.Version, version) {
			return candidate, true
		}
	}
	return ModVersion{}, false
}

func newestVersion(versions []ModVersion, stableOnly bool) (ModVersion, bool) {
	var best ModVersion
	found := false
	for _, candidate := range versions {
		if stableOnly && !isStableRelease(candidate.ReleaseType) {
			continue
		}
		if !found || CompareVersions(candidate.Version, best.Version) > 0 {
			best = candidate
			found = true
		}
	}
	return best, found
}

func isStableRelease(releaseType string) bool {
	return releaseType == "" || strings.EqualFold(releaseType, "stable")
}

func summarize(mods []ModUpdate) Summary {
	var summary Summary
	summary.TotalMods = len(mods)
	for _, mod := range mods {
		switch mod.Status {
		case StatusUpToDate:
			summary.UpToDate++
		case StatusUpdateAvailable:
			summary.UpdatesAvailable++
			if !mod.Compatible {
				summary.Incompatible++
			}
		case StatusNotUpdatable:
			switch mod.Reason {
			case ReasonLocalMod:
				summary.NotUpdatableLocal++
			case ReasonNotInCatalog:
				summary.NotUpdatableAbsent++
			case ReasonCatalogError:
				summary.NotUpdatableCatalogError++
			}
		}
	}
	return summary
}
