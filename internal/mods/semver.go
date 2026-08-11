package mods

import (
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

// findDependencyVersion picks the best catalog release satisfying a dependency
// requirement for the given game versions. An "any version" requirement falls
// back to the best release when the game-version list is not refreshed, so a
// still-working dependency never fails an install.
func findDependencyVersion(
	versions []ModVersion,
	requirement string,
	gameVersions []string,
	allowIncompatible bool,
) (ModVersion, bool) {
	best, ok := bestSatisfyingVersion(versions, requirement, gameVersions, allowIncompatible)
	if ok {
		return best, true
	}
	requirement = strings.TrimSpace(requirement)
	if requirement == "" || requirement == "*" {
		return bestSatisfyingVersion(versions, requirement, gameVersions, true)
	}
	return ModVersion{}, false
}

// bestSatisfyingVersion ranks the compatible releases of a dependency: exact
// requirement matches first, then stable releases, then semantic versions,
// then published dates, then release IDs.
func bestSatisfyingVersion(
	versions []ModVersion,
	requirement string,
	gameVersions []string,
	allowIncompatible bool,
) (ModVersion, bool) {
	candidates := make([]ModVersion, 0, len(versions))
	for _, version := range versions {
		if !strings.HasPrefix(version.DownloadURL, "https://") {
			continue
		}
		if !modVersionSatisfies(version.Version, requirement) {
			continue
		}
		if !allowIncompatible {
			compatible := true
			for _, gameVersion := range gameVersions {
				if !ModSupportsVersion(version.GameVersions, gameVersion) {
					compatible = false
					break
				}
			}
			if !compatible {
				continue
			}
		}
		candidates = append(candidates, version)
	}
	if len(candidates) == 0 {
		return ModVersion{}, false
	}

	sort.SliceStable(candidates, func(left, right int) bool {
		leftExact := modVersionExactlyMatches(candidates[left].Version, requirement)
		rightExact := modVersionExactlyMatches(candidates[right].Version, requirement)
		if leftExact != rightExact {
			return leftExact
		}

		leftStable := strings.EqualFold(candidates[left].ReleaseType, "stable")
		rightStable := strings.EqualFold(candidates[right].ReleaseType, "stable")
		if leftStable != rightStable {
			return leftStable
		}

		leftVersion := normalizeModSemver(candidates[left].Version)
		rightVersion := normalizeModSemver(candidates[right].Version)
		if leftVersion != "" && rightVersion != "" && leftVersion != rightVersion {
			return semver.Compare(leftVersion, rightVersion) > 0
		}
		if candidates[left].PublishedAt != nil && candidates[right].PublishedAt != nil &&
			!candidates[left].PublishedAt.Equal(*candidates[right].PublishedAt) {
			return candidates[left].PublishedAt.After(*candidates[right].PublishedAt)
		}
		return candidates[left].ID > candidates[right].ID
	})
	return candidates[0], true
}

func modVersionExactlyMatches(version, requirement string) bool {
	requirement = strings.TrimSpace(requirement)
	if requirement == "" || requirement == "*" {
		return false
	}
	for _, operator := range []string{">=", "<=", "==", ">", "<", "="} {
		if strings.HasPrefix(requirement, operator) {
			return false
		}
	}
	actual := normalizeModSemver(version)
	required := normalizeModSemver(requirement)
	if actual != "" && required != "" {
		return semver.Compare(actual, required) == 0
	}
	return strings.EqualFold(strings.TrimSpace(version), requirement)
}

func modVersionSatisfies(version, requirement string) bool {
	requirement = strings.TrimSpace(requirement)
	if requirement == "" || requirement == "*" {
		return true
	}

	operator := ">="
	for _, candidate := range []string{">=", "<=", "==", ">", "<", "="} {
		if strings.HasPrefix(requirement, candidate) {
			operator = candidate
			requirement = strings.TrimSpace(strings.TrimPrefix(requirement, candidate))
			break
		}
	}

	actual := normalizeModSemver(version)
	required := normalizeModSemver(requirement)
	if actual == "" || required == "" {
		if operator == "=" || operator == "==" {
			return strings.EqualFold(strings.TrimSpace(version), requirement)
		}
		return false
	}

	comparison := semver.Compare(actual, required)
	switch operator {
	case ">":
		return comparison > 0
	case "<":
		return comparison < 0
	case "<=":
		return comparison <= 0
	case "=", "==":
		return comparison == 0
	default:
		// Vintage Story treats a plain dependency version as the minimum
		// acceptable version.
		return comparison >= 0
	}
}

func normalizeModSemver(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(version), "v") {
		version = "v" + version
	}
	if !semver.IsValid(version) {
		return ""
	}
	return version
}
