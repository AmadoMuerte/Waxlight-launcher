package mods

import "strings"

// ParseModDBSource splits a persisted mod source of the form
// "moddb:<modID>:<versionID>" into its parts.
func ParseModDBSource(source string) (string, string, bool) {
	parts := strings.Split(source, ":")
	if len(parts) != 3 || parts[0] != "moddb" || parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

// ModDBSource renders the persisted source marker of a catalog mod release.
func ModDBSource(modID, versionID string) string {
	return "moddb:" + modID + ":" + versionID
}

// IsLocalModSource reports whether a persisted source marker denotes a local
// (non-catalog) mod.
func IsLocalModSource(source string) bool {
	return source == "" || source == "local"
}

// FindModVersion resolves a release by ID, preferring the stable release when
// the ID is empty.
func FindModVersion(versions []ModVersion, id string) (ModVersion, bool) {
	if id == "" && len(versions) > 0 {
		for _, version := range versions {
			if version.ReleaseType == "stable" {
				return version, true
			}
		}
		return versions[0], true
	}
	for _, version := range versions {
		if version.ID == id {
			return version, true
		}
	}
	return ModVersion{}, false
}

// ModSupportsVersion reports whether a catalog release's game-version list
// covers the requested game version. An empty supported list is treated as
// compatible.
func ModSupportsVersion(versions []string, requested string) bool {
	for _, version := range versions {
		if version == requested {
			return true
		}
		majorMinor := strings.Join(strings.Split(version, ".")[:min(2, len(strings.Split(version, ".")))], ".")
		if majorMinor != "" && strings.HasPrefix(requested, majorMinor+".") {
			return true
		}
	}
	return len(versions) == 0
}

func isBuiltInModDependency(modID string) bool {
	switch strings.ToLower(strings.TrimSpace(modID)) {
	case "game", "survival", "creative":
		return true
	default:
		return false
	}
}

func canonicalCatalogModID(details ModDetails) string {
	id := strings.TrimSpace(details.ID)
	if id == "" {
		id = strings.TrimSpace(details.Slug)
	}
	return strings.ToLower(id)
}

func modDownloadKey(modID, versionID string) string { return modID + ":" + versionID }

// Identity implements the instance package mod-identity port over the mods
// identity helpers.
type Identity struct{}

func (Identity) ParseModDBSource(source string) (string, string, bool) {
	return ParseModDBSource(source)
}

func (Identity) ModDBSource(modID, versionID string) string {
	return ModDBSource(modID, versionID)
}

func (Identity) FindModVersion(versions []ModVersion, id string) (ModVersion, bool) {
	return FindModVersion(versions, id)
}

func (Identity) ModSupportsVersion(versions []string, requested string) bool {
	return ModSupportsVersion(versions, requested)
}
